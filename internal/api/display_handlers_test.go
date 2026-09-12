package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestThemeCRUDEndpoints(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	if _, err := sqldb.Exec(`DELETE FROM themes`); err != nil {
		t.Fatalf("clearing seeded themes: %v", err)
	}

	body, _ := json.Marshal(themeRequest{Name: "Glass Dark", Tokens: json.RawMessage(`{"accentColor":"#4f9dff"}`), IsDefault: true})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/themes", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created themeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding create response: %v", err)
	}
	if created.Name != "Glass Dark" || !created.IsDefault || string(created.Tokens) != `{"accentColor":"#4f9dff"}` {
		t.Errorf("created = %+v, unexpected values", created)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/themes", nil))
	var list []themeResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("theme list has %d entries, want 1", len(list))
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/themes/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var fetched themeResponse
	json.Unmarshal(rec.Body.Bytes(), &fetched)
	if fetched.ID != created.ID || fetched.Name != "Glass Dark" {
		t.Errorf("fetched = %+v, unexpected values", fetched)
	}

	updateBody, _ := json.Marshal(themeRequest{Name: "Glass Light", Tokens: json.RawMessage(`{}`), IsDefault: false})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/themes/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated themeResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Name != "Glass Light" || updated.IsDefault {
		t.Errorf("updated = %+v, unexpected values", updated)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/themes/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestGetThemeMissingReturns404(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/themes/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCreateThemeRejectsMissingName(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(themeRequest{Name: ""})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/themes", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateThemeRejectsNonObjectTokens(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(themeRequest{Name: "Bad", Tokens: json.RawMessage(`"not an object"`)})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/themes", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDeleteThemeInUseReturns409(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	themeBody, _ := json.Marshal(themeRequest{Name: "Glass Dark"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/themes", themeBody))
	var theme themeResponse
	json.Unmarshal(rec.Body.Bytes(), &theme)

	displayBody, _ := json.Marshal(displayRequest{Name: "Kitchen", Slug: "kitchen", ThemeID: &theme.ID})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", displayBody))
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating display: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/themes/"+strconv.Itoa(theme.ID), nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("deleting in-use theme: status = %d, want 409", rec.Code)
	}
}

func TestDisplayCRUDEndpoints(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(displayRequest{Name: "Kitchen", Slug: "kitchen", ShowTopBar: true, ShowBottomBar: true})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created displayResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Name != "Kitchen" || created.Slug != "kitchen" || created.RotationSeconds != 30 {
		t.Errorf("created = %+v, unexpected values (want default rotation_seconds=30)", created)
	}
	if !created.ShowTopBar || !created.ShowBottomBar {
		t.Errorf("created bars = (%v, %v), want (true, true)", created.ShowTopBar, created.ShowBottomBar)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/displays", nil))
	var list []displayResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("display list has %d entries, want 1", len(list))
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/displays/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var fetched displayResponse
	json.Unmarshal(rec.Body.Bytes(), &fetched)
	if fetched.ID != created.ID || fetched.Name != created.Name || fetched.Slug != created.Slug {
		t.Errorf("get = %+v, want %+v", fetched, created)
	}

	updateBody, _ := json.Marshal(displayRequest{Name: "Office", Slug: "office", RotationSeconds: 60})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/displays/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated displayResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Name != "Office" || updated.Slug != "office" || updated.RotationSeconds != 60 {
		t.Errorf("updated = %+v, unexpected values", updated)
	}
	if updated.ShowTopBar || updated.ShowBottomBar {
		t.Errorf("updated bars = (%v, %v), want (false, false) since the update omitted them", updated.ShowTopBar, updated.ShowBottomBar)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/displays/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateDisplayRejectsMissingFields(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(displayRequest{Name: "Kitchen"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing slug: status = %d, want 400", rec.Code)
	}
}

func TestCreateDisplayDuplicateSlugRejected(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(displayRequest{Name: "Kitchen", Slug: "kitchen"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d, want 201", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", body))
	if rec.Code == http.StatusCreated {
		t.Error("duplicate slug: expected non-201 status")
	}
}

func TestUpdateDeleteMissingDisplayReturns404(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/displays/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("get missing: status = %d, want 404", rec.Code)
	}

	body, _ := json.Marshal(displayRequest{Name: "X", Slug: "x"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/displays/9999", body))
	if rec.Code != http.StatusNotFound {
		t.Errorf("update missing: status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/displays/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete missing: status = %d, want 404", rec.Code)
	}
}

// createTestDisplay is a test helper: creates a display via the HTTP API
// and returns its ID.
func createTestDisplay(t *testing.T, router http.Handler) int {
	t.Helper()
	body, _ := json.Marshal(displayRequest{Name: "Kitchen", Slug: "kitchen-" + t.Name()})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("createTestDisplay: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var d displayResponse
	json.Unmarshal(rec.Body.Bytes(), &d)
	return d.ID
}

func TestScreenCRUDEndpoints(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", Position: 0})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created screenResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	// Columns/RowHeight/Gap all default when omitted on create (0 isn't a
	// usable grid size, and there's no prior value for an omitted gap to
	// plausibly mean "leave it" the way it can on update -- see
	// screenDefaults' doc comment). layout_mode was omitted too, so this
	// exercises simple mode's own defaults (issue #73) -- see
	// TestCreateFreeformScreenDefaults for the freeform equivalent.
	if created.DisplayID != displayID || created.Name != "Main" || created.LayoutMode != "simple" || created.Columns != simpleColumns || created.RowHeight != simpleRowHeight || created.Gap != simpleGap {
		t.Errorf("created = %+v, unexpected values (want layout_mode=simple, columns=%d, row_height=%d, gap=%d defaults)", created, simpleColumns, simpleRowHeight, simpleGap)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", nil))
	var list []screenResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("screen list has %d entries, want 1", len(list))
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/screens/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var fetched screenResponse
	json.Unmarshal(rec.Body.Bytes(), &fetched)
	if fetched.ID != created.ID || fetched.Name != "Main" {
		t.Errorf("fetched = %+v, unexpected values", fetched)
	}

	updateBody, _ := json.Marshal(screenRequest{Name: "Detail", Position: 1, Columns: 12, RowHeight: 50, Gap: 10})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/screens/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated screenResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Name != "Detail" || updated.Columns != 12 {
		t.Errorf("updated = %+v, unexpected values", updated)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/screens/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestScreenThemeOverrideRoundTrip covers issue #87's screen-level font
// override, mirroring cards.theme_override's own nullable-JSON-object
// semantics (see parseOptionalJSONObject): set on create, present on
// fetch, and cleared by an update that omits it -- there's no
// partial-update path, so an update always fully overwrites.
func TestScreenThemeOverrideRoundTrip(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", ThemeOverride: json.RawMessage(`{"fontFamily":"'Poppins', sans-serif"}`)})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created screenResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	if string(created.ThemeOverride) != `{"fontFamily":"'Poppins', sans-serif"}` {
		t.Errorf("created.ThemeOverride = %s, want the fontFamily object", created.ThemeOverride)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/screens/"+strconv.Itoa(created.ID), nil))
	var fetched screenResponse
	json.Unmarshal(rec.Body.Bytes(), &fetched)
	if string(fetched.ThemeOverride) != `{"fontFamily":"'Poppins', sans-serif"}` {
		t.Errorf("fetched.ThemeOverride = %s, want it to persist", fetched.ThemeOverride)
	}

	updateBody, _ := json.Marshal(screenRequest{Name: "Main", Position: 0})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/screens/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated screenResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if len(updated.ThemeOverride) != 0 {
		t.Errorf("updated.ThemeOverride = %s, want cleared (update has no partial-update path)", updated.ThemeOverride)
	}
}

func TestCreateScreenThemeOverrideRejectsNonObject(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", ThemeOverride: json.RawMessage(`"not an object"`)})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateScreenNegativeGapDefaults(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", Gap: -1})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created screenResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Gap != simpleGap {
		t.Errorf("Gap = %d, want %d (negative gap should default to simple mode's own gap, since layout_mode was omitted)", created.Gap, simpleGap)
	}
}

// TestCreateFreeformScreenDefaults guards the other half of issue #73's
// default-picking logic: a screen explicitly created as "freeform" still
// gets the original 16/40/8 grid when its own columns/row_height/gap are
// omitted, unaffected by simple mode becoming the overall default.
func TestCreateFreeformScreenDefaults(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", LayoutMode: "freeform"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created screenResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	// Gap defaults too, same as columns/row_height -- omitted fields on
	// create all get a sensible starting value, freeform included (see
	// screenDefaults' doc comment for why create and update treat an
	// omitted/zero gap differently).
	if created.LayoutMode != "freeform" || created.Columns != freeformColumns || created.RowHeight != freeformRowHeight || created.Gap != freeformGap {
		t.Errorf("created = %+v, want layout_mode=freeform, columns=%d, row_height=%d, gap=%d", created, freeformColumns, freeformRowHeight, freeformGap)
	}
}

// TestCreateScreenInvalidLayoutModeReturns400 guards normalizeLayoutMode
// actually rejecting garbage rather than silently coercing it.
func TestCreateScreenInvalidLayoutModeReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", LayoutMode: "bogus"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestUpdateScreenChangesLayoutMode guards the "Switch to advanced
// layout" toggle's actual mechanism: a PUT that only changes
// layout_mode (columns/row_height/gap omitted, same as a real toggle
// click would send) should still update the stored mode, not silently
// reset it back to simple on every unrelated edit.
func TestUpdateScreenChangesLayoutMode(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	var created screenResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.LayoutMode != "simple" {
		t.Fatalf("created.LayoutMode = %q, want simple", created.LayoutMode)
	}

	updateBody, _ := json.Marshal(screenRequest{Name: "Main", LayoutMode: "freeform", Columns: created.Columns, RowHeight: created.RowHeight, Gap: created.Gap})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/screens/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated screenResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.LayoutMode != "freeform" {
		t.Errorf("updated.LayoutMode = %q, want freeform", updated.LayoutMode)
	}
}

// TestUpdateScreenRespectsExplicitZeroGap guards the other half of
// screenDefaults' create-vs-update distinction: once a screen exists,
// an admin explicitly setting gap to 0 (e.g. via the Designer's
// freeform grid editor, which shows the real current value rather than
// a blank field) must stick, not get silently defaulted the way an
// omitted gap on create does.
func TestUpdateScreenRespectsExplicitZeroGap(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	body, _ := json.Marshal(screenRequest{Name: "Main", LayoutMode: "freeform"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	var created screenResponse
	json.Unmarshal(rec.Body.Bytes(), &created)

	updateBody, _ := json.Marshal(screenRequest{Name: "Main", LayoutMode: "freeform", Columns: created.Columns, RowHeight: created.RowHeight, Gap: 0})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/screens/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated screenResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Gap != 0 {
		t.Errorf("updated.Gap = %d, want 0 (explicit gap=0 on update should stick, not default)", updated.Gap)
	}
}

func TestCreateScreenUnknownDisplayReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	body, _ := json.Marshal(screenRequest{Name: "Main"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/9999/screens", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGetScreenMissingReturns404(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/screens/9999", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// createTestScreen is a test helper: creates a display and one screen on
// it via the HTTP API, returning the screen's ID.
func createTestScreen(t *testing.T, router http.Handler) int {
	t.Helper()
	displayID := createTestDisplay(t, router)
	body, _ := json.Marshal(screenRequest{Name: "Main"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("createTestScreen: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var s screenResponse
	json.Unmarshal(rec.Body.Bytes(), &s)
	return s.ID
}

func TestCardCRUDEndpoints(t *testing.T) {
	router, sqldb := newTestRouter(t, nil)
	screenID := createTestScreen(t, router)
	instanceID, err := insertTestInstance(sqldb, "clock", "Kitchen Clock", 60, true, "{}")
	if err != nil {
		t.Fatalf("insertTestInstance: %v", err)
	}

	body, _ := json.Marshal(cardRequest{
		UIPluginID: "clock", DataPluginInstanceID: &instanceID,
		X: 1, Y: 1, W: 4, H: 3, Config: json.RawMessage(`{"format":"24h"}`),
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/screens/"+strconv.Itoa(screenID)+"/cards", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var created cardResponse
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.ScreenID != screenID || created.UIPluginID != "clock" || created.DataPluginInstanceID == nil || *created.DataPluginInstanceID != instanceID {
		t.Errorf("created = %+v, unexpected values", created)
	}
	if created.X != 1 || created.Y != 1 || created.W != 4 || created.H != 3 {
		t.Errorf("created position = (%d,%d,%d,%d), want (1,1,4,3)", created.X, created.Y, created.W, created.H)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodGet, "/api/admin/screens/"+strconv.Itoa(screenID)+"/cards", nil))
	var list []cardResponse
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("card list has %d entries, want 1", len(list))
	}

	updateBody, _ := json.Marshal(cardRequest{UIPluginID: "clock", X: 2, Y: 2, W: 5, H: 4})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/cards/"+strconv.Itoa(created.ID), updateBody))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var updated cardResponse
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.DataPluginInstanceID != nil {
		t.Errorf("updated.DataPluginInstanceID = %v, want nil (cleared by update)", updated.DataPluginInstanceID)
	}
	if updated.X != 2 || updated.W != 5 {
		t.Errorf("updated position = (%d,_,%d,_), want (2,_,5,_)", updated.X, updated.W)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/cards/"+strconv.Itoa(created.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestCreateCardRejectsMissingUIPluginID(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	screenID := createTestScreen(t, router)

	body, _ := json.Marshal(cardRequest{X: 1, Y: 1, W: 4, H: 3})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/screens/"+strconv.Itoa(screenID)+"/cards", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateCardUnknownDataPluginInstanceReturns400(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	screenID := createTestScreen(t, router)

	missing := 9999
	body, _ := json.Marshal(cardRequest{UIPluginID: "clock", DataPluginInstanceID: &missing, X: 1, Y: 1, W: 4, H: 3})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/screens/"+strconv.Itoa(screenID)+"/cards", body))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestDeletingDisplayCascadesToScreensAndCardsViaAPI(t *testing.T) {
	router, _ := newTestRouter(t, nil)
	displayID := createTestDisplay(t, router)

	screenBody, _ := json.Marshal(screenRequest{Name: "Main"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(displayID)+"/screens", screenBody))
	var screen screenResponse
	json.Unmarshal(rec.Body.Bytes(), &screen)

	cardBody, _ := json.Marshal(cardRequest{UIPluginID: "clock", X: 1, Y: 1, W: 4, H: 3})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/screens/"+strconv.Itoa(screen.ID)+"/cards", cardBody))
	var card cardResponse
	json.Unmarshal(rec.Body.Bytes(), &card)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodDelete, "/api/admin/displays/"+strconv.Itoa(displayID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("deleting display: status = %d, want 204", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/screens/"+strconv.Itoa(screen.ID), screenBody))
	if rec.Code != http.StatusNotFound {
		t.Errorf("screen after cascade delete: status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPut, "/api/admin/cards/"+strconv.Itoa(card.ID), cardBody))
	if rec.Code != http.StatusNotFound {
		t.Errorf("card after cascade delete: status = %d, want 404", rec.Code)
	}
}

func TestGetDisplayLayoutEndpoint(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	themeBody, _ := json.Marshal(themeRequest{Name: "Glass Dark", Tokens: json.RawMessage(`{"accentColor":"#4f9dff"}`)})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/themes", themeBody))
	var theme themeResponse
	json.Unmarshal(rec.Body.Bytes(), &theme)

	displayBody, _ := json.Marshal(displayRequest{
		Name: "Kitchen", Slug: "kitchen", ThemeID: &theme.ID, RotationSeconds: 45, ShowTopBar: true, ShowBottomBar: true,
	})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", displayBody))
	var display displayResponse
	json.Unmarshal(rec.Body.Bytes(), &display)

	screenBody, _ := json.Marshal(screenRequest{Name: "Main"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays/"+strconv.Itoa(display.ID)+"/screens", screenBody))
	var screen screenResponse
	json.Unmarshal(rec.Body.Bytes(), &screen)

	cardBody, _ := json.Marshal(cardRequest{UIPluginID: "mullet-weather-current", X: 1, Y: 1, W: 4, H: 3, Config: json.RawMessage(`{"unit":"°F"}`)})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/screens/"+strconv.Itoa(screen.ID)+"/cards", cardBody))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create card: status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	// The display layout endpoint itself takes no auth token -- it's
	// served to the display kiosk, not an admin.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/display/kitchen", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var layout displayLayoutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &layout); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if layout.Name != "Kitchen" || layout.Slug != "kitchen" || layout.RotationSeconds != 45 {
		t.Errorf("layout = %+v, unexpected values", layout)
	}
	if string(layout.Theme) != `{"accentColor":"#4f9dff"}` {
		t.Errorf("layout.Theme = %s, want the display's theme tokens", layout.Theme)
	}
	if len(layout.Screens) != 1 || len(layout.Screens[0].Cards) != 1 {
		t.Fatalf("layout.Screens = %+v, want 1 screen with 1 card", layout.Screens)
	}
	if layout.Screens[0].Cards[0].UIPluginID != "mullet-weather-current" {
		t.Errorf("card ui_plugin_id = %q, want mullet-weather-current", layout.Screens[0].Cards[0].UIPluginID)
	}
}

func TestGetDisplayLayoutUnknownSlugReturns404(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/display/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestGetDisplayLayoutOmitsThemeWhenUnset(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	displayBody, _ := json.Marshal(displayRequest{Name: "Office", Slug: "office"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(t, http.MethodPost, "/api/admin/displays", displayBody))

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/display/office", nil))
	var layout displayLayoutResponse
	json.Unmarshal(rec.Body.Bytes(), &layout)
	if layout.Theme != nil {
		t.Errorf("layout.Theme = %s, want nil (no theme_id set on the display)", layout.Theme)
	}
	if len(layout.Screens) != 0 {
		t.Errorf("layout.Screens = %+v, want empty (no screens created)", layout.Screens)
	}
}

func TestDisplayHierarchyEndpointsRequireAuth(t *testing.T) {
	router, _ := newTestRouter(t, nil)

	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/admin/themes", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/themes/1", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/displays", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/displays/1/screens", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/screens/1", nil),
		httptest.NewRequest(http.MethodGet, "/api/admin/screens/1/cards", nil),
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: status = %d, want 401", req.Method, req.URL.Path, rec.Code)
		}
	}
}
