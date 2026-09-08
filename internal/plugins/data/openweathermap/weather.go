// Package openweathermap fetches current conditions and a 5-day forecast
// from the OpenWeatherMap API.
package openweathermap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// ID is this plugin's registry key.
const ID = "openweathermap"

const (
	shapeCurrent  = "weather_current"
	shapeForecast = "weather_forecast"

	defaultBaseURL = "https://api.openweathermap.org/data/2.5"
)

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin fetches current conditions and a 5-day/3-hour forecast (which it
// aggregates into daily highs/lows) from OpenWeatherMap.
type Plugin struct {
	baseURL    string
	httpClient *http.Client

	apiKey   string
	location string
	units    string // "metric" or "imperial"
}

// New returns an unconfigured OpenWeatherMap plugin; Configure must run
// (via the admin UI setup wizard) before Fetch will succeed.
func New() *Plugin {
	return &Plugin{
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		units:      "metric",
	}
}

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "OpenWeatherMap" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "OpenWeatherMap",
		Description: "Current conditions and a 5-day forecast from OpenWeatherMap.",
		DataShapes:  []string{shapeCurrent, shapeForecast},
		AuthType:    "api_key",
		SetupFields: []plugindata.SetupField{
			{
				Key: "api_key", Label: "API Key", Type: "password", Required: true,
				HelpText: "Your OpenWeatherMap API key.",
			},
			{
				Key: "location", Label: "Location", Type: "text", Required: true,
				Placeholder: "Seattle,US or 47.6062,-122.3321",
				HelpText:    "City name (optionally \"City,CountryCode\") or \"lat,lon\".",
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
	apiKey, _ := cfg["api_key"].(string)
	location, _ := cfg["location"].(string)
	units, _ := cfg["units"].(string)

	if apiKey == "" {
		return fmt.Errorf("openweathermap: api_key is required")
	}
	if location == "" {
		return fmt.Errorf("openweathermap: location is required")
	}
	if units != "metric" && units != "imperial" {
		units = "metric"
	}

	p.apiKey = apiKey
	p.location = location
	p.units = units
	return nil
}

// Fetch retrieves current conditions and the 5-day forecast in one cycle.
// Either request failing (invalid key, rate limit, network error) fails
// the whole fetch -- there's no partial write.
func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	if p.apiKey == "" || p.location == "" {
		return nil, fmt.Errorf("openweathermap: not configured")
	}

	current, err := p.fetchCurrent(ctx)
	if err != nil {
		return nil, err
	}

	forecast, err := p.fetchForecast(ctx)
	if err != nil {
		return nil, err
	}

	return map[string][]any{
		shapeCurrent:  {current},
		shapeForecast: forecast,
	}, nil
}

func (p *Plugin) fetchCurrent(ctx context.Context) (shapes.WeatherCurrent, error) {
	body, err := p.get(ctx, "weather")
	if err != nil {
		return shapes.WeatherCurrent{}, err
	}

	var resp currentResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return shapes.WeatherCurrent{}, fmt.Errorf("openweathermap: parsing current weather: %w", err)
	}

	return resp.toShape(), nil
}

func (p *Plugin) fetchForecast(ctx context.Context) ([]any, error) {
	body, err := p.get(ctx, "forecast")
	if err != nil {
		return nil, err
	}

	var resp forecastResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openweathermap: parsing forecast: %w", err)
	}

	days := aggregateDaily(resp.List)
	rows := make([]any, len(days))
	for i, d := range days {
		rows[i] = d
	}
	return rows, nil
}

