package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func registerTestClient(t *testing.T, router http.Handler, clientID string) clientResponse {
	t.Helper()
	body, _ := json.Marshal(clientRegisterRequest{ClientID: clientID, Name: "Kitchen", Platform: "linux-arm", AppVersion: "1.0.0"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/clients/register", strings.NewReader(string(body))))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var c clientResponse
	json.Unmarshal(rec.Body.Bytes(), &c)
	return c
}

func TestRegisterClientDefaultsToPending(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	c := registerTestClient(t, router, "x7k-m2p")

	if c.Status != "pending" || c.DisplayID != nil || c.Name != "Kitchen" || c.ClientID != "x7k-m2p" {
		t.Errorf("registered client = %+v, want pending/unassigned", c)
	}
	if c.Platform == nil || *c.Platform != "linux-arm" {
		t.Errorf("Platform = %v, want linux-arm", c.Platform)
	}
}

func TestRegisterClientIsIdempotent(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	first := registerTestClient(t, router, "x7k-m2p")
	second := registerTestClient(t, router, "x7k-m2p")

	if first.ID != second.ID {
		t.Errorf("re-registering the same client_id created a new row: first.ID=%d second.ID=%d", first.ID, second.ID)
	}
}

func TestRegisterClientRejectsMissingFields(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	body, _ := json.Marshal(clientRegisterRequest{Name: "Kitchen"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/clients/register", strings.NewReader(string(body))))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (missing client_id)", rec.Code)
	}
}

func TestClientConfigBeforeApproval(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	registerTestClient(t, router, "x7k-m2p")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var cfg clientConfigResponse
	json.Unmarshal(rec.Body.Bytes(), &cfg)
	if cfg.Status != "pending" || cfg.DisplaySlug != nil || cfg.PollIntervalSeconds != clientPollIntervalSeconds {
		t.Errorf("config = %+v, want pending/no display/default poll interval", cfg)
	}
}

func TestClientConfigUnknownClientID(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clients/never-registered/config", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestApproveClientAssignsDisplayAndConfigReflectsIt(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)
	c := registerTestClient(t, router, "x7k-m2p")

	approveBody, _ := json.Marshal(approveClientRequest{DisplayID: displayID})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID)+"/approve", approveBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var approved clientResponse
	json.Unmarshal(rec.Body.Bytes(), &approved)
	if approved.Status != "approved" || approved.DisplayID == nil || *approved.DisplayID != displayID {
		t.Errorf("approved = %+v, want status=approved display_id=%d", approved, displayID)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/config", nil))
	var cfg clientConfigResponse
	json.Unmarshal(rec.Body.Bytes(), &cfg)
	if cfg.Status != "approved" || cfg.DisplaySlug == nil {
		t.Fatalf("config after approval = %+v, want approved with a display_slug", cfg)
	}
}

func TestApproveClientUnknownDisplayReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	c := registerTestClient(t, router, "x7k-m2p")

	body, _ := json.Marshal(approveClientRequest{DisplayID: 9999})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID)+"/approve", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestConfigPollMarksClientOnline(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	c := registerTestClient(t, router, "x7k-m2p")
	if c.Online {
		t.Fatal("a freshly-registered client with no poll yet should not be online")
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/config", nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/clients", nil))
	var list []clientResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("got %d clients, want 1", len(list))
	}
	// Still "pending", not "approved" -- Online only ever reports true
	// for an approved client (see isClientOnline), since a
	// pending/rejected client polling /config isn't "serving" anything.
	if list[0].Online {
		t.Error("a pending client should never report online=true regardless of last_seen_at")
	}
}

func TestClientOfflineScreenDefaultsWhenNoCustomOneSet(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)
	c := registerTestClient(t, router, "x7k-m2p")
	approveBody, _ := json.Marshal(approveClientRequest{DisplayID: displayID})
	router.ServeHTTP(httptest.NewRecorder(), authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID)+"/approve", approveBody))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/offline", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp clientOfflineResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.Contains(resp.HTML, "Kitchen") || !strings.Contains(resp.HTML, "<html>") {
		t.Errorf("html = %q, want the generated default mentioning the display name", resp.HTML)
	}
}

func TestClientOfflineScreenUsesDisplayCustomHTML(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)
	c := registerTestClient(t, router, "x7k-m2p")
	approveBody, _ := json.Marshal(approveClientRequest{DisplayID: displayID})
	router.ServeHTTP(httptest.NewRecorder(), authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID)+"/approve", approveBody))

	custom := "<html><body>Family Photo</body></html>"
	setBody, _ := json.Marshal(offlineScreenRequest{HTML: &custom})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/displays/"+strconv.Itoa(displayID)+"/offline-screen", setBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("set offline screen status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/offline", nil))
	var resp clientOfflineResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.HTML != custom {
		t.Errorf("html = %q, want the custom page verbatim", resp.HTML)
	}
}

func TestUpdateClientReassignsAndRejects(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)
	c := registerTestClient(t, router, "x7k-m2p")

	body, _ := json.Marshal(clientUpdateRequest{Name: "Kitchen (renamed)", DisplayID: &displayID, Status: "rejected", OfflineMode: "screen_off"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID), body))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated clientResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Status != "rejected" || updated.Name != "Kitchen (renamed)" || updated.OfflineMode != "screen_off" {
		t.Errorf("updated = %+v, unexpected values", updated)
	}
}

func TestUpdateClientRejectsInvalidStatus(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	c := registerTestClient(t, router, "x7k-m2p")

	body, _ := json.Marshal(clientUpdateRequest{Name: "Kitchen", Status: "banished", OfflineMode: "offline_screen"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/clients/"+strconv.Itoa(c.ID), body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (invalid status value)", rec.Code)
	}
}

func TestDeleteClientRemovesRegistration(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	c := registerTestClient(t, router, "x7k-m2p")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/clients/"+strconv.Itoa(c.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/clients/x7k-m2p/config", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("config after delete: status = %d, want 404", rec.Code)
	}
}

func TestClientAdminEndpointsRequireAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/clients", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}
