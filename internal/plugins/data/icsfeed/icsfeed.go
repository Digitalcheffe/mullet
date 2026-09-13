// Package icsfeed is the universal calendar connector: it fetches and
// parses an ICS/iCal feed from any source (Outlook personal share links,
// Google Calendar public URLs, Apple iCloud, Nextcloud, school/league
// calendars) and writes to the framework's events contract. No OAuth --
// just a URL, and optionally HTTP basic auth credentials for feeds that
// require them.
package icsfeed

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"

	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// ID is this plugin's registry key.
const ID = "ics-feed"

const shapeEvents = "events"

// calendarExternalID is constant because one plugin instance always maps
// to exactly one feed/calendar (per docs/datasources.md: multiple feeds
// are added as separate plugin instances, not multiple URLs on one).
const calendarExternalID = "default"

const (
	// Occurrences (including expanded RRULE instances) are only kept if
	// their start falls in this window around "now". windowBackward
	// catches an event already in progress; windowForward matches the
	// documented default of 60 days.
	windowBackward = 24 * time.Hour
	windowForward  = 60 * 24 * time.Hour
)

func init() {
	if err := plugindata.Register(New()); err != nil {
		panic(err)
	}
}

// Plugin fetches one ICS feed and writes its events to the events
// contract, as a single discovered calendar.
type Plugin struct {
	httpClient *http.Client

	url      string
	name     string
	color    string
	username string
	password string
}

// New returns an unconfigured ICS feed plugin; Configure must run before
// Fetch will succeed.
func New() *Plugin {
	return &Plugin{httpClient: &http.Client{Timeout: 15 * time.Second}}
}

func (p *Plugin) ID() string   { return ID }
func (p *Plugin) Name() string { return "ICS Calendar Feed" }

func (p *Plugin) Manifest() plugindata.DataPluginManifest {
	return plugindata.DataPluginManifest{
		ID:          ID,
		Name:        "ICS Calendar Feed",
		Description: "Universal calendar connector for any ICS/iCal URL -- Outlook, Google, Apple, Nextcloud, and more. No OAuth required.",
		DataShapes:  []string{shapeEvents},
		AuthType:    "none",
		SetupFields: []plugindata.SetupField{
			{
				Key: "url", Label: "ICS Feed URL", Type: "text", Required: true,
				Placeholder: "https://example.com/calendar.ics",
				HelpText:    "To sync more than one calendar, add this plugin again with a different URL.",
			},
			{
				Key: "name", Label: "Display Name", Type: "text", Required: true,
				Placeholder: "Family",
			},
			{
				Key: "color", Label: "Color", Type: "color", Default: "#4285F4",
				HelpText: "Shown by calendar UI plugins.",
			},
			{
				Key: "username", Label: "Username", Type: "text",
				HelpText: "Only needed if the feed requires HTTP basic auth.",
			},
			{
				Key: "password", Label: "Password", Type: "password",
				HelpText: "Only needed if the feed requires HTTP basic auth.",
			},
		},
		RecommendedInterval: 15 * time.Minute,
		MinInterval:         time.Minute,
	}
}

func (p *Plugin) DataShapes() []string { return []string{shapeEvents} }

func (p *Plugin) RefreshInterval() time.Duration { return 15 * time.Minute }

// Configure applies the setup wizard's submitted config.
func (p *Plugin) Configure(cfg map[string]any) error {
	url, _ := cfg["url"].(string)
	name, _ := cfg["name"].(string)
	color, _ := cfg["color"].(string)
	username, _ := cfg["username"].(string)
	password, _ := cfg["password"].(string)

	if url == "" {
		return fmt.Errorf("ics-feed: url is required")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("ics-feed: url must start with http:// or https://")
	}
	if name == "" {
		return fmt.Errorf("ics-feed: name is required")
	}

	p.url = url
	p.name = name
	p.color = color
	p.username = username
	p.password = password
	return nil
}

// Fetch downloads and parses the configured ICS feed, expands any
// recurring events within the retention window, and returns them as
// events-contract rows for a single discovered calendar.
func (p *Plugin) Fetch(ctx context.Context) (map[string][]any, error) {
	if p.url == "" {
		return nil, fmt.Errorf("ics-feed: not configured")
	}

	body, err := p.download(ctx)
	if err != nil {
		return nil, err
	}

	cal, err := ics.ParseCalendarWithOptions(bytes.NewReader(body), ics.WithWindowsTimezoneMapping())
	if err != nil {
		return nil, fmt.Errorf("ics-feed: parsing ICS feed: %w", err)
	}

	now := time.Now().UTC()
	events := expandEvents(cal, now.Add(-windowBackward), now.Add(windowForward))

	rows := make([]any, len(events))
	for i, e := range events {
		e.CalendarExternalID = calendarExternalID
		e.CalendarName = p.name
		if p.color != "" {
			color := p.color
			e.CalendarColor = &color
		}
		rows[i] = e
	}

	return map[string][]any{shapeEvents: rows}, nil
}

func (p *Plugin) download(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, fmt.Errorf("ics-feed: invalid url: %w", err)
	}
	if p.username != "" {
		req.SetBasicAuth(p.username, p.password)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ics-feed: fetching feed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ics-feed: reading feed response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ics-feed: unexpected status %d fetching feed", resp.StatusCode)
	}
	return body, nil
}

