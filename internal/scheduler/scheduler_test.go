package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

type fakePlugin struct {
	id            string
	shape         string
	interval      time.Duration
	calls         atomic.Int32
	failWith      error
	failConfigure error
	lastConfig    map[string]any
}

func (f *fakePlugin) ID() string   { return f.id }
func (f *fakePlugin) Name() string { return f.id }
func (f *fakePlugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{ID: f.id}
}
func (f *fakePlugin) DataShapes() []string {
	if f.shape == "" {
		return nil
	}
	return []string{f.shape}
}
func (f *fakePlugin) RefreshInterval() time.Duration { return f.interval }
func (f *fakePlugin) Configure(cfg map[string]any) error {
	f.lastConfig = cfg
	return f.failConfigure
}
func (f *fakePlugin) Fetch(ctx context.Context) (map[string][]any, error) {
	f.calls.Add(1)
	if f.failWith != nil {
		return nil, f.failWith
	}
	if f.shape == "" {
		return map[string][]any{}, nil
	}
	return map[string][]any{
		f.shape: {shapes.WeatherCurrent{ID: "current", Temp: 72, Condition: "Clear", Icon: "sun"}},
	}, nil
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Migrations seed a clock plugin instance for a real deployment;
	// tests want a clean, deterministic table to assign their own IDs in.
	if _, err := sqldb.Exec(`DELETE FROM data_plugin_instances`); err != nil {
		t.Fatalf("clearing seeded plugin instances: %v", err)
	}
	return sqldb
}

func TestFetchOnceSuccessWritesShapeAndRecordsFetch(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (1, 'openweathermap', 'Home', 900)`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", shape: "weather_current", interval: time.Hour}
	s := New(sqldb, plugindata.NewRegistry())
	inst := db.PluginInstance{ID: 1, PluginID: "openweathermap", RefreshInterval: time.Hour}

	s.fetchOnce(context.Background(), inst, plugin)

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_weather_current WHERE plugin_instance_id = 1`).Scan(&count); err != nil {
		t.Fatalf("counting shape_weather_current: %v", err)
	}
	if count != 1 {
		t.Errorf("shape_weather_current has %d rows, want 1", count)
	}

	var lastFetchAt *string
	if err := sqldb.QueryRow(`SELECT last_fetch_at FROM data_plugin_instances WHERE id = 1`).Scan(&lastFetchAt); err != nil {
		t.Fatalf("reading last_fetch_at: %v", err)
	}
	if lastFetchAt == nil {
		t.Error("last_fetch_at is nil after successful fetch")
	}
}

func TestFetchOnceErrorRecordsErrorWithoutPanicking(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (1, 'openweathermap', 'Home', 900)`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", shape: "weather_current", interval: time.Hour, failWith: errors.New("api unreachable")}
	s := New(sqldb, plugindata.NewRegistry())
	inst := db.PluginInstance{ID: 1, PluginID: "openweathermap", RefreshInterval: time.Hour}

	s.fetchOnce(context.Background(), inst, plugin) // must not panic

	var lastError *string
	if err := sqldb.QueryRow(`SELECT last_error FROM data_plugin_instances WHERE id = 1`).Scan(&lastError); err != nil {
		t.Fatalf("reading last_error: %v", err)
	}
	if lastError == nil || *lastError != "api unreachable" {
		t.Errorf("last_error = %v, want %q", lastError, "api unreachable")
	}
}

func TestStartRunsOnIntervalAndStopHalts(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (1, 'openweathermap', 'Home', 0)`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", shape: "weather_current", interval: 10 * time.Millisecond}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s := New(sqldb, registry)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Let the ticker fire a few times.
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	callsAtStop := plugin.calls.Load()
	if callsAtStop < 2 {
		t.Fatalf("plugin was called %d times before Stop, want at least 2", callsAtStop)
	}

	// Give any in-flight tick a chance to land, then confirm no further
	// calls happen after Stop has returned.
	time.Sleep(50 * time.Millisecond)
	if got := plugin.calls.Load(); got != callsAtStop {
		t.Errorf("plugin was called %d more times after Stop returned, want 0", got-callsAtStop)
	}
}

func TestStartSkipsUnregisteredPlugin(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (1, 'unregistered-plugin', 'Home', 900)`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	s := New(sqldb, plugindata.NewRegistry())
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s.Stop() // must return promptly; no goroutine should have been spawned
}

func TestStartConfiguresPluginFromStoredConfig(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, config) VALUES (1, 'openweathermap', 'Home', 900, '{"api_key":"secret","location":"Seattle"}')`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", interval: time.Hour}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s := New(sqldb, registry)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s.Stop()

	if plugin.lastConfig["api_key"] != "secret" || plugin.lastConfig["location"] != "Seattle" {
		t.Errorf("lastConfig = %v, want api_key=secret location=Seattle", plugin.lastConfig)
	}
}

func TestStartSkipsInstanceWhenConfigureFails(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, config) VALUES (1, 'openweathermap', 'Home', 900, '{}')`,
	); err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", interval: 10 * time.Millisecond, failConfigure: errors.New("missing api_key")}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s := New(sqldb, registry)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	if plugin.calls.Load() != 0 {
		t.Errorf("Fetch was called %d times after Configure failed, want 0", plugin.calls.Load())
	}

	var lastError *string
	if err := sqldb.QueryRow(`SELECT last_error FROM data_plugin_instances WHERE id = 1`).Scan(&lastError); err != nil {
		t.Fatalf("reading last_error: %v", err)
	}
	if lastError == nil || *lastError != "missing api_key" {
		t.Errorf("last_error = %v, want %q", lastError, "missing api_key")
	}
}
