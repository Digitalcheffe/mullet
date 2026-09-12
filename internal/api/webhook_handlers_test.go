package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/db"
)

func TestWebhookConfigCRUD(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/webhook", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get (before save) status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var before webhookConfigResponse
	json.Unmarshal(rec.Body.Bytes(), &before)
	if before.HasSecret {
		t.Error("has_secret = true before anything was ever saved")
	}
	if before.PayloadTemplate != db.DefaultWebhookPayloadTemplate {
		t.Errorf("payload_template (before save) = %q, want the default", before.PayloadTemplate)
	}

	body, _ := json.Marshal(webhookConfigRequest{
		URL: "https://example.com/hook", Secret: "s3cret", PayloadTemplate: `{"e": "{event}"}`,
	})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/webhook", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/webhook", nil))
	var after webhookConfigResponse
	json.Unmarshal(rec.Body.Bytes(), &after)
	if after.URL != "https://example.com/hook" || !after.HasSecret || after.PayloadTemplate != `{"e": "{event}"}` {
		t.Errorf("get (after save) = %+v, unexpected values", after)
	}

	// Updating without a secret must not clear the one already saved.
	body, _ = json.Marshal(webhookConfigRequest{URL: "https://example.com/hook2"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/webhook", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put (no secret) status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/webhook", nil))
	json.Unmarshal(rec.Body.Bytes(), &after)
	if !after.HasSecret {
		t.Error("has_secret = false after an update that omitted the secret")
	}
	if after.PayloadTemplate != db.DefaultWebhookPayloadTemplate {
		t.Errorf("payload_template = %q, want reset to the default (omitted on that update)", after.PayloadTemplate)
	}
}

func TestPutWebhookConfigRejectsInvalidURL(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	cases := []string{"not a url", "ftp://example.com/hook", "javascript:alert(1)"}
	for _, u := range cases {
		body, _ := json.Marshal(webhookConfigRequest{URL: u})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/webhook", body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("url %q: status = %d, want 400", u, rec.Code)
		}
	}
}

func TestPutWebhookConfigRejectsInvalidPayloadTemplate(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(webhookConfigRequest{
		URL: "https://example.com/hook", PayloadTemplate: `{"e": {event}}`, // unquoted -- invalid JSON once rendered
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/webhook", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestTestWebhookNotConfiguredReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/settings/webhook/test", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestTestWebhookDeliversToConfiguredURL(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	received := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body, _ := json.Marshal(webhookConfigRequest{URL: srv.URL})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/webhook", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("saving config: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/settings/webhook/test", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	select {
	case got := <-received:
		var payload struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(got, &payload); err != nil || payload.Event != "test" {
			t.Errorf("received payload = %s, want event=test", got)
		}
	default:
		t.Error("webhook endpoint never received a request")
	}
}
