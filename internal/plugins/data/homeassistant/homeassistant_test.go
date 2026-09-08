package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// fakeHAServer mimics just enough of a real Home Assistant instance's
// REST API (https://developers.home-assistant.io/docs/api/rest/) for
// this plugin to run its full Configure/Fetch/Discover cycle against,
// without needing a real Home Assistant instance.
func fakeHAServer(t *testing.T, areaByEntity map[string]string) *httptest.Server {
	t.Helper()
	states := []map[string]any{
		{"entity_id": "light.living_room", "state": "on", "attributes": map[string]any{"friendly_name": "Living Room Light"}},
		{"entity_id": "lock.front_door", "state": "locked", "attributes": map[string]any{"friendly_name": "Front Door"}},
		{"entity_id": "cover.garage_door", "state": "closed", "attributes": map[string]any{"friendly_name": "Garage Door", "device_class": "garage"}},
		{"entity_id": "binary_sensor.back_door", "state": "off", "attributes": map[string]any{"friendly_name": "Back Door", "device_class": "door"}},
		{"entity_id": "sensor.outdoor_temp", "state": "68.5", "attributes": map[string]any{"friendly_name": "Outdoor Temperature", "unit_of_measurement": "°F"}},
		{"entity_id": "climate.thermostat", "state": "heat", "attributes": map[string]any{"friendly_name": "Thermostat"}},
		// Unsupported domains -- must never show up in Fetch/Discover output.
		{"entity_id": "automation.morning_routine", "state": "on", "attributes": map[string]any{"friendly_name": "Morning Routine"}},
		{"entity_id": "zone.home", "state": "zoning", "attributes": map[string]any{"friendly_name": "Home"}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("states request Authorization = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(states)
	})
	mux.HandleFunc("/api/template", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Template string `json:"template"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding template request: %v", err)
		}
		if body.Template == "" {
			t.Error("template request body has no template field")
		}
		// Real HA returns plain text (the template's own rendered
		// output) -- this plugin's template renders JSON as that text,
		// so respond with the same shape a real instance would: a JSON
		// array of area names (or null), sorted by entity_id (the
		// template's own `| sort` filter), as plain text (not
		// application/json -- matching the real endpoint).
		ids := make([]string, len(states))
		for i, s := range states {
			ids[i] = s["entity_id"].(string)
		}
		sort.Strings(ids)
		out := make([]*string, len(ids))
		for i, id := range ids {
			if a, ok := areaByEntity[id]; ok {
				area := a
				out[i] = &area
			}
		}
		raw, _ := json.Marshal(out)
		w.Write(raw)
	})

	return httptest.NewServer(mux)
}

func TestFetchAllSupportedEntities(t *testing.T) {
	srv := fakeHAServer(t, map[string]string{"light.living_room": "Living Room", "lock.front_door": "Entryway"})
	defer srv.Close()

	p := New()
	if err := p.Configure(map[string]any{"url": srv.URL, "token": "test-token"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	devices := result["home_devices"]
	if len(devices) != 6 {
		t.Fatalf("got %d devices, want 6 (automation/zone excluded)", len(devices))
	}

	byID := map[string]shapes.HomeDevice{}
	for _, row := range devices {
		d := row.(shapes.HomeDevice)
		byID[d.ID] = d
	}

	light, ok := byID["light.living_room"]
	if !ok || light.Name != "Living Room Light" || light.DeviceType != "light" || light.State != "on" {
		t.Errorf("light = %+v (ok=%v), unexpected values", light, ok)
	}
	if light.Area == nil || *light.Area != "Living Room" {
		t.Errorf("light.Area = %v, want Living Room", light.Area)
	}

	garage, ok := byID["cover.garage_door"]
	if !ok || garage.DeviceType != "garage" {
		t.Errorf("garage = %+v (ok=%v), want DeviceType=garage", garage, ok)
	}

	door, ok := byID["binary_sensor.back_door"]
	if !ok || door.DeviceType != "door" {
		t.Errorf("door = %+v (ok=%v), want DeviceType=door", door, ok)
	}

	sensor, ok := byID["sensor.outdoor_temp"]
	if !ok || sensor.State != "68.5" || sensor.Area != nil {
		t.Errorf("sensor = %+v (ok=%v), want State=68.5, Area=nil (no area configured for it)", sensor, ok)
	}

	if _, ok := byID["automation.morning_routine"]; ok {
		t.Error("automation entity leaked into home_devices output")
	}
}

func TestFetchFiltersToSelectedEntities(t *testing.T) {
	srv := fakeHAServer(t, nil)
	defer srv.Close()

	p := New()
	if err := p.Configure(map[string]any{"url": srv.URL, "token": "test-token", "entities": []any{"light.living_room"}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(result["home_devices"]) != 1 {
		t.Fatalf("got %d devices, want 1 (only the selected entity)", len(result["home_devices"]))
	}
	d := result["home_devices"][0].(shapes.HomeDevice)
	if d.ID != "light.living_room" {
		t.Errorf("ID = %q, want light.living_room", d.ID)
	}
}

func TestFetchSurvivesAreaLookupFailure(t *testing.T) {
	// A server that 500s on /api/template (simulating an HA instance
	// with template rendering disabled/restricted, or just erroring)
	// must not fail the whole Fetch -- area is best-effort.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"entity_id": "light.kitchen", "state": "off", "attributes": map[string]any{"friendly_name": "Kitchen"}},
		})
	})
	mux.HandleFunc("/api/template", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "template rendering disabled", http.StatusForbidden)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	p := New()
	if err := p.Configure(map[string]any{"url": srv.URL, "token": "test-token"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v, want no error even though area lookup failed", err)
	}
	if len(result["home_devices"]) != 1 {
		t.Fatalf("got %d devices, want 1", len(result["home_devices"]))
	}
	d := result["home_devices"][0].(shapes.HomeDevice)
	if d.Area != nil {
		t.Errorf("Area = %v, want nil (area lookup failed, shouldn't block Fetch)", *d.Area)
	}
}

func TestConfigureRequiresURLAndToken(t *testing.T) {
	p := New()
	if err := p.Configure(map[string]any{"token": "x"}); err == nil {
		t.Error("Configure with no url: expected error, got nil")
	}
	if err := p.Configure(map[string]any{"url": "http://x"}); err == nil {
		t.Error("Configure with no token: expected error, got nil")
	}
}

func TestDiscoverEntities(t *testing.T) {
	srv := fakeHAServer(t, nil)
	defer srv.Close()

	p := New()
	options, err := p.Discover(context.Background(), "entities", map[string]any{"url": srv.URL, "token": "test-token"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(options) != 6 {
		t.Fatalf("got %d options, want 6 (automation/zone excluded)", len(options))
	}
	found := false
	for _, o := range options {
		if o.Value == "light.living_room" && o.Label == "Living Room Light" {
			found = true
		}
	}
	if !found {
		t.Errorf("options = %+v, missing light.living_room", options)
	}
}

func TestDiscoverUnknownFieldFails(t *testing.T) {
	p := New()
	if _, err := p.Discover(context.Background(), "not-a-real-field", map[string]any{"url": "http://x", "token": "y"}); err == nil {
		t.Error("Discover(unknown field): expected error, got nil")
	}
}

func TestManifestDeclaresAPIKeyAuth(t *testing.T) {
	m := New().Manifest()
	if m.AuthType != "api_key" {
		t.Errorf("AuthType = %q, want api_key (no OAuth2 flow for this API)", m.AuthType)
	}
	found := false
	for _, f := range m.SetupFields {
		if f.Key == "entities" {
			found = true
			if !f.Dynamic {
				t.Error("entities field is not marked Dynamic")
			}
		}
	}
	if !found {
		t.Error("manifest has no \"entities\" SetupField")
	}
}
