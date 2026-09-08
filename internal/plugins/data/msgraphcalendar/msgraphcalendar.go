// Package msgraphcalendar fetches Outlook/Microsoft 365 calendar events
// via the official Microsoft Graph SDK. Scoped to personal (consumer)
// Microsoft accounts -- tenant defaults to "consumers"; an enterprise
// (Entra ID) tenant GUID would also work mechanically (the OAuth2 flow
// and this plugin don't hardcode "consumers" anywhere but the default),
// but hasn't been tested against one.
package msgraphcalendar

import (
	"context"
	"fmt"
	"time"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"

	"github.com/Digitalcheffe/mullet/internal/plugins/data/msgraphclient"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

const ID = "msgraph-calendar"

// windowBackward/windowForward mirror icsfeed's own event window, for
// consistent behavior across every calendar-shaped data plugin.
const (
	windowBackward = 24 * time.Hour
	windowForward  = 60 * 24 * time.Hour
	// eventsPerCalendar caps each calendar's calendarView page --
	// there's no follow-the-nextLink pagination yet, so a calendar with
	// more occurrences than this in the window is silently truncated.
	// Generous enough for a dashboard widget's actual use case.
	eventsPerCalendar = int32(250)
)

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin implements plugindata.DataPlugin and plugindata.Discoverable.
type Plugin struct {
	accessToken string
	calendarIDs []string // empty means "all calendars"

	// baseURL overrides the Graph API's base URL -- empty in real use
	// (talks to graph.microsoft.com); tests set it to point at a fake
	// local server instead of needing a real Microsoft account.
	baseURL string
}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "Microsoft Calendar" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "Microsoft Calendar",
		Description: "Outlook/Microsoft 365 calendar events via Microsoft Graph. Works with personal Microsoft accounts.",
		DataShapes:  []string{"events"},
		AuthType:    "oauth2",
		OAuthConfig: &plugindata.OAuthConfig{
			AuthURL:     "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/authorize",
			TokenURL:    "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token",
			Scopes:      []string{"Calendars.Read", "offline_access"},
			TenantField: "tenant",
		},
		SetupFields: []plugindata.SetupField{
			{Key: "client_id", Label: "Client ID", Type: "text", Required: true, HelpText: "From your Azure App Registration."},
			{Key: "client_secret", Label: "Client Secret", Type: "password", Required: true},
			{
				Key: "tenant", Label: "Tenant", Type: "text", Required: true, Default: "consumers",
				HelpText: `Use "consumers" for a personal Outlook/Hotmail/Live account.`,
			},
			{
				Key: "calendars", Label: "Calendars", Type: "multi-select", Dynamic: true,
				HelpText: "Authorize first, then edit this instance to pick specific calendars -- leave empty for all of them.",
			},
		},
		RecommendedInterval: 15 * time.Minute,
		MinInterval:         5 * time.Minute,
	}
}

func (p *Plugin) DataShapes() []string           { return []string{"events"} }
func (p *Plugin) RefreshInterval() time.Duration { return 15 * time.Minute }

// Configure reads the access token the scheduler injects (see
// internal/scheduler.configureInstance) for every OAuth2-type plugin,
// plus this instance's own calendar selection. client_id/client_secret/
// tenant aren't needed here -- they're only for the OAuth2 handshake
// itself (internal/oauth), not for calling Graph with an already-issued
// bearer token.
func (p *Plugin) Configure(cfg map[string]any) error {
	token, _ := cfg["access_token"].(string)
	if token == "" {
		return fmt.Errorf("msgraph-calendar: access_token is required (instance not authorized yet)")
	}
	p.accessToken = token
	p.calendarIDs = stringSlice(cfg["calendars"])
	return nil
}

func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	client, err := newClient(p.accessToken, p.baseURL)
	if err != nil {
		return nil, err
	}

	cals, err := resolveCalendars(ctx, client, p.calendarIDs)
	if err != nil {
		return nil, err
	}

	start := time.Now().Add(-windowBackward).UTC().Format(time.RFC3339)
	end := time.Now().Add(windowForward).UTC().Format(time.RFC3339)
	top := eventsPerCalendar

	var rows []any
	for _, cal := range cals {
		headers := abstractions.NewRequestHeaders()
		// Without this, Graph returns each DateTimeTimeZone in whatever
		// zone the event itself was created in (often a Windows zone
		// name like "Pacific Standard Time") -- requesting UTC up front
		// means every event comes back already normalized, no
		// Windows-zone-name lookup table needed to interpret it.
		headers.TryAdd("Prefer", `outlook.timezone="UTC"`)

		resp, err := client.Me().Calendars().ByCalendarId(cal.id).CalendarView().Get(ctx,
			&users.ItemCalendarsItemCalendarViewRequestBuilderGetRequestConfiguration{
				Headers: headers,
				QueryParameters: &users.ItemCalendarsItemCalendarViewRequestBuilderGetQueryParameters{
					StartDateTime: &start,
					EndDateTime:   &end,
					Top:           &top,
				},
			})
		if err != nil {
			return nil, fmt.Errorf("fetching calendar view for %q: %w", cal.name, err)
		}
		for _, ev := range resp.GetValue() {
			row, err := toShape(ev, cal)
			if err != nil {
				// One malformed event (missing start time, say)
				// shouldn't take down the whole calendar's fetch.
				continue
			}
			rows = append(rows, row)
		}
	}

	return map[string][]any{"events": rows}, nil
}

