// Package homeassistant fetches smart-home device states from a Home
// Assistant instance's own REST API (https://developers.home-assistant.io/docs/api/rest/).
// Auth is a plain long-lived access token the admin generates in Home
// Assistant's own UI (Profile > Security) -- there's no OAuth2 flow to
// this API at all, unlike the msgraph-* plugins.
package homeassistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

const ID = "home-assistant"

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// supportedDomains is which Home Assistant entity domains (the part of
// an entity_id before the ".") this plugin surfaces at all -- a real HA
// instance typically has far more entities (automations, scripts,
// zones, ...) than are meaningful on a dashboard, so everything else is
// silently skipped rather than shown as an opaque, unhandled device.
var supportedDomains = map[string]bool{
	"light": true, "lock": true, "cover": true, "binary_sensor": true,
	"sensor": true, "climate": true,
}

// Plugin implements plugindata.DataPlugin and plugindata.Discoverable.
type Plugin struct {
	url      string
	token    string
	entities []string // empty means "all supported entities"
}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "Home Assistant" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "Home Assistant",
		Description: "Smart home device states (lights, locks, covers, sensors, climate) via Home Assistant's REST API.",
		DataShapes:  []string{"home_devices"},
		AuthType:    "api_key",
		SetupFields: []plugindata.SetupField{
			{
				Key: "url", Label: "Home Assistant URL", Type: "text", Required: true,
				Placeholder: "http://homeassistant.local:8123",
			},
			{
				Key: "token", Label: "Long-Lived Access Token", Type: "password", Required: true,
				HelpText: "Create one in Home Assistant: your profile page > Security > Long-Lived Access Tokens.",
			},
			{
				Key: "entities", Label: "Entities", Type: "multi-select", Dynamic: true,
				HelpText: "Save this instance first, then edit it to pick specific entities -- leave empty for all supported ones.",
			},
		},
		RecommendedInterval: 30 * time.Second,
		MinInterval:         10 * time.Second,
	}
}

func (p *Plugin) DataShapes() []string           { return []string{"home_devices"} }
func (p *Plugin) RefreshInterval() time.Duration { return 30 * time.Second }

func (p *Plugin) Configure(cfg map[string]any) error {
	url, _ := cfg["url"].(string)
	token, _ := cfg["token"].(string)
	if url == "" || token == "" {
		return fmt.Errorf("home-assistant: url and token are required")
	}
	p.url = strings.TrimSuffix(url, "/")
	p.token = token
	p.entities = stringSlice(cfg["entities"])
	return nil
}

func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	states, err := fetchStates(ctx, p.url, p.token)
	if err != nil {
		return nil, err
	}
	// Best-effort: a nil/empty map here just means every device.Area
	// comes back nil below, not a failed Fetch -- see fetchAreas's own
	// doc comment for why this can't be relied on.
	areas := fetchAreas(ctx, p.url, p.token, states)

	selected := make(map[string]bool, len(p.entities))
	for _, id := range p.entities {
		selected[id] = true
	}

	var rows []any
	for _, s := range states {
		if !supportedDomains[domain(s.EntityID)] {
			continue
		}
		if len(selected) > 0 && !selected[s.EntityID] {
			continue
		}

		var area *string
		if a, ok := areas[s.EntityID]; ok && a != "" {
			area = &a
		}

		rows = append(rows, shapes.HomeDevice{
			ID:         s.EntityID,
			Name:       friendlyName(s),
			Area:       area,
			DeviceType: deviceType(s),
			State:      s.State,
		})
	}

	return map[string][]any{"home_devices": rows}, nil
}

