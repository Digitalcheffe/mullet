package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Digitalcheffe/mullet/internal/auth"
)

// enrollAndConfirmTOTP drives the full enroll -> confirm flow for the
// already-authenticated seeded admin, returning the confirmed secret
// and the backup codes shown at confirmation.
func enrollAndConfirmTOTP(t *testing.T, router http.Handler) (secret string, backupCodes []string) {
	t.Helper()

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/account/totp/enroll", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var enrolled totpEnrollResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &enrolled); err != nil {
		t.Fatalf("decoding enroll response: %v", err)
	}
	if enrolled.Secret == "" || enrolled.AuthURL == "" {
		t.Fatalf("enroll response = %+v, want a non-empty secret and auth_url", enrolled)
	}

	code, err := auth.CurrentTOTPCode(enrolled.Secret)
	if err != nil {
		t.Fatalf("CurrentTOTPCode: %v", err)
	}
	confirmBody, _ := json.Marshal(totpConfirmRequest{Code: code})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/account/totp/confirm", confirmBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var confirmed totpConfirmResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &confirmed); err != nil {
		t.Fatalf("decoding confirm response: %v", err)
	}
	if len(confirmed.BackupCodes) == 0 {
		t.Fatal("confirm response had no backup codes")
	}

	return enrolled.Secret, confirmed.BackupCodes
}

func TestEnrollTOTPRejectsWhenAlreadyEnabled(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	enrollAndConfirmTOTP(t, router)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/account/totp/enroll", nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("re-enroll status = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestConfirmTOTPRejectsWrongCode(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/account/totp/enroll", nil))
	var enrolled totpEnrollResponse
	json.Unmarshal(rec.Body.Bytes(), &enrolled)

	body, _ := json.Marshal(totpConfirmRequest{Code: "000000"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/account/totp/confirm", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	// Still not enabled after a failed confirmation.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/account/totp", nil))
	var status totpStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &status)
	if status.Enabled {
		t.Error("Enabled = true after a failed confirm attempt")
	}
}

func TestGetTOTPStatusReflectsEnrollment(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/account/totp", nil))
	var before totpStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &before)
	if before.Enabled || before.BackupCodesRemaining != 0 {
		t.Errorf("initial status = %+v, want disabled with no backup codes", before)
	}

	enrollAndConfirmTOTP(t, router)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/account/totp", nil))
	var after totpStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &after)
	if !after.Enabled || after.BackupCodesRemaining != 10 {
		t.Errorf("status after enrollment = %+v, want enabled with 10 backup codes", after)
	}
}

func TestLoginWithTOTPEnabledRequiresMFA(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	secret, _ := enrollAndConfirmTOTP(t, router)

	loginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var loginResp loginResponse
	json.Unmarshal(rec.Body.Bytes(), &loginResp)
	if !loginResp.MFARequired || loginResp.PendingToken == "" || loginResp.Token != "" {
		t.Fatalf("login response = %+v, want mfa_required with a pending_token and no token", loginResp)
	}

	// The pending token grants no access to an ordinary admin route.
	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.PendingToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("using the pending token at /api/admin/me: status = %d, want 401", rec.Code)
	}

	code, err := auth.CurrentTOTPCode(secret)
	if err != nil {
		t.Fatalf("CurrentTOTPCode: %v", err)
	}
	verifyBody, _ := json.Marshal(mfaVerifyRequest{PendingToken: loginResp.PendingToken, Code: code})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/mfa/verify", bytes.NewReader(verifyBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("mfa/verify status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var verifyResp loginResponse
	json.Unmarshal(rec.Body.Bytes(), &verifyResp)
	if verifyResp.Token == "" {
		t.Fatal("mfa/verify response had no session token")
	}

	// The new session token works normally.
	req = httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer "+verifyResp.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("using the session token at /api/admin/me: status = %d, want 200", rec.Code)
	}
}

func TestMFAVerifyRejectsWrongCode(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	enrollAndConfirmTOTP(t, router)

	loginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	var loginResp loginResponse
	json.Unmarshal(rec.Body.Bytes(), &loginResp)

	verifyBody, _ := json.Marshal(mfaVerifyRequest{PendingToken: loginResp.PendingToken, Code: "000000"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/mfa/verify", bytes.NewReader(verifyBody)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestMFAVerifyRejectsAnOrdinarySessionTokenAsPendingToken(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	loginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	var loginResp loginResponse
	json.Unmarshal(rec.Body.Bytes(), &loginResp)
	if loginResp.Token == "" {
		t.Fatal("expected a real session token (TOTP not enabled in this test)")
	}

	verifyBody, _ := json.Marshal(mfaVerifyRequest{PendingToken: loginResp.Token, Code: "000000"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/mfa/verify", bytes.NewReader(verifyBody)))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (a real session token must not work as a pending token)", rec.Code)
	}
}

func TestMFAVerifyWithBackupCodeConsumesIt(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	_, backupCodes := enrollAndConfirmTOTP(t, router)

	loginBody, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	var loginResp loginResponse
	json.Unmarshal(rec.Body.Bytes(), &loginResp)

	verifyBody, _ := json.Marshal(mfaVerifyRequest{PendingToken: loginResp.PendingToken, Code: backupCodes[0]})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/mfa/verify", bytes.NewReader(verifyBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("mfa/verify (backup code) status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	// The same backup code can't be reused for a second login.
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/admin/login", bytes.NewReader(loginBody)))
	var loginResp2 loginResponse
	json.Unmarshal(rec2.Body.Bytes(), &loginResp2)
	verifyBody2, _ := json.Marshal(mfaVerifyRequest{PendingToken: loginResp2.PendingToken, Code: backupCodes[0]})
	rec2 = httptest.NewRecorder()
	router.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/admin/mfa/verify", bytes.NewReader(verifyBody2)))
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("reusing a backup code: status = %d, want 401", rec2.Code)
	}
}

func TestDisableTOTPRequiresCorrectPassword(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	enrollAndConfirmTOTP(t, router)

	body, _ := json.Marshal(totpPasswordConfirmRequest{Password: "wrong-password"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/account/totp", body))
	// 403, not 401 -- see account_handlers.go's handleUpdateAccountPassword.
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}

	body, _ = json.Marshal(totpPasswordConfirmRequest{Password: "s3cret"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/account/totp", body))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/account/totp", nil))
	var status totpStatusResponse
	json.Unmarshal(rec.Body.Bytes(), &status)
	if status.Enabled {
		t.Error("Enabled = true after DELETE /api/admin/account/totp")
	}
}

func TestRegenerateBackupCodesInvalidatesOldOnes(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	_, oldCodes := enrollAndConfirmTOTP(t, router)

	body, _ := json.Marshal(totpPasswordConfirmRequest{Password: "s3cret"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/account/totp/backup-codes", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp totpConfirmResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.BackupCodes) != 10 || resp.BackupCodes[0] == oldCodes[0] {
		t.Errorf("regenerated codes = %+v, want a fresh set distinct from %+v", resp.BackupCodes, oldCodes)
	}
}
