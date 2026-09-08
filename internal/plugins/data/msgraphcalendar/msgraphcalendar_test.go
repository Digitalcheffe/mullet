package msgraphcalendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/shapes"
)

// fakeGraphServer mimics just enough of the real Microsoft Graph API
// (https://graph.microsoft.com/v1.0) for this plugin to run its full
// Configure/Fetch/Discover cycle against, without needing a real
// Microsoft account. The plugin talks to it via NewWithBaseURL,
// pointing the SDK's own RequestAdapter.SetBaseUrl at this server
// instead of intercepting HTTP transport.
func fakeGraphServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/me/calendars", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Calendars request Authorization = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"id": "cal-work", "name": "Work", "hexColor": "#4285F4"},
				{"id": "cal-family", "name": "Family", "hexColor": "#0F9D58"},
			},
		})
	})

	mux.HandleFunc("/me/calendars/cal-work/calendarView", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("startDateTime") == "" || r.URL.Query().Get("endDateTime") == "" {
			t.Error("calendarView request missing startDateTime/endDateTime query params")
		}
		if got := r.Header.Get("Prefer"); !strings.Contains(got, `outlook.timezone="UTC"`) {
			t.Errorf("Prefer header = %q, want it to request UTC", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{
					"id": "evt-1", "subject": "Standup", "isAllDay": false,
					"start":       map[string]string{"dateTime": "2026-01-01T09:00:00.0000000", "timeZone": "UTC"},
					"end":         map[string]string{"dateTime": "2026-01-01T09:30:00.0000000", "timeZone": "UTC"},
					"location":    map[string]string{"displayName": "Room 1"},
					"bodyPreview": "Daily sync",
				},
				{
					"id": "evt-2", "subject": "Offsite", "isAllDay": true,
					"start": map[string]string{"dateTime": "2026-01-05T00:00:00.0000000", "timeZone": "UTC"},
					"end":   map[string]string{"dateTime": "2026-01-06T00:00:00.0000000", "timeZone": "UTC"},
				},
			},
		})
	})

	mux.HandleFunc("/me/calendars/cal-family/calendarView", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{
					"id": "evt-3", "subject": "Dinner", "isAllDay": false,
					"start": map[string]string{"dateTime": "2026-01-02T18:00:00.0000000", "timeZone": "UTC"},
					"end":   map[string]string{"dateTime": "2026-01-02T19:00:00.0000000", "timeZone": "UTC"},
				},
			},
		})
	})

	return httptest.NewServer(mux)
}

func TestFetchAllCalendars(t *testing.T) {
	srv := fakeGraphServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL
	if err := p.Configure(map[string]any{"access_token": "test-token"}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	events := result["events"]
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (2 from work + 1 from family)", len(events))
	}

	byID := map[string]shapes.CalendarEvent{}
	for _, row := range events {
		e := row.(shapes.CalendarEvent)
		byID[e.ID] = e
	}

	standup, ok := byID["evt-1@cal-work"]
	if !ok {
		t.Fatalf("events = %+v, missing evt-1@cal-work", byID)
	}
	if standup.Title != "Standup" || standup.CalendarExternalID != "cal-work" || standup.CalendarName != "Work" {
		t.Errorf("standup = %+v, unexpected values", standup)
	}
	if standup.AllDay {
		t.Error("standup.AllDay = true, want false")
	}
	if standup.Location == nil || *standup.Location != "Room 1" {
		t.Errorf("standup.Location = %v, want Room 1", standup.Location)
	}
	if standup.Description == nil || *standup.Description != "Daily sync" {
		t.Errorf("standup.Description = %v, want Daily sync", standup.Description)
	}
	if standup.Start.Hour() != 9 || standup.Start.Minute() != 0 {
		t.Errorf("standup.Start = %v, want 09:00", standup.Start)
	}
	if standup.CalendarColor == nil || *standup.CalendarColor != "#4285F4" {
		t.Errorf("standup.CalendarColor = %v, want #4285F4", standup.CalendarColor)
	}

	offsite, ok := byID["evt-2@cal-work"]
	if !ok || !offsite.AllDay {
		t.Errorf("offsite = %+v (ok=%v), want AllDay=true", offsite, ok)
	}

	dinner, ok := byID["evt-3@cal-family"]
	if !ok || dinner.CalendarName != "Family" {
		t.Errorf("dinner = %+v (ok=%v), want CalendarName=Family", dinner, ok)
	}
}

func TestFetchFiltersToSelectedCalendars(t *testing.T) {
	srv := fakeGraphServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL
	if err := p.Configure(map[string]any{"access_token": "test-token", "calendars": []any{"cal-family"}}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	result, err := p.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	events := result["events"]
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (family calendar only)", len(events))
	}
	e := events[0].(shapes.CalendarEvent)
	if e.CalendarExternalID != "cal-family" {
		t.Errorf("CalendarExternalID = %q, want cal-family", e.CalendarExternalID)
	}
}

func TestConfigureRequiresAccessToken(t *testing.T) {
	p := New()
	if err := p.Configure(map[string]any{}); err == nil {
		t.Error("Configure with no access_token: expected error, got nil")
	}
}

func TestDiscoverCalendars(t *testing.T) {
	srv := fakeGraphServer(t)
	defer srv.Close()

	p := New()
	p.baseURL = srv.URL

	options, err := p.Discover(context.Background(), "calendars", map[string]any{"access_token": "test-token"})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(options) != 2 {
		t.Fatalf("got %d options, want 2", len(options))
	}
	if options[0].Value != "cal-work" || options[0].Label != "Work" {
		t.Errorf("options[0] = %+v, want {cal-work, Work}", options[0])
	}
}

func TestDiscoverUnknownFieldFails(t *testing.T) {
	p := New()
	if _, err := p.Discover(context.Background(), "not-a-real-field", map[string]any{"access_token": "x"}); err == nil {
		t.Error("Discover(unknown field): expected error, got nil")
	}
}

func TestManifestDeclaresOAuth2(t *testing.T) {
	m := New().Manifest()
	if m.AuthType != "oauth2" {
		t.Errorf("AuthType = %q, want oauth2", m.AuthType)
	}
	if m.OAuthConfig == nil || m.OAuthConfig.TenantField != "tenant" {
		t.Errorf("OAuthConfig = %+v, want TenantField=tenant", m.OAuthConfig)
	}
	found := false
	for _, f := range m.SetupFields {
		if f.Key == "calendars" {
			found = true
			if !f.Dynamic {
				t.Error("calendars field is not marked Dynamic")
			}
		}
	}
	if !found {
		t.Error("manifest has no \"calendars\" SetupField")
	}
}
