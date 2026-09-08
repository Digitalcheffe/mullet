package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
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
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s := New(sqldb, registry)
	inst := db.PluginInstance{ID: 1, PluginID: "openweathermap", RefreshInterval: time.Hour}

	s.fetchOnce(context.Background(), inst)

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
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s := New(sqldb, registry)
	inst := db.PluginInstance{ID: 1, PluginID: "openweathermap", RefreshInterval: time.Hour}

	s.fetchOnce(context.Background(), inst) // must not panic

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

// waitForCalls polls until plugin.calls reaches want, or fails the test
// after timeout. Start/Reload launch fetch goroutines asynchronously, so
// tests that assert on an "immediate" fetch must wait for it rather than
// checking calls.Load() right after the call returns.
func waitForCalls(t *testing.T, plugin *fakePlugin, want int32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if plugin.calls.Load() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("plugin called %d times, want >= %d within %v", plugin.calls.Load(), want, timeout)
}

func TestReloadPicksUpNewlyAddedInstance(t *testing.T) {
	sqldb := newTestDB(t)

	plugin := &fakePlugin{id: "openweathermap", interval: time.Hour}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s := New(sqldb, registry)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	if plugin.calls.Load() != 0 {
		t.Fatalf("plugin called %d times before any instance existed, want 0", plugin.calls.Load())
	}

	// Simulate the admin API creating a new instance.
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, config) VALUES (1, 'openweathermap', 'Home', 3600, '{}')`,
	); err != nil {
		t.Fatalf("inserting instance: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	waitForCalls(t, plugin, 1, time.Second)
}

func TestReloadStopsRemovedAndDisabledInstances(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		// refresh_seconds=0 falls back to the plugin's own RefreshInterval
		// (10ms below), so this test doesn't take real wall-clock minutes.
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, config) VALUES (1, 'openweathermap', 'Home', 0, '{}')`,
	); err != nil {
		t.Fatalf("seeding instance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", interval: 10 * time.Millisecond}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s := New(sqldb, registry)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	waitForCalls(t, plugin, 2, time.Second)
	callsBeforeDisable := plugin.calls.Load()

	if _, err := sqldb.Exec(`UPDATE data_plugin_instances SET enabled = 0 WHERE id = 1`); err != nil {
		t.Fatalf("disabling instance: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	time.Sleep(30 * time.Millisecond)
	callsAfterDisable := plugin.calls.Load()
	if callsAfterDisable != callsBeforeDisable {
		t.Errorf("plugin called %d more times after being disabled, want 0", callsAfterDisable-callsBeforeDisable)
	}
}

func TestReloadRestartsChangedInstanceImmediately(t *testing.T) {
	sqldb := newTestDB(t)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds, config) VALUES (1, 'openweathermap', 'Home', 3600, '{}')`,
	); err != nil {
		t.Fatalf("seeding instance: %v", err)
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
	defer s.Stop()

	waitForCalls(t, plugin, 1, time.Second)

	// Simulate the admin API editing the instance's config. With a
	// 1-hour interval, only an immediate re-fetch on Reload proves the
	// change took effect without a restart.
	if _, err := sqldb.Exec(
		`UPDATE data_plugin_instances SET config = '{"location":"changed"}' WHERE id = 1`,
	); err != nil {
		t.Fatalf("updating instance config: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	waitForCalls(t, plugin, 2, time.Second)
	if plugin.lastConfig["location"] != "changed" {
		t.Errorf("lastConfig = %v, want location=changed", plugin.lastConfig)
	}
}

func TestTestInstanceRunsConfigureAndFetch(t *testing.T) {
	sqldb := newTestDB(t)
	id, err := db.CreatePluginInstance(sqldb, "openweathermap", "Home", 900, false, `{"location":"Seattle"}`)
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", shape: "weather_current", interval: time.Hour}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s := New(sqldb, registry)

	if err := s.TestInstance(context.Background(), id); err != nil {
		t.Fatalf("TestInstance: %v", err)
	}

	if plugin.calls.Load() != 1 {
		t.Errorf("plugin called %d times, want 1", plugin.calls.Load())
	}
	if plugin.lastConfig["location"] != "Seattle" {
		t.Errorf("lastConfig = %v, want location=Seattle", plugin.lastConfig)
	}

	var count int
	if err := sqldb.QueryRow(`SELECT COUNT(*) FROM shape_weather_current WHERE plugin_instance_id = ?`, id).Scan(&count); err != nil {
		t.Fatalf("counting shape_weather_current: %v", err)
	}
	if count != 1 {
		t.Errorf("shape_weather_current has %d rows after TestInstance, want 1", count)
	}
}

func TestTestInstanceReturnsFetchError(t *testing.T) {
	sqldb := newTestDB(t)
	id, err := db.CreatePluginInstance(sqldb, "openweathermap", "Home", 900, false, `{}`)
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	plugin := &fakePlugin{id: "openweathermap", interval: time.Hour, failWith: errors.New("api unreachable")}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}
	s := New(sqldb, registry)

	err = s.TestInstance(context.Background(), id)
	if err == nil || err.Error() != "api unreachable" {
		t.Errorf("TestInstance error = %v, want api unreachable", err)
	}

	var lastError *string
	sqldb.QueryRow(`SELECT last_error FROM data_plugin_instances WHERE id = ?`, id).Scan(&lastError)
	if lastError == nil || *lastError != "api unreachable" {
		t.Errorf("last_error = %v, want api unreachable", lastError)
	}
}

func TestTestInstanceUnknownID(t *testing.T) {
	sqldb := newTestDB(t)
	s := New(sqldb, plugindata.NewRegistry())

	if err := s.TestInstance(context.Background(), 9999); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("TestInstance(missing) = %v, want ErrNotFound", err)
	}
}

func TestTestInstanceDoesNotRaceWithScheduledFetch(t *testing.T) {
	sqldb := newTestDB(t)
	id, err := db.CreatePluginInstance(sqldb, "openweathermap", "Home", 0, true, `{}`)
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	plugin := &concurrencyCheckFakePlugin{fakePlugin: fakePlugin{id: "openweathermap", shape: "weather_current", interval: 5 * time.Millisecond}}
	registry := plugindata.NewRegistry()
	if err := registry.Register(plugin); err != nil {
		t.Fatalf("Register: %v", err)
	}

	s := New(sqldb, registry)
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Stop()

	// Hammer TestInstance concurrently with the scheduler's own ticks.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.TestInstance(context.Background(), id)
		}()
	}
	wg.Wait()

	if plugin.violation.Load() {
		t.Error("a manual TestInstance call overlapped with a scheduled fetch on the shared plugin object")
	}
}

// concurrencyCheckFakePlugin extends fakePlugin to detect two callers
// inside Fetch() at once, proving Registry.WithPlugin's serialization
// covers TestInstance vs. the scheduler's own ticks, not just two
// TestInstance calls against each other.
type concurrencyCheckFakePlugin struct {
	fakePlugin
	inside    atomic.Bool
	violation atomic.Bool
}

func (p *concurrencyCheckFakePlugin) Fetch(ctx context.Context) (map[string][]any, error) {
	if !p.inside.CompareAndSwap(false, true) {
		p.violation.Store(true)
	}
	defer p.inside.Store(false)
	time.Sleep(time.Millisecond)
	return p.fakePlugin.Fetch(ctx)
}
