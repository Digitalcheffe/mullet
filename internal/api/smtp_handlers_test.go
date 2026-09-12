package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// startFakeSMTPServer is a minimal plaintext SMTP server, just enough
// to let handleTestSMTP's send actually succeed end-to-end through the
// real internal/email.Send code path -- the mechanics of talking SMTP
// are already covered by internal/email's own tests; this only needs
// to accept the standard command sequence and say OK to each one.
func startFakeSMTPServer(t *testing.T) (host string, port int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting fake smtp server: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 fake.smtp ESMTP\r\n")
				inData := false
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					switch {
					case inData:
						if line == ".\r\n" {
							inData = false
							fmt.Fprint(conn, "250 OK\r\n")
						}
					case len(line) >= 4 && line[:4] == "DATA":
						inData = true
						fmt.Fprint(conn, "354 go ahead\r\n")
					case len(line) >= 4 && line[:4] == "QUIT":
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "250 OK\r\n")
					}
				}
			}()
		}
	}()

	h, p, _ := net.SplitHostPort(listener.Addr().String())
	portNum, _ := strconv.Atoi(p)
	return h, portNum
}

func TestSMTPConfigCRUD(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/smtp", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get (before save) status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var before smtpConfigResponse
	json.Unmarshal(rec.Body.Bytes(), &before)
	if before.HasPassword {
		t.Error("has_password = true before anything was ever saved")
	}

	body, _ := json.Marshal(smtpConfigRequest{
		Host: "smtp.example.com", Port: 587, Username: "mullet",
		Password: "s3cret", FromAddress: "mullet@example.com", TLSMode: "starttls",
	})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/smtp", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/smtp", nil))
	var after smtpConfigResponse
	json.Unmarshal(rec.Body.Bytes(), &after)
	if after.Host != "smtp.example.com" || after.Port != 587 || after.Username != "mullet" ||
		after.FromAddress != "mullet@example.com" || after.TLSMode != "starttls" || !after.HasPassword {
		t.Errorf("get (after save) = %+v, unexpected values", after)
	}

	// Updating without a password must not clear the one already saved.
	body, _ = json.Marshal(smtpConfigRequest{Host: "smtp2.example.com", Port: 465, FromAddress: "mullet@example.com", TLSMode: "tls"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/smtp", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("put (no password) status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/settings/smtp", nil))
	json.Unmarshal(rec.Body.Bytes(), &after)
	if !after.HasPassword {
		t.Error("has_password = false after an update that omitted the password")
	}
}

func TestPutSMTPConfigValidation(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	cases := []smtpConfigRequest{
		{Port: 587, FromAddress: "a@example.com", TLSMode: "starttls"},               // missing host
		{Host: "smtp.example.com", FromAddress: "a@example.com", TLSMode: "starttls"}, // missing port
		{Host: "smtp.example.com", Port: 587, TLSMode: "starttls"},                    // missing from_address
		{Host: "smtp.example.com", Port: 587, FromAddress: "a@example.com", TLSMode: "not-a-mode"},
	}
	for i, req := range cases {
		body, _ := json.Marshal(req)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/smtp", body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("case %d: status = %d, want 400 (body: %s)", i, rec.Code, rec.Body.String())
		}
	}
}

func TestTestSMTPNotConfiguredReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/settings/smtp/test", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestTestSMTPWithNoAddressAndNoAccountEmailReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	host, port := startFakeSMTPServer(t)

	body, _ := json.Marshal(smtpConfigRequest{Host: host, Port: port, FromAddress: "mullet@example.com", TLSMode: "none"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/smtp", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("saving config: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/settings/smtp/test", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestTestSMTPSendsToExplicitAddress(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	host, port := startFakeSMTPServer(t)

	body, _ := json.Marshal(smtpConfigRequest{Host: host, Port: port, FromAddress: "mullet@example.com", TLSMode: "none"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/settings/smtp", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("saving config: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	testBody, _ := json.Marshal(testEmailRequest{To: "someone@example.com"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/settings/smtp/test", testBody))
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}
