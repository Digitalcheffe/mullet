package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// capturingWebhookServer records every request it receives, so tests
// can assert on the delivered body and headers.
type capturingWebhookServer struct {
	*httptest.Server

	mu       sync.Mutex
	bodies   [][]byte
	sigs     []string
	statuses []int
}

func startCapturingWebhookServer(t *testing.T) *capturingWebhookServer {
	t.Helper()
	s := &capturingWebhookServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, body)
		s.sigs = append(s.sigs, r.Header.Get("X-Mullet-Signature"))
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *capturingWebhookServer) Bodies() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.bodies...)
}

func (s *capturingWebhookServer) Signatures() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.sigs...)
}

func newWebhookTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqldb, err := db.Open(filepath.Join(t.TempDir(), "mullet.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if err := db.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return sqldb
}

func TestDeliverWebhookNoopsWithoutConfigured(t *testing.T) {
	sqldb := newWebhookTestDB(t)
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{ClientApproved: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}
	if err := ClientApproved(sqldb, "Living Room TV"); err != nil {
		t.Errorf("ClientApproved without a webhook configured = %v, want nil (graceful no-op)", err)
	}
}

func TestWebhookDeliversWithSignature(t *testing.T) {
	srv := startCapturingWebhookServer(t)
	sqldb := newWebhookTestDB(t)
	if err := db.SaveWebhookConfig(sqldb, db.WebhookConfig{URL: srv.URL, Secret: "sign-me"}); err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{ClientApproved: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	if err := ClientApproved(sqldb, "Living Room TV"); err != nil {
		t.Fatalf("ClientApproved: %v", err)
	}

	bodies := srv.Bodies()
	if len(bodies) != 1 {
		t.Fatalf("Bodies = %d, want exactly 1", len(bodies))
	}

	var payload struct {
		Event   string `json:"event"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(bodies[0], &payload); err != nil {
		t.Fatalf("delivered body isn't valid JSON: %v\nbody: %s", err, bodies[0])
	}
	if payload.Event != "client_approved" {
		t.Errorf("event = %q, want client_approved", payload.Event)
	}
	if payload.Subject != "Client approved" {
		t.Errorf("subject = %q, want Client approved", payload.Subject)
	}

	mac := hmac.New(sha256.New, []byte("sign-me"))
	mac.Write(bodies[0])
	wantSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := srv.Signatures()[0]; got != wantSig {
		t.Errorf("X-Mullet-Signature = %q, want %q", got, wantSig)
	}
}

func TestWebhookWithoutSecretOmitsSignatureHeader(t *testing.T) {
	srv := startCapturingWebhookServer(t)
	sqldb := newWebhookTestDB(t)
	if err := db.SaveWebhookConfig(sqldb, db.WebhookConfig{URL: srv.URL}); err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{ClientApproved: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	if err := ClientApproved(sqldb, "Living Room TV"); err != nil {
		t.Fatalf("ClientApproved: %v", err)
	}
	if sig := srv.Signatures()[0]; sig != "" {
		t.Errorf("X-Mullet-Signature = %q, want empty (no secret configured)", sig)
	}
}

func TestCustomPayloadTemplateIsUsed(t *testing.T) {
	srv := startCapturingWebhookServer(t)
	sqldb := newWebhookTestDB(t)
	if err := db.SaveWebhookConfig(sqldb, db.WebhookConfig{
		URL:             srv.URL,
		PayloadTemplate: `{"custom_event": "{event}", "note": "got: {message}"}`,
	}); err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}
	if err := db.SaveNotificationPreferences(sqldb, db.NotificationPreferences{ClientApproved: true}); err != nil {
		t.Fatalf("SaveNotificationPreferences: %v", err)
	}

	if err := ClientApproved(sqldb, "Living Room TV"); err != nil {
		t.Fatalf("ClientApproved: %v", err)
	}

	var payload struct {
		CustomEvent string `json:"custom_event"`
		Note        string `json:"note"`
	}
	if err := json.Unmarshal(srv.Bodies()[0], &payload); err != nil {
		t.Fatalf("delivered body isn't valid JSON: %v\nbody: %s", err, srv.Bodies()[0])
	}
	if payload.CustomEvent != "client_approved" {
		t.Errorf("custom_event = %q, want client_approved", payload.CustomEvent)
	}
	if payload.Note == "" || payload.Note == "got: " {
		t.Errorf("note = %q, want the event message spliced in", payload.Note)
	}
}

func TestTestWebhookNotConfiguredReturnsErr(t *testing.T) {
	sqldb := newWebhookTestDB(t)
	if err := TestWebhook(sqldb); err != ErrWebhookNotConfigured {
		t.Errorf("TestWebhook (unconfigured) = %v, want ErrWebhookNotConfigured", err)
	}
}

func TestTestWebhookDeliversToConfiguredURL(t *testing.T) {
	srv := startCapturingWebhookServer(t)
	sqldb := newWebhookTestDB(t)
	if err := db.SaveWebhookConfig(sqldb, db.WebhookConfig{URL: srv.URL}); err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}
	if err := TestWebhook(sqldb); err != nil {
		t.Fatalf("TestWebhook: %v", err)
	}
	if len(srv.Bodies()) != 1 {
		t.Fatalf("Bodies = %d, want exactly 1", len(srv.Bodies()))
	}
}

func TestValidatePayloadTemplate(t *testing.T) {
	cases := []struct {
		name    string
		tmpl    string
		wantErr bool
	}{
		{"default template", db.DefaultWebhookPayloadTemplate, false},
		{"custom valid template", `{"e": "{event}", "m": "{message}"}`, false},
		{"unquoted variable breaks json", `{"e": {event}}`, true},
		{"not json at all", `plain text {event}`, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePayloadTemplate(tt.tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePayloadTemplate(%q) = %v, wantErr=%v", tt.tmpl, err, tt.wantErr)
			}
		})
	}
}

func TestRenderPayloadTemplateEscapesSpecialCharacters(t *testing.T) {
	rendered := RenderPayloadTemplate(db.DefaultWebhookPayloadTemplate, "test",
		`a "quoted" subject`, "a message\nwith a newline and a \\ backslash", time.Now())

	var payload struct {
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(rendered), &payload); err != nil {
		t.Fatalf("rendered template isn't valid JSON: %v\nrendered: %s", err, rendered)
	}
	if payload.Subject != `a "quoted" subject` {
		t.Errorf("Subject round-tripped as %q", payload.Subject)
	}
	if payload.Message != "a message\nwith a newline and a \\ backslash" {
		t.Errorf("Message round-tripped as %q", payload.Message)
	}
}
