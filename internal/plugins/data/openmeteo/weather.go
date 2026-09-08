// Package openmeteo fetches current conditions and a daily forecast from
// Open-Meteo (open-meteo.com) -- a free weather API that needs no
// account or API key. It writes to the same weather_current /
// weather_forecast framework contracts as openweathermap, so either
// plugin can back the same UI plugins interchangeably.
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// ID is this plugin's registry key.
const ID = "open-meteo"

const (
	shapeCurrent  = "weather_current"
	shapeForecast = "weather_forecast"

	defaultForecastURL = "https://api.open-meteo.com/v1/forecast"
	defaultGeocodeURL  = "https://geocoding-api.open-meteo.com/v1/search"

	maxForecastDays = 5
)

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin fetches current conditions and a 5-day forecast from Open-Meteo.
// Unlike openweathermap, it needs no API key -- Configure only takes a
// location and units.
type Plugin struct {
	forecastURL string
	geocodeURL  string
	httpClient  *http.Client

	location string
	units    string // "metric" or "imperial"
}

// New returns an unconfigured Open-Meteo plugin; Configure must run
// (via the admin UI setup wizard) before Fetch will succeed.
func New() *Plugin {
	return &Plugin{
		forecastURL: defaultForecastURL,
		geocodeURL:  defaultGeocodeURL,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		units:       "metric",
	}
}

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "Open-Meteo" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "Open-Meteo",
		Description: "Current conditions and a 5-day forecast from Open-Meteo. Free, no API key required.",
		DataShapes:  []string{shapeCurrent, shapeForecast},
		AuthType:    "none",
		SetupFields: []plugindata.SetupField{
			{
				Key: "location", Label: "Location", Type: "text", Required: true,
				Placeholder: "Seattle,US or 47.6062,-122.3321",
				HelpText:    "City name or \"lat,lon\". No API key needed.",
			},
			{
				Key: "units", Label: "Units", Type: "select", Required: true, Default: "metric",
				Options: []plugindata.SelectOption{
					{Value: "metric", Label: "Celsius"},
					{Value: "imperial", Label: "Fahrenheit"},
				},
			},
		},
		RecommendedInterval: 15 * time.Minute,
		MinInterval:         5 * time.Minute,
	}
}

func (p *Plugin) DataShapes() []string { return []string{shapeCurrent, shapeForecast} }

func (p *Plugin) RefreshInterval() time.Duration { return 15 * time.Minute }

// Configure applies the setup wizard's submitted config.
func (p *Plugin) Configure(cfg map[string]any) error {
	location, _ := cfg["location"].(string)
	units, _ := cfg["units"].(string)

	if location == "" {
		return fmt.Errorf("open-meteo: location is required")
	}
	if units != "metric" && units != "imperial" {
		units = "metric"
	}

	p.location = location
	p.units = units
	return nil
}

// Fetch resolves the configured location to coordinates (geocoding a
// city name if needed), then retrieves current conditions and a 5-day
// forecast in one request.
func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	if p.location == "" {
		return nil, fmt.Errorf("open-meteo: not configured")
	}

	lat, lon, err := p.resolveLocation(ctx)
	if err != nil {
		return nil, err
	}

	body, err := p.getForecast(ctx, lat, lon)
	if err != nil {
		return nil, err
	}

	var resp forecastResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("open-meteo: parsing response: %w", err)
	}

	forecast := resp.dailyShapes()
	forecastRows := make([]any, len(forecast))
	for i, d := range forecast {
		forecastRows[i] = d
	}

	return map[string][]any{
		shapeCurrent:  {resp.currentShape()},
		shapeForecast: forecastRows,
	}, nil
}

var latLonPattern = regexp.MustCompile(`^-?\d+(\.\d+)?,-?\d+(\.\d+)?$`)

// resolveLocation returns lat/lon strings for the configured location,
// geocoding it first if it isn't already in "lat,lon" form.
func (p *Plugin) resolveLocation(ctx context.Context) (lat, lon string, err error) {
	if latLonPattern.MatchString(p.location) {
		latStr, lonStr, _ := strings.Cut(p.location, ",")
		return latStr, lonStr, nil
	}
	return p.geocode(ctx, p.location)
}

