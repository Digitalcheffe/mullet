package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetShapeDataUnknownShapeIs404(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/not_a_shape", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestGetShapeDataEmptyIsEmptyArrayNoAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	// No Authorization header at all: /api/data/* must not require auth.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/weather_current", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp dataResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp.Data == nil || len(resp.Data) != 0 {
		t.Errorf("Data = %v, want empty (non-null) array", resp.Data)
	}
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Errorf("body = %s, want literal \"data\":[] (not null)", rec.Body.String())
	}
}

func TestGetShapeDataReturnsSeededRow(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	// newTestUserDB's migration auto-seeds a 'clock' instance at id=1;
	// insert our own openweathermap instance rather than assuming an id.
	res, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (plugin_id, instance_name, refresh_seconds) VALUES ('openweathermap', 'Home', 900)`,
	)
	if err != nil {
		t.Fatalf("seeding plugin instance: %v", err)
	}
	instanceID, _ := res.LastInsertId()

	if _, err := sqldb.Exec(
		`INSERT INTO shape_weather_current (id, plugin_instance_id, temp, condition, icon) VALUES ('current', ?, 21.5, 'Clear', 'clear')`,
		instanceID,
	); err != nil {
		t.Fatalf("seeding row: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/weather_current", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp dataResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Data[0]["temp"] != 21.5 {
		t.Errorf("Data = %+v, want 1 row with temp=21.5", resp.Data)
	}
	if resp.Source != "openweathermap" {
		t.Errorf("Source = %q, want openweathermap", resp.Source)
	}
}

func TestGetShapeDataFiltersByPluginQueryParam(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	if _, err := sqldb.Exec(
		`INSERT INTO data_plugin_instances (id, plugin_id, instance_name, refresh_seconds) VALUES (2, 'open-meteo', 'Backyard', 900)`,
	); err != nil {
		t.Fatalf("seeding second instance: %v", err)
	}
	// Row for instance 1 (the migration's auto-seeded 'clock' instance --
	// its actual plugin_id doesn't matter for this test, just that it's a
	// distinct instance from id=2).
	if _, err := sqldb.Exec(
		`INSERT INTO shape_weather_current (id, plugin_instance_id, temp, condition, icon) VALUES ('current', 1, 20, 'Clear', 'clear')`,
	); err != nil {
		t.Fatalf("seeding instance 1 row: %v", err)
	}
	if _, err := sqldb.Exec(
		`INSERT INTO shape_weather_current (id, plugin_instance_id, temp, condition, icon) VALUES ('current', 2, 15, 'Rain', 'rain')`,
	); err != nil {
		t.Fatalf("seeding instance 2 row: %v", err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/weather_current?plugin=2", nil))

	var resp dataResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Data[0]["temp"] != 15.0 {
		t.Errorf("Data = %+v, want 1 row with temp=15 (instance 2 only)", resp.Data)
	}
}

func TestGetShapeDataInvalidPluginParam(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/weather_current?plugin=abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGetShapeDataTimeRangeUnsupportedShape(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/weather_current?from=2024-01-01T00:00:00Z", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (weather_current doesn't support from/to)", rec.Code)
	}
}

func TestGetShapeDataEventsTimeRange(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	if _, err := sqldb.Exec(
		`INSERT INTO calendars (id, plugin_instance_id, external_id, name) VALUES (1, 1, 'ext-1', 'Family')`,
	); err != nil {
		t.Fatalf("seeding calendar: %v", err)
	}
	for _, e := range []struct{ id, start string }{
		{"evt-early", "2024-01-01T00:00:00Z"},
		{"evt-mid", "2024-06-01T00:00:00Z"},
	} {
		if _, err := sqldb.Exec(
			`INSERT INTO shape_events (id, plugin_instance_id, calendar_id, title, start) VALUES (?, 1, 1, 'Event', ?)`,
			e.id, e.start,
		); err != nil {
			t.Fatalf("seeding event: %v", err)
		}
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/events?from=2024-03-01T00:00:00Z", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	var resp dataResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 || resp.Data[0]["id"] != "evt-mid" {
		t.Errorf("Data = %+v, want only evt-mid", resp.Data)
	}
}

func TestGetShapeDataInvalidTimeFormat(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/data/events?from=not-a-date", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