// get issues a GET to path (e.g. "weather", "forecast") with the
// configured location/units/API key, and returns the raw response body
// on a 200. Other statuses and transport errors are turned into
// descriptive errors -- invalid key, rate limit, and network failures are
// each distinguishable by the caller (and by a human reading the log).
func (p *Plugin) get(ctx context.Context, path string) ([]byte, error) {
	params := p.locationParams()
	params.Set("appid", p.apiKey)
	params.Set("units", p.units)

	reqURL := fmt.Sprintf("%s/%s?%s", p.baseURL, path, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("openweathermap: building request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openweathermap: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openweathermap: reading response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("openweathermap: invalid API key")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("openweathermap: rate limited (429)")
	default:
		return nil, fmt.Errorf("openweathermap: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

var latLonPattern = regexp.MustCompile(`^-?\d+(\.\d+)?,-?\d+(\.\d+)?$`)

// locationParams returns the query params identifying where to fetch
// weather for: lat/lon if location looks like "lat,lon", otherwise a
// city-name lookup.
func (p *Plugin) locationParams() url.Values {
	params := url.Values{}
	if latLonPattern.MatchString(p.location) {
		lat, lon, _ := strings.Cut(p.location, ",")
		params.Set("lat", lat)
		params.Set("lon", lon)
	} else {
		params.Set("q", p.location)
	}
	return params
}

// currentResponse is the subset of OpenWeatherMap's /weather response we
// use.
type currentResponse struct {
	Weather []struct {
		Main string `json:"main"`
		Icon string `json:"icon"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		TempMin   float64 `json:"temp_min"`
		TempMax   float64 `json:"temp_max"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Sys struct {
		Sunrise int64 `json:"sunrise"`
		Sunset  int64 `json:"sunset"`
	} `json:"sys"`
}

func (r currentResponse) toShape() shapes.WeatherCurrent {
	var condition, icon string
	if len(r.Weather) > 0 {
		condition = r.Weather[0].Main
		icon = r.Weather[0].Icon
	}

	feelsLike := r.Main.FeelsLike
	humidity := r.Main.Humidity
	high := r.Main.TempMax
	low := r.Main.TempMin

	current := shapes.WeatherCurrent{
		ID:        "current",
		Temp:      r.Main.Temp,
		FeelsLike: &feelsLike,
		Condition: condition,
		Icon:      icon,
		Humidity:  &humidity,
		High:      &high,
		Low:       &low,
	}

	if r.Sys.Sunrise > 0 {
		sunrise := time.Unix(r.Sys.Sunrise, 0).UTC().Format("15:04")
		current.Sunrise = &sunrise
	}
	if r.Sys.Sunset > 0 {
		sunset := time.Unix(r.Sys.Sunset, 0).UTC().Format("15:04")
		current.Sunset = &sunset
	}

	return current
}

// forecastResponse is the subset of OpenWeatherMap's /forecast response
// we use: a list of 3-hourly entries covering ~5 days.
type forecastResponse struct {
	List []forecastEntry `json:"list"`
}

type forecastEntry struct {
	DateTimeText string `json:"dt_txt"`
	Main         struct {
		Temp    float64 `json:"temp"`
		TempMin float64 `json:"temp_min"`
		TempMax float64 `json:"temp_max"`
	} `json:"main"`
	Weather []struct {
		Main string `json:"main"`
		Icon string `json:"icon"`
	} `json:"weather"`
	Pop float64 `json:"pop"` // probability of precipitation, 0.0-1.0
}

const maxForecastDays = 5

// aggregateDaily buckets 3-hourly forecast entries by calendar date into
// one shapes.WeatherForecast per day: high/low from the min/max across
// that day's entries, precip chance from the day's highest pop, and
// condition/icon from whichever entry falls closest to noon (a single
// "representative" reading for the day, same heuristic most weather UIs
// use for a multi-day summary).
func aggregateDaily(entries []forecastEntry) []shapes.WeatherForecast {
	type dayAgg struct {
		date            string
		high, low       float64
		haveHighLow     bool
		condition, icon string
		bestHourDist    int
		maxPop          float64
	}

	days := make(map[string]*dayAgg)
	var order []string

	for _, e := range entries {
		t, err := time.Parse("2006-01-02 15:04:05", e.DateTimeText)
		if err != nil {
			continue
		}
		date := t.Format("2006-01-02")

		day, ok := days[date]
		if !ok {
			day = &dayAgg{date: date, bestHourDist: 1 << 30}
			days[date] = day
			order = append(order, date)
		}

		if !day.haveHighLow || e.Main.TempMax > day.high {
			day.high = e.Main.TempMax
		}
		if !day.haveHighLow || e.Main.TempMin < day.low {
			day.low = e.Main.TempMin
		}
		day.haveHighLow = true

		if e.Pop > day.maxPop {
			day.maxPop = e.Pop
		}

		hourDist := t.Hour() - 12
		if hourDist < 0 {
			hourDist = -hourDist
		}
		if hourDist < day.bestHourDist && len(e.Weather) > 0 {
			day.bestHourDist = hourDist
			day.condition = e.Weather[0].Main
			day.icon = e.Weather[0].Icon
		}
	}

	sort.Strings(order)
	if len(order) > maxForecastDays {
		order = order[:maxForecastDays]
	}

	result := make([]shapes.WeatherForecast, 0, len(order))
	for _, date := range order {
		d := days[date]
		precip := int(d.maxPop*100 + 0.5)
		result = append(result, shapes.WeatherForecast{
			ID:           date,
			Date:         date,
			High:         d.high,
			Low:          d.low,
			Condition:    d.condition,
			Icon:         d.icon,
			PrecipChance: &precip,
		})
	}
	return result
}
