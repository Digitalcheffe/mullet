package icsfeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

func TestManifest(t *testing.T) {
	m := New().Manifest()
	if m.ID != ID {
		t.Errorf("Manifest.ID = %q, want %q", m.ID, ID)
	}
	if m.AuthType != "none" {
		t.Errorf("AuthType = %q, want none", m.AuthType)
	}
	if len(m.DataShapes) != 1 || m.DataShapes[0] != "events" {
		t.Errorf("DataShapes = %v, want [events]", m.DataShapes)
	}

	keys := map[string]bool{}
	for _, f := range m.SetupFields {
		keys[f.Key] = true
	}
	for _, want := range []string{"url", "name", "color", "username", "password"} {
		if !keys[want] {
			t.Errorf("SetupFields missing key %q", want)
		}
	}
}

func TestConfigureValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{"missing url", map[string]any{"name": "Family"}, true},
		{"missing name", map[string]any{"url": "https://example.com/cal.ics"}, true},
		{"unsupported scheme", map[string]any{"url": "ftp://example.com/cal.ics", "name": "Family"}, true},
		{"valid", map[string]any{"url": "https://example.com/cal.ics", "name": "Family"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New().Configure(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure(%v) error = %v, wantErr %v", tt.cfg, err, tt.wantErr)
			}
		})
	}
}

func TestFetchNotConfigured(t *testing.T) {
	if _, err := New().Fetch(context.Background()); err == nil {
		t.Error("Fetch without Configure: expected error, got nil")
	}
}

// serveICS starts an httptest server returning body for every request and
// returns a Plugin already Configure()'d to fetch from it.
func serveICS(t *testing.T, body string) *Plugin {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	p := New()
	if err := p.Configure(map[string]any{"url": srv.URL, "name": "Family", "color": "#4285F4"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	return p
}

func icsTime(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func minimalValidICS() string {
	return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\nEND:VCALENDAR\n"
}

func fetchEvents(t *testing.T, p *Plugin) []shapes.CalendarEvent {
	t.Helper()
	rows, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	events := make([]shapes.CalendarEvent, len(rows["events"]))
	for i, row := range rows["events"] {
		e, ok := row.(shapes.CalendarEvent)
		if !ok {
			t.Fatalf("row %d has type %T, want shapes.CalendarEvent", i, row)
		}
		events[i] = e
	}
	return events
}

func TestFetchSimpleEvent(t *testing.T) {
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:evt-1@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART:" + icsTime(start) + "\n" +
		"DTEND:" + icsTime(end) + "\n" +
		"SUMMARY:Team Standup\n" +
		"LOCATION:Room 1\n" +
		"DESCRIPTION:Daily sync\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Title != "Team Standup" {
		t.Errorf("Title = %q, want Team Standup", e.Title)
	}
	if e.AllDay {
		t.Error("AllDay = true, want false")
	}
	if !e.Start.Equal(start) {
		t.Errorf("Start = %v, want %v", e.Start, start)
	}
	if e.End == nil || !e.End.Equal(end) {
		t.Errorf("End = %v, want %v", e.End, end)
	}
	if e.Location == nil || *e.Location != "Room 1" {
		t.Errorf("Location = %v, want Room 1", e.Location)
	}
	if e.Description == nil || *e.Description != "Daily sync" {
		t.Errorf("Description = %v, want Daily sync", e.Description)
	}
	if e.CalendarExternalID != "default" || e.CalendarName != "Family" {
		t.Errorf("calendar identity = (%q, %q), want (default, Family)", e.CalendarExternalID, e.CalendarName)
	}
	if e.CalendarColor == nil || *e.CalendarColor != "#4285F4" {
		t.Errorf("CalendarColor = %v, want #4285F4", e.CalendarColor)
	}
}

func TestFetchAllDayEvent(t *testing.T) {
	date := time.Now().UTC().AddDate(0, 0, 3)
	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:allday-1@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART;VALUE=DATE:" + date.Format("20060102") + "\n" +
		"SUMMARY:Company Holiday\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if !events[0].AllDay {
		t.Error("AllDay = false, want true")
	}
	if events[0].End != nil {
		t.Errorf("End = %v, want nil (no DTEND)", events[0].End)
	}
}

func TestFetchTZIDResolvesToCorrectInstant(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation(America/New_York): %v", err)
	}
	// A wall-clock time a few days out, so it stays inside the fetch
	// window regardless of when this test runs.
	local := time.Now().In(loc).AddDate(0, 0, 3)
	wallClock := time.Date(local.Year(), local.Month(), local.Day(), 9, 0, 0, 0, loc)
	wantStart := wallClock.UTC()

	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:tzid-1@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART;TZID=America/New_York:" + wallClock.Format("20060102T150405") + "\n" +
		"SUMMARY:Eastern Time Meeting\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if !events[0].Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v (9am America/New_York converted to UTC)", events[0].Start, wantStart)
	}
}

