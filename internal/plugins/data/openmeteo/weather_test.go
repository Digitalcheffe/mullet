package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

const forecastJSON = `{
	"current": {"temperature_2m": 17.3, "apparent_temperature": 18.0, "relative_humidity_2m": 83, "weather_code": 0},
	"daily": {
		"time": ["2026-09-07", "2026-09-08", "2026-09-09", "2026-09-10", "2026-09-11", "2026-09-12"],
		"temperature_2m_max": [18.9, 22.2, 24.8, 20.0, 19.0, 21.0],
		"temperature_2m_min": [13.4, 11.3, 12.2, 12.7, 11.7, 12.0],
		"weather_code": [3, 0, 61, 3, 3, 0],
		"precipitation_probability_max": [5, 0, 80, 10, 18, 0],
		"sunrise": ["2026-09-07T06:36", "2026-09-08T06:37", "2026-09-09T06:38", "2026-09-10T06:40", "2026-09-11T06:41", "2026-09-12T06:42"],
		"sunset": ["2026-09-07T19:37", "2026-09-08T19:35", "2026-09-09T19:33", "2026-09-10T19:31", "2026-09-11T19:29", "2026-09-12T19:27"]
	}
}`

const geocodeJSON = `{"results":[{"latitude":47.6062,"longitude":-122.3321,"name":"Seattle"}]}`

func newMockServers(t *testing.T, forecastBody string, forecastStatus int, geocodeBody string, geocodeStatus int) (forecastURL, geocodeURL string) {
	t.Helper()
	forecast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(forecastStatus)
		w.Write([]byte(forecastBody))
	}))
	t.Cleanup(forecast.Close)

	geocode := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(geocodeStatus)
		w.Write([]byte(geocodeBody))
	}))
	t.Cleanup(geocode.Close)

	return forecast.URL, geocode.URL
}

func TestFetchSuccessWithCityName(t *testing.T) {
	forecastURL, geocodeURL := newMockServers(t, forecastJSON, http.StatusOK, geocodeJSON, http.StatusOK)

	p := New()
	p.forecastURL = forecastURL
	p.geocodeURL = geocodeURL
	if err := p.Configure(map[string]any{"location": "Seattle,US", "units": "metric"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	currentRows := result[shapeCurrent]
	if len(currentRows) != 1 {
		t.Fatalf("result[%q] = %v, want 1 row", shapeCurrent, currentRows)
	}
	current := currentRows[0].(shapes.WeatherCurrent)
	if current.Temp != 17.3 || current.Condition != "Clear" || current.Icon != "clear" {
		t.Errorf("current = %+v, unexpected values", current)
	}
	if current.Sunrise == nil || *current.Sunrise != "06:36" {
		t.Errorf("current.Sunrise = %v, want 06:36", current.Sunrise)
	}

	forecastRows := result[shapeForecast]
	if len(forecastRows) != maxForecastDays {
		t.Fatalf("result[%q] has %d rows, want %d (capped)", shapeForecast, len(forecastRows), maxForecastDays)
	}
	day3 := forecastRows[2].(shapes.WeatherForecast)
	if day3.Condition != "Rain" || day3.PrecipChance == nil || *day3.PrecipChance != 80 {
		t.Errorf("day3 = %+v, want Condition=Rain PrecipChance=80", day3)
	}
}

func TestFetchSuccessWithLatLon(t *testing.T) {
	forecastURL, geocodeURL := newMockServers(t, forecastJSON, http.StatusOK, `{"results":[]}`, http.StatusOK)

	p := New()
	p.forecastURL = forecastURL
	p.geocodeURL = geocodeURL // must not be hit for lat/lon input
	if err := p.Configure(map[string]any{"location": "47.6062,-122.3321"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if _, err := p.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
}

func TestFetchGeocodeNotFound(t *testing.T) {
	forecastURL, geocodeURL := newMockServers(t, forecastJSON, http.StatusOK, `{"results":[]}`, http.StatusOK)

	p := New()
	p.forecastURL = forecastURL
	p.geocodeURL = geocodeURL
	if err := p.Configure(map[string]any{"location": "Nowhereville"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	_, err := p.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Fetch error = %v, want mention of location not found", err)
	}
}

func TestFetchRateLimited(t *testing.T) {
	forecastURL, geocodeURL := newMockServers(t, `{}`, http.StatusTooManyRequests, geocodeJSON, http.StatusOK)

	p := New()
	p.forecastURL = forecastURL
	p.geocodeURL = geocodeURL
	if err := p.Configure(map[string]any{"location": "Seattle"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	_, err := p.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("Fetch error = %v, want mention of rate limiting", err)
	}
}

func TestFetchNetworkFailure(t *testing.T) {
	p := New()
	p.forecastURL = "http://127.0.0.1:1"
	if err := p.Configure(map[string]any{"location": "47.6062,-122.3321"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

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

	if err := p.Configure(map[string]any{}); err == nil {
		t.Error("Configure without location: expected error, got nil")
	}

	if err := p.Configure(map[string]any{"location": "Seattle", "units": "bogus"}); err != nil {
		t.Fatalf("Configure with bad units: %v", err)
	}
	if p.units != "metric" {
		t.Errorf("units = %q after invalid input, want default %q", p.units, "metric")
	}
}

func TestConditionForCode(t *testing.T) {
	cases := map[int]string{
		0: "Clear", 1: "Clear", 2: "Clouds", 3: "Clouds", 45: "Fog", 55: "Drizzle",
		63: "Rain", 80: "Rain", 73: "Snow", 86: "Snow", 95: "Thunderstorm", 200: "Unknown",
	}
	for code, want := range cases {
		if got, _ := conditionForCode(code); got != want {
			t.Errorf("conditionForCode(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestManifest(t *testing.T) {
	p := New()
	m := p.Manifest()

	if m.AuthType != "none" {
		t.Errorf("AuthType = %q, want none", m.AuthType)
	}
	if len(m.DataShapes) != 2 {
		t.Errorf("DataShapes = %v, want 2 entries", m.DataShapes)
	}
	for _, f := range m.SetupFields {
		if f.Key == "api_key" {
			t.Error("SetupFields should not include api_key -- open-meteo needs no auth")
		}
	}
}

func TestRegistersItselfAtInit(t *testing.T) {
	plugin, ok := plugindata.Get(ID)
	if !ok {
		t.Fatal("open-meteo plugin not found in Default registry; init() should have registered it")
	}
	if plugin.ID() != ID {
		t.Errorf("registered plugin ID = %q, want %q", plugin.ID(), ID)
	}
}
