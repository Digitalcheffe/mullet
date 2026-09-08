package exampleplugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// fakeAPIServer mimics just enough of the fictional upstream API for
// Fetch to run against, the same technique every real plugin's own tests
// use (see e.g. internal/plugins/data/homeassistant/homeassistant_test.go)
// -- no real network calls, no API key needed to run `go test`.
func fakeAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/current", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("key"); got != "test-key" {
			t.Errorf("api key = %q, want test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(apiResponse{TempF: 68.5, Condition: "Clear", HumidityPc: 41})
	})
	return httptest.NewServer(mux)
}

func TestFetch(t *testing.T) {
	srv := fakeAPIServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL // point the plugin at the fake server instead of the real API
	if err := p.Configure(map[string]any{"api_key": "test-key", "location": "Seattle,US"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	rows := result["weather_current"]
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	current := rows[0].(shapes.WeatherCurrent)
	if current.Temp != 68.5 || current.Condition != "Clear" || current.Humidity == nil || *current.Humidity != 41 {
		t.Errorf("current = %+v, unexpected values", current)
	}
}

func TestConfigureRequiresAPIKeyAndLocation(t *testing.T) {
	p := New()
	if err := p.Configure(map[string]any{"location": "Seattle,US"}); err == nil {
		t.Error("Configure with no api_key: expected error, got nil")
	}
	if err := p.Configure(map[string]any{"api_key": "test-key"}); err == nil {
		t.Error("Configure with no location: expected error, got nil")
	}
}

func TestFetchBeforeConfigureFails(t *testing.T) {
	p := New()
	if _, err := p.Fetch(context.Background()); err == nil {
		t.Error("Fetch before Configure: expected error, got nil")
	}
}