func TestFetchRecurringEventWithExdate(t *testing.T) {
	dtstart := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	excluded := dtstart.AddDate(0, 0, 2)
	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:daily-standup@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART:" + icsTime(dtstart) + "\n" +
		"DTEND:" + icsTime(dtstart.Add(30*time.Minute)) + "\n" +
		"RRULE:FREQ=DAILY;COUNT=5\n" +
		"EXDATE:" + icsTime(excluded) + "\n" +
		"SUMMARY:Daily Standup\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4 (5 occurrences minus 1 EXDATE)", len(events))
	}
	for _, e := range events {
		if e.Start.Equal(excluded) {
			t.Errorf("excluded occurrence %v was not removed", excluded)
		}
		if !strings.HasPrefix(e.ID, "daily-standup@example.com@") {
			t.Errorf("row id = %q, want prefix daily-standup@example.com@", e.ID)
		}
	}
}

func TestFetchEventOutsideWindowExcluded(t *testing.T) {
	farFuture := time.Now().UTC().AddDate(0, 0, 90)
	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:far-future@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART:" + icsTime(farFuture) + "\n" +
		"SUMMARY:Way Out There\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 0 {
		t.Errorf("got %d events, want 0 (event is outside the 60-day window)", len(events))
	}
}

func TestFetchCancelledEventSkipped(t *testing.T) {
	start := time.Now().UTC().Add(time.Hour)
	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:cancelled-1@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART:" + icsTime(start) + "\n" +
		"STATUS:CANCELLED\n" +
		"SUMMARY:Cancelled Meeting\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 0 {
		t.Errorf("got %d events, want 0 (event is STATUS:CANCELLED)", len(events))
	}
}

func TestFetchRecurrenceIDOverrideReplacesOccurrence(t *testing.T) {
	dtstart := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	overridden := dtstart.AddDate(0, 0, 1)
	movedStart := overridden.Add(3 * time.Hour)

	body := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//test//test//EN\n" +
		"BEGIN:VEVENT\n" +
		"UID:weekly-sync@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"DTSTART:" + icsTime(dtstart) + "\n" +
		"RRULE:FREQ=DAILY;COUNT=3\n" +
		"SUMMARY:Weekly Sync\n" +
		"END:VEVENT\n" +
		"BEGIN:VEVENT\n" +
		"UID:weekly-sync@example.com\n" +
		"DTSTAMP:" + icsTime(time.Now()) + "\n" +
		"RECURRENCE-ID:" + icsTime(overridden) + "\n" +
		"DTSTART:" + icsTime(movedStart) + "\n" +
		"SUMMARY:Weekly Sync (moved)\n" +
		"END:VEVENT\nEND:VCALENDAR\n"

	events := fetchEvents(t, serveICS(t, body))
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (3 occurrences, override not duplicated)", len(events))
	}

	var foundMoved bool
	for _, e := range events {
		if e.Start.Equal(overridden) {
			t.Error("original un-overridden occurrence is still present")
		}
		if e.Title == "Weekly Sync (moved)" {
			foundMoved = true
			if !e.Start.Equal(movedStart) {
				t.Errorf("moved occurrence Start = %v, want %v", e.Start, movedStart)
			}
		}
	}
	if !foundMoved {
		t.Error("overridden occurrence not found in results")
	}
}

func TestFetchMalformedICS(t *testing.T) {
	if _, err := serveICS(t, "this is not an ICS file at all").Fetch(context.Background()); err == nil {
		t.Error("Fetch with malformed ICS body: expected error, got nil")
	}
}

func TestFetchHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := New()
	if err := p.Configure(map[string]any{"url": srv.URL, "name": "Family"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if _, err := p.Fetch(context.Background()); err == nil {
		t.Error("Fetch with 404 response: expected error, got nil")
	}
}

func TestFetchNetworkTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer srv.Close()

	p := New()
	if err := p.Configure(map[string]any{"url": srv.URL, "name": "Family"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	p.httpClient.Timeout = 5 * time.Millisecond

	if _, err := p.Fetch(context.Background()); err == nil {
		t.Error("Fetch against a slow server: expected timeout error, got nil")
	}
}

func TestFetchSendsBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	var gotOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPass, gotOK = r.BasicAuth()
		w.Write([]byte(minimalValidICS()))
	}))
	defer srv.Close()

	p := New()
	if err := p.Configure(map[string]any{
		"url": srv.URL, "name": "Family", "username": "alice", "password": "s3cret",
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if _, err := p.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !gotOK || gotUser != "alice" || gotPass != "s3cret" {
		t.Errorf("BasicAuth = (%q, %q, %v), want (alice, s3cret, true)", gotUser, gotPass, gotOK)
	}
}

func TestFetchEmptyFeedReturnsNoEvents(t *testing.T) {
	events := fetchEvents(t, serveICS(t, minimalValidICS()))
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}