// Discover lists the account's calendars, for the admin UI's "calendars"
// multi-select (see plugindata.Discoverable) -- cfg carries the same
// framework-injected access_token Configure/Fetch use, but Discover gets
// its own cfg directly rather than reading Plugin's stored state, since
// it can be called against an instance that was never Configure'd in
// this process (e.g. right after the very first authorize).
func (p *Plugin) Discover(ctx context.Context, field string, cfg map[string]any) ([]plugindata.DiscoveredOption, error) {
	if field != "calendars" {
		return nil, fmt.Errorf("msgraph-calendar: no such discoverable field %q", field)
	}
	token, _ := cfg["access_token"].(string)
	if token == "" {
		return nil, fmt.Errorf("msgraph-calendar: access_token is required")
	}
	client, err := newClient(token, p.baseURL)
	if err != nil {
		return nil, err
	}
	cals, err := resolveCalendars(ctx, client, nil)
	if err != nil {
		return nil, err
	}
	options := make([]plugindata.DiscoveredOption, len(cals))
	for i, c := range cals {
		options[i] = plugindata.DiscoveredOption{Value: c.id, Label: c.name}
	}
	return options, nil
}

func newClient(accessToken, baseURL string) (*msgraphsdk.GraphServiceClient, error) {
	if baseURL != "" {
		return msgraphclient.NewWithBaseURL(accessToken, baseURL)
	}
	return msgraphclient.New(accessToken)
}

type calendarRef struct {
	id, name string
	color    *string
}

// resolveCalendars always lists every calendar (their names are needed
// regardless, to populate shapes.CalendarEvent.CalendarName), then
// narrows to selectedIDs if it's non-empty -- an empty selection means
// "all calendars", the natural default before an admin has picked
// specific ones via Discover.
func resolveCalendars(ctx context.Context, client *msgraphsdk.GraphServiceClient, selectedIDs []string) ([]calendarRef, error) {
	resp, err := client.Me().Calendars().Get(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("listing calendars: %w", err)
	}

	selected := make(map[string]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selected[id] = true
	}

	var refs []calendarRef
	for _, c := range resp.GetValue() {
		var id string
		if c.GetId() != nil {
			id = *c.GetId()
		}
		if len(selected) > 0 && !selected[id] {
			continue
		}
		var name string
		if c.GetName() != nil {
			name = *c.GetName()
		}
		var color *string
		if hex := c.GetHexColor(); hex != nil && *hex != "" {
			color = hex
		}
		refs = append(refs, calendarRef{id: id, name: name, color: color})
	}
	return refs, nil
}

func toShape(ev models.Eventable, cal calendarRef) (shapes.CalendarEvent, error) {
	var id string
	if ev.GetId() != nil {
		id = *ev.GetId()
	}
	var title string
	if ev.GetSubject() != nil {
		title = *ev.GetSubject()
	}

	start, err := parseGraphTime(ev.GetStart())
	if err != nil {
		return shapes.CalendarEvent{}, fmt.Errorf("event %q: parsing start: %w", id, err)
	}
	var end *time.Time
	if e, err := parseGraphTime(ev.GetEnd()); err == nil {
		end = &e
	}

	var allDay bool
	if ev.GetIsAllDay() != nil {
		allDay = *ev.GetIsAllDay()
	}

	var location *string
	if loc := ev.GetLocation(); loc != nil {
		if dn := loc.GetDisplayName(); dn != nil && *dn != "" {
			location = dn
		}
	}
	var description *string
	if bp := ev.GetBodyPreview(); bp != nil && *bp != "" {
		description = bp
	}

	return shapes.CalendarEvent{
		// Namespaced by calendar so the same underlying event ID from
		// two different calendars (shouldn't normally happen, but Graph
		// doesn't guarantee it can't) never collides.
		ID:                 fmt.Sprintf("%s@%s", id, cal.id),
		Title:              title,
		Start:              start,
		End:                end,
		AllDay:             allDay,
		Location:           location,
		Description:        description,
		CalendarExternalID: cal.id,
		CalendarName:       cal.name,
		CalendarColor:      cal.color,
	}, nil
}

// graphTimeLayouts covers the DateTime formats Graph actually sends --
// normally fractional seconds to 7 digits, but plain-seconds and full
// RFC3339 (with an explicit offset, in case the Prefer:UTC header is
// ever not honored by a given tenant) are accepted too.
var graphTimeLayouts = []string{
	"2006-01-02T15:04:05.0000000",
	"2006-01-02T15:04:05",
	time.RFC3339,
}

func parseGraphTime(dt models.DateTimeTimeZoneable) (time.Time, error) {
	if dt == nil || dt.GetDateTime() == nil || *dt.GetDateTime() == "" {
		return time.Time{}, fmt.Errorf("missing date/time")
	}
	raw := *dt.GetDateTime()
	for _, layout := range graphTimeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized datetime format %q", raw)
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