// Discover lists every entity in a supported domain, for the admin UI's
// "entities" multi-select. Unlike an OAuth2 plugin's Discover, this
// needs nothing from the framework beyond what's already in cfg -- a
// long-lived access token works immediately, there's no separate
// authorize step for handleDiscover to gate this behind.
func (p *Plugin) Discover(ctx context.Context, field string, cfg map[string]any) ([]plugindata.DiscoveredOption, error) {
	if field != "entities" {
		return nil, fmt.Errorf("home-assistant: no such discoverable field %q", field)
	}
	url, _ := cfg["url"].(string)
	token, _ := cfg["token"].(string)
	if url == "" || token == "" {
		return nil, fmt.Errorf("home-assistant: url and token are required")
	}
	url = strings.TrimSuffix(url, "/")

	states, err := fetchStates(ctx, url, token)
	if err != nil {
		return nil, err
	}

	options := make([]plugindata.DiscoveredOption, 0, len(states))
	for _, s := range states {
		if !supportedDomains[domain(s.EntityID)] {
			continue
		}
		options = append(options, plugindata.DiscoveredOption{Value: s.EntityID, Label: friendlyName(s)})
	}
	return options, nil
}

type haState struct {
	EntityID   string         `json:"entity_id"`
	State      string         `json:"state"`
	Attributes map[string]any `json:"attributes"`
}

func fetchStates(ctx context.Context, baseURL, token string) ([]haState, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/states", nil)
	if err != nil {
		return nil, fmt.Errorf("building states request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching states: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching states: status %d", resp.StatusCode)
	}

	var states []haState
	if err := json.NewDecoder(resp.Body).Decode(&states); err != nil {
		return nil, fmt.Errorf("decoding states: %w", err)
	}
	return states, nil
}

// fetchAreas asks Home Assistant's own template-rendering endpoint
// (POST /api/template, returns plain text -- the template below is
// written to render valid JSON as that text) for every entity's area,
// via the area_name() template function. There's no dedicated REST
// endpoint for area/device registry data, so this is the documented
// way to get it without a websocket connection.
//
// Returns nil (not an error) on any failure -- an older HA version, one
// with template rendering restricted, or a transient network error
// shouldn't fail the whole Fetch over a "nice to have" grouping; the
// caller just gets no area for anything, same as an entity legitimately
// unassigned to one.
func fetchAreas(ctx context.Context, baseURL, token string, states []haState) map[string]string {
	// `sort` inside the template itself, not just on the Go side, so the
	// two lists are guaranteed to agree on ordering however HA's own
	// sort happens to work -- zipping by index only holds if both sides
	// used the exact same sort.
	const template = `{{ states | map(attribute='entity_id') | list | sort | map('area_name') | list | tojson }}`

	body, err := json.Marshal(map[string]string{"template": template})
	if err != nil {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/template", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var areaNames []*string
	if err := json.Unmarshal(raw, &areaNames); err != nil {
		return nil
	}
	if len(states) != len(areaNames) {
		return nil
	}
	entityIDs := make([]string, len(states))
	for i, s := range states {
		entityIDs[i] = s.EntityID
	}
	sort.Strings(entityIDs)

	result := make(map[string]string, len(entityIDs))
	for i, id := range entityIDs {
		if areaNames[i] != nil {
			result[id] = *areaNames[i]
		}
	}
	return result
}

func domain(entityID string) string {
	if i := strings.IndexByte(entityID, '.'); i >= 0 {
		return entityID[:i]
	}
	return ""
}

// deviceType maps an entity to shapes.HomeDevice.DeviceType -- usually
// just its HA domain, except covers/binary_sensors get a more specific
// category from their device_class where HA's own vocabulary lines up
// with what the issue asked for ("doors", "garage" as their own things,
// not just generic "cover"/"binary_sensor").
func deviceType(s haState) string {
	dom := domain(s.EntityID)
	deviceClass, _ := s.Attributes["device_class"].(string)
	switch {
	case dom == "cover" && deviceClass == "garage":
		return "garage"
	case dom == "binary_sensor" && (deviceClass == "door" || deviceClass == "garage_door" || deviceClass == "window"):
		return "door"
	default:
		return dom
	}
}

func friendlyName(s haState) string {
	if name, ok := s.Attributes["friendly_name"].(string); ok && name != "" {
		return name
	}
	return s.EntityID
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}
