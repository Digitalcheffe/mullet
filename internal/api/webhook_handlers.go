package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/Digitalcheffe/mullet/internal/db"
	"github.com/Digitalcheffe/mullet/internal/notify"
)

type webhookConfigResponse struct {
	URL string `json:"url"`
	// HasSecret reports whether a signing secret is currently saved,
	// without ever sending the secret itself back to the browser.
	HasSecret       bool   `json:"has_secret"`
	PayloadTemplate string `json:"payload_template"`
}

// handleGetWebhookConfig returns the saved webhook configuration, or the
// zero value (with the default payload template) if nothing has been
// saved yet.
func handleGetWebhookConfig(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _, err := db.GetWebhookConfig(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(webhookConfigResponse{
			URL: cfg.URL, HasSecret: cfg.Secret != "", PayloadTemplate: cfg.PayloadTemplate,
		})
	}
}

type webhookConfigRequest struct {
	URL string `json:"url"`
	// Secret left empty keeps whatever secret is already saved -- same
	// reasoning as smtpConfigRequest.Password: the frontend never
	// receives the real secret back to resubmit.
	Secret string `json:"secret"`
	// PayloadTemplate left empty resets to db.DefaultWebhookPayloadTemplate
	// -- unlike Secret, this field is always visible in the Settings
	// textarea, so there's no write-only value to preserve by leaving it
	// out.
	PayloadTemplate string `json:"payload_template"`
}

// handlePutWebhookConfig saves the webhook configuration used to
// deliver notification events (issue #112).
func handlePutWebhookConfig(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req webhookConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.URL != "" {
			parsed, err := url.Parse(req.URL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
				http.Error(w, "url must be a valid http:// or https:// address", http.StatusBadRequest)
				return
			}
		}
		if req.PayloadTemplate != "" {
			if err := notify.ValidatePayloadTemplate(req.PayloadTemplate); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		err := db.SaveWebhookConfig(sqldb, db.WebhookConfig{
			URL: req.URL, Secret: req.Secret, PayloadTemplate: req.PayloadTemplate,
		})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleTestWebhook sends a one-off test payload using the currently
// saved webhook URL, so an admin can confirm it actually works before
// relying on it.
func handleTestWebhook(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := notify.TestWebhook(sqldb)
		if errors.Is(err, notify.ErrWebhookNotConfigured) {
			http.Error(w, "webhook isn't configured yet -- save a URL first", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "failed to deliver: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
