// Package exampleplugin is a reference template for writing a new data
// plugin -- see docs/plugin-development.md for the full walkthrough this
// backs. It's a complete, working plugin (compiles, has tests, would run
// correctly if wired up), fetching shapes.WeatherCurrent from a fictional
// REST API so a real developer can follow along with something concrete
// rather than a shape they'd first have to invent.
//
// It is NOT blank-imported by cmd/server/main.go, so it never registers
// with the real server -- copy this directory, rename the package, and
// add that import line yourself once your own plugin is ready (that's
// the last step in the guide, deliberately not done here).
package exampleplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// ID is this plugin's registry key -- must be unique across every
// registered plugin, and stays stable forever once real instances exist
// (it's persisted in data_plugin_instances.plugin_id; renaming it would
// orphan every configured instance).
const ID = "exampleplugin"

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin implements plugindata.DataPlugin. Holds whatever Configure
// parses out of the admin's setup form -- nothing here talks to the
// network until Fetch is called.
type Plugin struct {
	baseURL    string
	httpClient *http.Client

	apiKey   string
	location string
}

// New returns an unconfigured plugin; Configure must run (via the admin
// UI's setup wizard, or a test calling it directly) before Fetch will
// succeed.
func New() *Plugin {
	return &Plugin{
		baseURL:    "https://api.example.com/v1",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "Example Plugin" }

// Manifest drives the admin UI's setup wizard -- SetupFields becomes a
// form, and whatever the admin submits is what Configure receives as
// cfg. See plugindata.SetupField's own doc comment for every field
// type/option this can express (dynamic multi-selects, OAuth2, etc.);
// this template sticks to the common case, a plain API key + a text
// field.
func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "Example Plugin",
		Description: "Template data plugin -- fetches current conditions from a fictional API. Not a real integration.",
		DataShapes:  []string{"weather_current"},
		AuthType:    "api_key",
		SetupFields: []plugindata.SetupField{
			{
				Key: "api_key", Label: "API Key", Type: "password", Required: true,
				HelpText: "Your Example API key.",
			},
			{
				Key: "location", Label: "Location", Type: "text", Required: true,
				Placeholder: "Seattle,US",
				HelpText:    "City name, or whatever format your real API expects.",
			},
		},
		RecommendedInterval: 15 * time.Minute,
		MinInterval:         5 * time.Minute,
	}
}

func (p *Plugin) DataShapes() []string { return []string{"weather_current"} }

func (p *Plugin) RefreshInterval() time.Duration { return 15 * time.Minute }

// Configure applies the setup wizard's submitted config. Called again on
// every scheduled tick (see internal/scheduler) with the same stored
// config -- Configure isn't a one-time setup step, it's how the shared
// plugin object gets pointed at the right instance's settings right
// before each Fetch. Validate everything Fetch needs here, not there.
func (p *Plugin) Configure(cfg map[string]any) error {
	apiKey, _ := cfg["api_key"].(string)
	location, _ := cfg["location"].(string)

	if apiKey == "" {
		return fmt.Errorf("exampleplugin: api_key is required")
	}
	if location == "" {
		return fmt.Errorf("exampleplugin: location is required")
	}

	p.apiKey = apiKey
	p.location = location
	return nil
}

// Fetch returns every shape this plugin produces in one call, keyed by
// shape name -- a plugin producing more than one shape (see
// openweathermap, which returns weather_current AND weather_forecast)
// returns both from the same Fetch. The scheduler replaces this
// instance's existing rows for each shape with whatever's returned here.
func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	if p.apiKey == "" || p.location == "" {
		return nil, fmt.Errorf("exampleplugin: not configured")
	}

	current, err := p.fetchCurrent(ctx)
	if err != nil {
		return nil, err
	}

	return map[string][]any{
		"weather_current": {current},
	}, nil
}

// apiResponse is the subset of the fictional API's response this plugin
// actually uses -- real plugins typically define one of these per
// endpoint and only pull out the fields their shape needs, ignoring the
// rest (see openweathermap.currentResponse for a real example with more
// fields).
type apiResponse struct {
	TempF      float64 `json:"temp_f"`
	Condition  string  `json:"condition"`
	HumidityPc int     `json:"humidity_percent"`
}

func (p *Plugin) fetchCurrent(ctx context.Context) (shapes.WeatherCurrent, error) {
	params := url.Values{}
	params.Set("location", p.location)
	params.Set("key", p.apiKey)
	reqURL := fmt.Sprintf("%s/current?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return shapes.WeatherCurrent{}, fmt.Errorf("exampleplugin: building request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return shapes.WeatherCurrent{}, fmt.Errorf("exampleplugin: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return shapes.WeatherCurrent{}, fmt.Errorf("exampleplugin: unexpected status %d", resp.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return shapes.WeatherCurrent{}, fmt.Errorf("exampleplugin: parsing response: %w", err)
	}

	humidity := body.HumidityPc
	return shapes.WeatherCurrent{
		ID:        "current",
		Temp:      body.TempF,
		Condition: body.Condition,
		Humidity:  &humidity,
	}, nil
}