// expandEvents walks every VEVENT in cal and returns the events-contract
// rows that fall within [windowStart, windowEnd]: non-recurring events
// as-is, and RRULE events expanded into one row per occurrence. EXDATE
// exceptions are excluded from expansion, VTIMEZONE-qualified times are
// resolved via TZID (see ics.WithWindowsTimezoneMapping in Fetch, plus
// Go's IANA tzdata), and DTSTART;VALUE=DATE events are flagged AllDay.
//
// A RECURRENCE-ID override (a separate VEVENT that modifies or cancels
// one occurrence of a recurring master) has its timestamp excluded from
// the master's expansion, and is itself emitted as its own row via the
// non-recurring path -- unless it's STATUS:CANCELLED, in which case that
// occurrence is dropped entirely.
//
// A malformed individual event (bad DTSTART or RRULE) is skipped rather
// than failing the whole feed -- one bad VEVENT in an otherwise-valid
// feed of hundreds shouldn't sink all of them.
func expandEvents(cal *ics.Calendar, windowStart, windowEnd time.Time) []shapes.CalendarEvent {
	vevents := cal.Events()

	overrides := map[string][]time.Time{}
	for _, ev := range vevents {
		if rid, err := ev.GetRecurrenceID(); err == nil {
			overrides[ev.Id()] = append(overrides[ev.Id()], rid)
		}
	}

	var result []shapes.CalendarEvent
	for _, ev := range vevents {
		if isCancelled(ev) {
			continue
		}

		dtstart, err := ev.GetStartAt()
		if err != nil {
			continue
		}

		var duration time.Duration
		hasEnd := false
		if dtend, err := ev.GetEndAt(); err == nil {
			duration = dtend.Sub(dtstart)
			hasEnd = true
		}

		if !ev.HasProperty(ics.ComponentPropertyRrule) {
			if !withinWindow(dtstart, windowStart, windowEnd) {
				continue
			}
			result = append(result, buildEvent(ev, uniqueRowID(ev), dtstart, hasEnd, duration))
			continue
		}

		starts, err := expandRecurrence(ev, dtstart, overrides[ev.Id()], windowStart, windowEnd)
		if err != nil {
			continue
		}
		for _, occStart := range starts {
			id := ev.Id() + "@" + occStart.UTC().Format(time.RFC3339)
			result = append(result, buildEvent(ev, id, occStart, hasEnd, duration))
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Start.Before(result[j].Start) })
	return result
}

// expandRecurrence returns every occurrence start time of ev's RRULE that
// falls within [windowStart, windowEnd], honoring its own EXDATE/RDATE
// plus extraExclusions (RECURRENCE-ID timestamps of override VEVENTs for
// this same series, which supply that occurrence's row themselves).
func expandRecurrence(ev *ics.VEvent, dtstart time.Time, extraExclusions []time.Time, windowStart, windowEnd time.Time) ([]time.Time, error) {
	rawRule := propValue(ev, ics.ComponentPropertyRrule)
	opt, err := rrule.StrToROption(rawRule)
	if err != nil {
		return nil, fmt.Errorf("parsing RRULE %q: %w", rawRule, err)
	}
	opt.Dtstart = dtstart

	rr, err := rrule.NewRRule(*opt)
	if err != nil {
		return nil, fmt.Errorf("building RRULE: %w", err)
	}

	set := &rrule.Set{}
	set.DTStart(dtstart)
	set.RRule(rr)

	exdates, err := ev.GetExDates()
	if err != nil {
		return nil, fmt.Errorf("reading EXDATE: %w", err)
	}
	for _, ex := range exdates {
		set.ExDate(ex)
	}
	for _, ex := range extraExclusions {
		set.ExDate(ex)
	}

	rdates, err := ev.GetRDates()
	if err != nil {
		return nil, fmt.Errorf("reading RDATE: %w", err)
	}
	for _, rd := range rdates {
		set.RDate(rd)
	}

	return set.Between(windowStart, windowEnd, true), nil
}

func buildEvent(ev *ics.VEvent, id string, start time.Time, hasEnd bool, duration time.Duration) shapes.CalendarEvent {
	evt := shapes.CalendarEvent{
		ID:     id,
		Title:  propValue(ev, ics.ComponentPropertySummary),
		Start:  start,
		AllDay: isAllDay(ev),
	}
	if hasEnd {
		end := start.Add(duration)
		evt.End = &end
	}
	if loc := propValue(ev, ics.ComponentPropertyLocation); loc != "" {
		evt.Location = &loc
	}
	if desc := propValue(ev, ics.ComponentPropertyDescription); desc != "" {
		evt.Description = &desc
	}
	return evt
}

// uniqueRowID returns ev's row ID for the events table: the bare UID for
// a normal event, or "UID@recurrence-id" for a RECURRENCE-ID override --
// matching the "UID@occurrence-start" scheme expandEvents uses for
// RRULE-expanded rows, so an override never collides with (or duplicates)
// its corresponding expanded occurrence.
func uniqueRowID(ev *ics.VEvent) string {
	if rid, err := ev.GetRecurrenceID(); err == nil {
		return ev.Id() + "@" + rid.UTC().Format(time.RFC3339)
	}
	return ev.Id()
}

func isAllDay(ev *ics.VEvent) bool {
	prop := ev.GetProperty(ics.ComponentPropertyDtStart)
	if prop == nil {
		return false
	}
	for _, v := range prop.ICalParameters["VALUE"] {
		if strings.EqualFold(v, "DATE") {
			return true
		}
	}
	return false
}

func isCancelled(ev *ics.VEvent) bool {
	return strings.EqualFold(propValue(ev, ics.ComponentPropertyStatus), "CANCELLED")
}

func propValue(ev *ics.VEvent, key ics.ComponentProperty) string {
	p := ev.GetProperty(key)
	if p == nil {
		return ""
	}
	return p.Value
}

func withinWindow(t, start, end time.Time) bool {
	return !t.Before(start) && !t.After(end)
}