func (p *Plugin) geocode(ctx context.Context, name string) (lat, lon string, err error) {
	params := url.Values{"name": {name}, "count": {"1"}}
	reqURL := fmt.Sprintf("%s?%s", p.geocodeURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("open-meteo: building geocode request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("open-meteo: geocode request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("open-meteo: reading geocode response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("open-meteo: geocoding failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var geo geocodeResponse
	if err := json.Unmarshal(body, &geo); err != nil {
		return "", "", fmt.Errorf("open-meteo: parsing geocode response: %w", err)
	}
	if len(geo.Results) == 0 {
		return "", "", fmt.Errorf("open-meteo: location %q not found", name)
	}

	r := geo.Results[0]
	return strconv.FormatFloat(r.Latitude, 'f', -1, 64), strconv.FormatFloat(r.Longitude, 'f', -1, 64), nil
}

type geocodeResponse struct {
	Results []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
	} `json:"results"`
}

func (p *Plugin) getForecast(ctx context.Context, lat, lon string) ([]byte, error) {
	tempUnit := "celsius"
	if p.units == "imperial" {
		tempUnit = "fahrenheit"
	}

	params := url.Values{
		"latitude":         {lat},
		"longitude":        {lon},
		"current":          {"temperature_2m,apparent_temperature,relative_humidity_2m,weather_code"},
		"daily":            {"temperature_2m_max,temperature_2m_min,weather_code,precipitation_probability_max,sunrise,sunset"},
		"timezone":         {"auto"},
		"forecast_days":    {"5"},
		"temperature_unit": {tempUnit},
	}
	reqURL := fmt.Sprintf("%s?%s", p.forecastURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: building request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("open-meteo: reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("open-meteo: rate limited (429)")
	default:
		return nil, fmt.Errorf("open-meteo: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

// forecastResponse is the subset of Open-Meteo's /v1/forecast response we
// use. Unlike OpenWeatherMap's per-entry objects, Open-Meteo returns
// parallel arrays (daily.time[i] pairs with daily.temperature_2m_max[i],
// etc.).
type forecastResponse struct {
	Current struct {
		Temperature         float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		RelativeHumidity    int     `json:"relative_humidity_2m"`
		WeatherCode         int     `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		Time                        []string  `json:"time"`
		TemperatureMax              []float64 `json:"temperature_2m_max"`
		TemperatureMin              []float64 `json:"temperature_2m_min"`
		WeatherCode                 []int     `json:"weather_code"`
		PrecipitationProbabilityMax []int     `json:"precipitation_probability_max"`
		Sunrise                     []string  `json:"sunrise"`
		Sunset                      []string  `json:"sunset"`
	} `json:"daily"`
}

func (r forecastResponse) currentShape() shapes.WeatherCurrent {
	condition, icon := conditionForCode(r.Current.WeatherCode)
	feelsLike := r.Current.ApparentTemperature
	humidity := r.Current.RelativeHumidity

	current := shapes.WeatherCurrent{
		ID:        "current",
		Temp:      r.Current.Temperature,
		FeelsLike: &feelsLike,
		Condition: condition,
		Icon:      icon,
		Humidity:  &humidity,
	}

	if len(r.Daily.TemperatureMax) > 0 {
		high := r.Daily.TemperatureMax[0]
		current.High = &high
	}
	if len(r.Daily.TemperatureMin) > 0 {
		low := r.Daily.TemperatureMin[0]
		current.Low = &low
	}
	if len(r.Daily.Sunrise) > 0 {
		sunrise := timeOfDay(r.Daily.Sunrise[0])
		current.Sunrise = &sunrise
	}
	if len(r.Daily.Sunset) > 0 {
		sunset := timeOfDay(r.Daily.Sunset[0])
		current.Sunset = &sunset
	}

	return current
}

func (r forecastResponse) dailyShapes() []shapes.WeatherForecast {
	n := len(r.Daily.Time)
	if n > maxForecastDays {
		n = maxForecastDays
	}

	result := make([]shapes.WeatherForecast, 0, n)
	for i := 0; i < n; i++ {
		var condition, icon string
		if i < len(r.Daily.WeatherCode) {
			condition, icon = conditionForCode(r.Daily.WeatherCode[i])
		}
		var precip *int
		if i < len(r.Daily.PrecipitationProbabilityMax) {
			v := r.Daily.PrecipitationProbabilityMax[i]
			precip = &v
		}

		result = append(result, shapes.WeatherForecast{
			ID:           r.Daily.Time[i],
			Date:         r.Daily.Time[i],
			High:         valueAt(r.Daily.TemperatureMax, i),
			Low:          valueAt(r.Daily.TemperatureMin, i),
			Condition:    condition,
			Icon:         icon,
			PrecipChance: precip,
		})
	}
	return result
}

func valueAt(values []float64, i int) float64 {
	if i < len(values) {
		return values[i]
	}
	return 0
}

// timeOfDay extracts "HH:MM" from an Open-Meteo ISO8601 timestamp like
// "2026-09-07T06:36".
func timeOfDay(iso8601 string) string {
	if idx := strings.IndexByte(iso8601, 'T'); idx != -1 && idx+1 < len(iso8601) {
		return iso8601[idx+1:]
	}
	return iso8601
}

// conditionForCode normalizes a WMO weather code (Open-Meteo's condition
// vocabulary) into the same rough Condition/Icon categories OpenWeatherMap
// uses ("Clear", "Clouds", "Rain", ...), so a UI plugin consuming the
// weather_current/weather_forecast contract renders consistently
// regardless of which data plugin wrote the row.
func conditionForCode(code int) (condition, icon string) {
	switch {
	case code == 0 || code == 1:
		return "Clear", "clear"
	case code == 2 || code == 3:
		return "Clouds", "clouds"
	case code == 45 || code == 48:
		return "Fog", "fog"
	case code >= 51 && code <= 57:
		return "Drizzle", "drizzle"
	case (code >= 61 && code <= 67) || (code >= 80 && code <= 82):
		return "Rain", "rain"
	case (code >= 71 && code <= 77) || code == 85 || code == 86:
		return "Snow", "snow"
	case code >= 95 && code <= 99:
		return "Thunderstorm", "thunderstorm"
	default:
		return "Unknown", "unknown"
	}
}
