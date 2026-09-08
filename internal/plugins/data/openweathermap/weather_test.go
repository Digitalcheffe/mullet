package openweathermap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

const currentJSON = `{
	"weather": [{"main": "Clouds", "description": "few clouds", "icon": "02d"}],
	"main": {"temp": 21.5, "feels_like": 21.0, "temp_min": 20.0, "temp_max": 23.0, "humidity": 60},
	"sys": {"sunrise": 1700000000, "sunset": 1700040000}
}`

// Two entries on day 1 (one near noon, one late) and one on day 2, so the
// aggregation across day boundaries, noon-proximity, and max-pop logic
// are all exercised.
const forecastJSON = `{
	"list": [
		{"dt_txt": "2024-01-01 09:00:00", "main": {"temp": 10, "temp_min": 9, "temp_max": 11}, "weather": [{"main": "Rain", "icon": "10d"}], "pop": 0.8},
		{"dt_txt": "2024-01-01 12:00:00", "main": {"temp": 15, "temp_min": 14, "temp_max": 16}, "weather": [{"main": "Clouds", "icon": "03d"}], "pop": 0.2},
		{"dt_txt": "2024-01-02 15:00:00", "main": {"temp": 18, "temp_min": 17, "temp_max": 19}, "weather": [{"main": "Clear", "icon": "01d"}], "pop": 0.0}
	]
}`

func newMockServer(t *testing.T, currentBody, forecastBody string, currentStatus, forecastStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/weather"):
			w.WriteHeader(currentStatus)
			w.Write([]byte(currentBody))
		case strings.HasPrefix(r.URL.Path, "/forecast"):
			w.WriteHeader(forecastStatus)
			w.Write([]byte(forecastBody))
		default:
			http.NotFound(w, r)
		}
	}))
}

func configuredPlugin(t *testing.T, baseURL string) *Plugin {
	t.Helper()
	p := New()
	p.baseURL = baseURL
	if err := p.Configure(map[string]any{"api_key": "test-key", "location": "Seattle,US", "units": "metric"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	return p
}

func TestFetchSuccess(t *testing.T) {
	server := newMockServer(t, currentJSON, forecastJSON, http.StatusOK, http.StatusOK)
	defer server.Close()

	p := configuredPlugin(t, server.URL)

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	currentRows, ok := result[shapeCurrent]
	if !ok || len(currentRows) != 1 {
		t.Fatalf("result[%q] = %v, want 1 row", shapeCurrent, currentRows)
	}
	current, ok := currentRows[0].(shapes.WeatherCurrent)
	if !ok {
		t.Fatalf("current row is %T, want shapes.WeatherCurrent", currentRows[0])
	}
	if current.Temp != 21.5 || current.Condition != "Clouds" || current.Icon != "02d" {
		t.Errorf("current = %+v, unexpected values", current)
	}
	if current.Sunrise == nil || current.Sunset == nil {
		t.Error("current.Sunrise/Sunset not set")
	}

	forecastRows, ok := result[shapeForecast]
	if !ok || len(forecastRows) != 2 {
		t.Fatalf("result[%q] has %d rows, want 2 (two distinct dates)", shapeForecast, len(forecastRows))
	}
	day1, ok := forecastRows[0].(shapes.WeatherForecast)
	if !ok {
		t.Fatalf("forecast row is %T, want shapes.WeatherForecast", forecastRows[0])
	}
	if day1.Date != "2024-01-01" || day1.High != 16 || day1.Low != 9 {
		t.Errorf("day1 = %+v, want Date=2024-01-01 High=16 Low=9", day1)
	}
	// Noon entry (12:00, Clouds) is closer to noon than 09:00 (Rain).
	if day1.Condition != "Clouds" {
		t.Errorf("day1.Condition = %q, want %q (closest reading to noon)", day1.Condition, "Clouds")
	}
	if day1.PrecipChance == nil || *day1.PrecipChance != 80 {
		t.Errorf("day1.PrecipChance = %v, want 80 (max pop across the day)", day1.PrecipChance)
	}
}

func TestFetchInvalidAPIKey(t *testing.T) {
	server := newMockServer(t, `{"cod":401,"message":"Invalid API key"}`, "", http.StatusUnauthorized, http.StatusOK)
	defer server.Close()

	p := configuredPlugin(t, server.URL)
	_, err := p.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("Fetch error = %v, want mention of invalid API key", err)
	}
}

func TestFetchRateLimited(t *testing.T) {
	server := newMockServer(t, `{"cod":429,"message":"rate limited"}`, "", http.StatusTooManyRequests, http.StatusOK)
	defer server.Close()

	p := configuredPlugin(t, server.URL)
	_, err := p.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("Fetch error = %v, want mention of rate limiting", err)
	}
}

func TestFetchNetworkFailure(t *testing.T) {
	p := configuredPlugin(t, "http://127.0.0.1:1") // nothing listens here

	_, err := p.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Errorf("Fetch error = %v, want mention of request failure", err)
	}
}

func TestFetchNotConfigured(t *testing.T) {
	p := New()
	if _, err := p.Fetch(context.Background()); err == nil {
		t.Error("Fetch on unconfigured plugin: expected error, got nil")
	}
}

func TestConfigureValidation(t *testing.T) {
	p := New()

	if err := p.Configure(map[string]any{"location": "Seattle"}); err == nil {
		t.Error("Configure without api_key: expected error, got nil")
	}
	if err := p.Configure(map[string]any{"api_key": "k"}); err == nil {
		t.Error("Configure without location: expected error, got nil")
	}

	if err := p.Configure(map[string]any{"api_key": "k", "location": "Seattle", "units": "bogus"}); err != nil {
		t.Fatalf("Configure with bad units: %v", err)
	}
	if p.units != "metric" {
		t.Errorf("units = %q after invalid input, want default %q", p.units, "metric")
	}
}

func TestLocationParams(t *testing.T) {
	p := New()

	p.location = "Seattle,US"
	params := p.locationParams()
	if params.Get("q") != "Seattle,US" {
		t.Errorf("city name: params = %v, want q=Seattle,US", params)
	}

	p.location = "47.6062,-122.3321"
	params = p.locationParams()
	if params.Get("lat") != "47.6062" || params.Get("lon") != "-122.3321" {
		t.Errorf("lat/lon: params = %v, want lat=47.6062 lon=-122.3321", params)
	}
}

func TestManifest(t *testing.T) {
	p := New()
	m := p.Manifest()

	if m.AuthType != "api_key" {
		t.Errorf("AuthType = %q, want api_key", m.AuthType)
	}
	if len(m.DataShapes) != 2 {
		t.Errorf("DataShapes = %v, want 2 entries", m.DataShapes)
	}
	fieldKeys := map[string]bool{}
	for _, f := range m.SetupFields {
		fieldKeys[f.Key] = true
	}
	for _, want := range []string{"api_key", "location", "units"} {
		if !fieldKeys[want] {
			t.Errorf("SetupFields missing key %q", want)
		}
	}
}

func TestRegistersItselfAtInit(t *testing.T) {
	plugin, ok := plugindata.Get(ID)
	if !ok {
		t.Fatal("openweathermap plugin not found in Default registry; init() should have registered it")
	}
	if plugin.ID() != ID {
		t.Errorf("registered plugin ID = %q, want %q", plugin.ID(), ID)
	}
}
