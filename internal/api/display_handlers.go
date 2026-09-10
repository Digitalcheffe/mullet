package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Digitalcheffe/mullet/internal/db"
)

// parseJSONObject validates raw as a JSON object, returning it as a
// string for storage. Absent input defaults to "{}".
func parseJSONObject(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("must be a JSON object")
	}
	return string(raw), nil
}

// parseOptionalJSONObject validates raw as a nullable JSON object:
// absent or JSON null both return (nil, nil); anything else must be a
// JSON object.
func parseOptionalJSONObject(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if _, err := parseJSONObject(raw); err != nil {
		return nil, fmt.Errorf("must be a JSON object or null")
	}
	s := string(raw)
	return &s, nil
}

// inUseError maps db.ErrInUse -- returned when a foreign key referenced
// a row that doesn't exist, or a delete is blocked by a referencing row
// -- to a client-facing 409/400.
func inUseError(w http.ResponseWriter, deleteContext string) {
	if deleteContext == "" {
		http.Error(w, "references a row that doesn't exist", http.StatusBadRequest)
		return
	}
	http.Error(w, deleteContext+" is still in use", http.StatusConflict)
}

// ---- Themes ----

type themeRequest struct {
	Name      string          `json:"name"`
	Tokens    json.RawMessage `json:"tokens"`
	IsDefault bool            `json:"is_default"`
}

type themeResponse struct {
	ID        int             `json:"id"`
	Name      string          `json:"name"`
	Tokens    json.RawMessage `json:"tokens"`
	IsDefault bool            `json:"is_default"`
}

func toThemeResponse(t db.Theme) themeResponse {
	tokens := t.Tokens
	if tokens == "" {
		tokens = "{}"
	}
	return themeResponse{ID: t.ID, Name: t.Name, Tokens: json.RawMessage(tokens), IsDefault: t.IsDefault}
}

func handleGetTheme(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		t, err := db.GetTheme(sqldb, id)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toThemeResponse(t))
	}
}

func handleListThemes(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		themes, err := db.ListThemes(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]themeResponse, len(themes))
		for i, t := range themes {
			resp[i] = toThemeResponse(t)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleCreateTheme(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req themeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		tokens, err := parseJSONObject(req.Tokens)
		if err != nil {
			http.Error(w, "tokens "+err.Error(), http.StatusBadRequest)
			return
		}

		id, err := db.CreateTheme(sqldb, req.Name, tokens, req.IsDefault)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeTheme(w, sqldb, id, http.StatusCreated)
	}
}

func handleUpdateTheme(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req themeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		tokens, err := parseJSONObject(req.Tokens)
		if err != nil {
			http.Error(w, "tokens "+err.Error(), http.StatusBadRequest)
			return
		}

		if err := db.UpdateTheme(sqldb, id, req.Name, tokens, req.IsDefault); errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeTheme(w, sqldb, id, http.StatusOK)
	}
}

func handleDeleteTheme(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch err := db.DeleteTheme(sqldb, id); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, db.ErrInUse):
			inUseError(w, "theme")
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func writeTheme(w http.ResponseWriter, sqldb *sql.DB, id int, status int) {
	t, err := db.GetTheme(sqldb, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toThemeResponse(t))
}

// ---- Displays ----

type displayRequest struct {
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	ThemeID         *int   `json:"theme_id"`
	RotationSeconds int    `json:"rotation_seconds"`
	ShowTopBar      bool   `json:"show_top_bar"`
	ShowBottomBar   bool   `json:"show_bottom_bar"`
}

type displayResponse struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	ThemeID         *int   `json:"theme_id,omitempty"`
	RotationSeconds int    `json:"rotation_seconds"`
	ShowTopBar      bool   `json:"show_top_bar"`
	ShowBottomBar   bool   `json:"show_bottom_bar"`
}

func toDisplayResponse(d db.Display) displayResponse {
	return displayResponse{
		ID: d.ID, Name: d.Name, Slug: d.Slug, ThemeID: d.ThemeID, RotationSeconds: d.RotationSeconds,
		ShowTopBar: d.ShowTopBar, ShowBottomBar: d.ShowBottomBar,
	}
}

func handleListDisplays(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		displays, err := db.ListDisplays(sqldb)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]displayResponse, len(displays))
		for i, d := range displays {
			resp[i] = toDisplayResponse(d)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleGetDisplay(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		d, err := db.GetDisplay(sqldb, id)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toDisplayResponse(d))
	}
}

func handleCreateDisplay(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req displayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Slug == "" {
			http.Error(w, "name and slug are required", http.StatusBadRequest)
			return
		}
		rotation := req.RotationSeconds
		if rotation <= 0 {
			rotation = 30
		}

		id, err := db.CreateDisplay(sqldb, req.Name, req.Slug, req.ThemeID, rotation, req.ShowTopBar, req.ShowBottomBar)
		switch {
		case errors.Is(err, db.ErrInUse):
			inUseError(w, "")
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeDisplay(w, sqldb, id, http.StatusCreated)
	}
}

func handleUpdateDisplay(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req displayRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Slug == "" {
			http.Error(w, "name and slug are required", http.StatusBadRequest)
			return
		}
		rotation := req.RotationSeconds
		if rotation <= 0 {
			rotation = 30
		}

		switch err := db.UpdateDisplay(sqldb, id, req.Name, req.Slug, req.ThemeID, rotation, req.ShowTopBar, req.ShowBottomBar); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
			return
		case errors.Is(err, db.ErrInUse):
			inUseError(w, "")
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeDisplay(w, sqldb, id, http.StatusOK)
	}
}

func handleDeleteDisplay(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch err := db.DeleteDisplay(sqldb, id); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func writeDisplay(w http.ResponseWriter, sqldb *sql.DB, id int, status int) {
	d, err := db.GetDisplay(sqldb, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toDisplayResponse(d))
}

// ---- Screens ----

// Layout modes (issue #73): "simple" is a small fixed grid with S/M/L
// size presets and no resize handles, aimed at someone who just wants a
// working dashboard fast; "freeform" is the original full-control grid
// (arbitrary columns/row_height/gap, 8-handle resize). Both store cards
// in the exact same x/y/w/h shape -- layout_mode only changes what the
// Designer's UI offers, never the display renderer or the data model.
const (
	layoutModeSimple   = "simple"
	layoutModeFreeform = "freeform"
)

// Simple mode's grid is fixed and never surfaced as a form field --
// picked once here rather than left to whatever an admin might type.
// Columns stays narrow (4) to keep the "just add widgets" experience
// uncluttered; row_height/gap are only ever used by the Designer's own
// pixel-based grid math (the live renderer uses CSS Grid fractions, not
// row_height at all -- see web/src/display/ScreenGrid.tsx), so these
// only affect how editing looks, not the final display.
const (
	simpleColumns   = 4
	simpleRowHeight = 90
	simpleGap       = 12
)

// Freeform's own defaults, used when a freeform screen's columns/
// row_height/gap are omitted -- including "Switch to advanced layout"
// itself (web/src/admin/pages/DesignerPage.tsx's SIMPLE_GRID/
// handleToggleLayoutMode has the matching freeform seed values). Started
// at 16/40/8; brought down to 8 columns after live testing, since the
// original density was exactly the kind of "overwhelming" issue #73 set
// out to avoid in the first place -- freeform is still real full control
// (arbitrary sizing, resize handles, columns/row_height/gap all
// editable), just not defaulting to the finest-grained version of it.
const (
	freeformColumns   = 8
	freeformRowHeight = 60
	freeformGap       = 10
)

type screenRequest struct {
	Name       string `json:"name"`
	Position   int    `json:"position"`
	Columns    int    `json:"columns"`
	RowHeight  int    `json:"row_height"`
	Gap        int    `json:"gap"`
	LayoutMode string `json:"layout_mode"`
}

type screenResponse struct {
	ID         int    `json:"id"`
	DisplayID  int    `json:"display_id"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	Columns    int    `json:"columns"`
	RowHeight  int    `json:"row_height"`
	Gap        int    `json:"gap"`
	LayoutMode string `json:"layout_mode"`
}

func toScreenResponse(s db.Screen) screenResponse {
	return screenResponse{
		ID: s.ID, DisplayID: s.DisplayID, Name: s.Name,
		Position: s.Position, Columns: s.Columns, RowHeight: s.RowHeight, Gap: s.Gap,
		LayoutMode: s.LayoutMode,
	}
}

// normalizeLayoutMode validates a request's layout_mode, defaulting an
// omitted one to "simple" -- the same default the DB column itself
// carries, kept in sync deliberately so a request that doesn't mention
// layout_mode at all (an old client, or a partial update) still lands on
// the same value the schema would pick on its own.
func normalizeLayoutMode(raw string) (string, error) {
	switch raw {
	case "":
		return layoutModeSimple, nil
	case layoutModeSimple, layoutModeFreeform:
		return raw, nil
	default:
		return "", fmt.Errorf("layout_mode must be %q or %q", layoutModeSimple, layoutModeFreeform)
	}
}

// screenDefaults fills in a screen's grid dimensions when the request
// left them unset, rather than creating (or leaving, on update) an
// unusable 0-column screen. Simple mode's defaults are its own fixed
// grid (see simpleColumns et al.); freeform's are the original 16/40/8
// -- either can still be overridden explicitly (a freeform screen isn't
// locked to 16 columns, and neither is simple locked to 4 if a caller
// has a reason not to).
//
// gap is treated differently depending on isCreate. There's no way to
// distinguish "omitted" from "explicitly 0" once JSON has decoded into a
// plain int, so on update (isCreate false) a 0 is trusted at face value
// -- the admin is editing a real numeric field (see the Designer's
// freeform grid editor) that already shows the current value, so a 0
// they typed there means it. On create there's no prior value for 0 to
// plausibly mean "keep it as it was" -- issue #73's own admin UI never
// asks for gap at all when creating a screen, so an omitted-and-thus-0
// gap on create almost always means "I have no opinion," and defaulting
// it the same as columns/row_height is what actually delivers a
// picked-once-internally, never-surfaced gap for a new simple screen.
func screenDefaults(req screenRequest, layoutMode string, isCreate bool) (columns, rowHeight, gap int) {
	columns, rowHeight, gap = req.Columns, req.RowHeight, req.Gap
	defaultColumns, defaultRowHeight, defaultGap := freeformColumns, freeformRowHeight, freeformGap
	if layoutMode == layoutModeSimple {
		defaultColumns, defaultRowHeight, defaultGap = simpleColumns, simpleRowHeight, simpleGap
	}
	if columns <= 0 {
		columns = defaultColumns
	}
	if rowHeight <= 0 {
		rowHeight = defaultRowHeight
	}
	if gap < 0 || (isCreate && gap == 0) {
		gap = defaultGap
	}
	return columns, rowHeight, gap
}

func handleGetScreen(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		s, err := db.GetScreen(sqldb, id)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(toScreenResponse(s))
	}
}

func handleListScreens(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		displayID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		screens, err := db.ListScreensByDisplay(sqldb, displayID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]screenResponse, len(screens))
		for i, s := range screens {
			resp[i] = toScreenResponse(s)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleCreateScreen(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		displayID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req screenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		layoutMode, err := normalizeLayoutMode(req.LayoutMode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		columns, rowHeight, gap := screenDefaults(req, layoutMode, true)

		id, err := db.CreateScreen(sqldb, displayID, req.Name, req.Position, columns, rowHeight, gap, layoutMode)
		switch {
		case errors.Is(err, db.ErrInUse):
			http.Error(w, "display not found", http.StatusBadRequest)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeScreen(w, sqldb, id, http.StatusCreated)
	}
}

func handleUpdateScreen(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req screenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		layoutMode, err := normalizeLayoutMode(req.LayoutMode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		columns, rowHeight, gap := screenDefaults(req, layoutMode, false)

		if err := db.UpdateScreen(sqldb, id, req.Name, req.Position, columns, rowHeight, gap, layoutMode); errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeScreen(w, sqldb, id, http.StatusOK)
	}
}

func handleDeleteScreen(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch err := db.DeleteScreen(sqldb, id); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func writeScreen(w http.ResponseWriter, sqldb *sql.DB, id int, status int) {
	s, err := db.GetScreen(sqldb, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toScreenResponse(s))
}

// ---- Cards ----

type cardRequest struct {
	UIPluginID           string          `json:"ui_plugin_id"`
	DataPluginInstanceID *int            `json:"data_plugin_instance_id"`
	X                    int             `json:"x"`
	Y                    int             `json:"y"`
	W                    int             `json:"w"`
	H                    int             `json:"h"`
	Config               json.RawMessage `json:"config"`
	ThemeOverride        json.RawMessage `json:"theme_override"`
}

type cardResponse struct {
	ID                   int             `json:"id"`
	ScreenID             int             `json:"screen_id"`
	UIPluginID           string          `json:"ui_plugin_id"`
	DataPluginInstanceID *int            `json:"data_plugin_instance_id,omitempty"`
	X                    int             `json:"x"`
	Y                    int             `json:"y"`
	W                    int             `json:"w"`
	H                    int             `json:"h"`
	Config               json.RawMessage `json:"config"`
	ThemeOverride        json.RawMessage `json:"theme_override,omitempty"`
}

func toCardResponse(c db.Card) cardResponse {
	config := c.Config
	if config == "" {
		config = "{}"
	}
	resp := cardResponse{
		ID: c.ID, ScreenID: c.ScreenID, UIPluginID: c.UIPluginID, DataPluginInstanceID: c.DataPluginInstanceID,
		X: c.X, Y: c.Y, W: c.W, H: c.H, Config: json.RawMessage(config),
	}
	if c.ThemeOverride != nil {
		resp.ThemeOverride = json.RawMessage(*c.ThemeOverride)
	}
	return resp
}

func handleListCards(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		screenID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		cards, err := db.ListCardsByScreen(sqldb, screenID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]cardResponse, len(cards))
		for i, c := range cards {
			resp[i] = toCardResponse(c)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func handleCreateCard(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		screenID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req cardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.UIPluginID == "" {
			http.Error(w, "ui_plugin_id is required", http.StatusBadRequest)
			return
		}
		config, err := parseJSONObject(req.Config)
		if err != nil {
			http.Error(w, "config "+err.Error(), http.StatusBadRequest)
			return
		}
		themeOverride, err := parseOptionalJSONObject(req.ThemeOverride)
		if err != nil {
			http.Error(w, "theme_override "+err.Error(), http.StatusBadRequest)
			return
		}

		id, err := db.CreateCard(sqldb, screenID, req.UIPluginID, req.DataPluginInstanceID, req.X, req.Y, req.W, req.H, config, themeOverride)
		switch {
		case errors.Is(err, db.ErrInUse):
			http.Error(w, "screen not found, or data_plugin_instance_id doesn't exist", http.StatusBadRequest)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeCard(w, sqldb, id, http.StatusCreated)
	}
}

func handleUpdateCard(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		var req cardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.UIPluginID == "" {
			http.Error(w, "ui_plugin_id is required", http.StatusBadRequest)
			return
		}
		config, err := parseJSONObject(req.Config)
		if err != nil {
			http.Error(w, "config "+err.Error(), http.StatusBadRequest)
			return
		}
		themeOverride, err := parseOptionalJSONObject(req.ThemeOverride)
		if err != nil {
			http.Error(w, "theme_override "+err.Error(), http.StatusBadRequest)
			return
		}

		switch err := db.UpdateCard(sqldb, id, req.UIPluginID, req.DataPluginInstanceID, req.X, req.Y, req.W, req.H, config, themeOverride); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
			return
		case errors.Is(err, db.ErrInUse):
			http.Error(w, "data_plugin_instance_id doesn't exist", http.StatusBadRequest)
			return
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		writeCard(w, sqldb, id, http.StatusOK)
	}
}

func handleDeleteCard(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch err := db.DeleteCard(sqldb, id); {
		case errors.Is(err, db.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case err != nil:
			http.Error(w, "internal error", http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func writeCard(w http.ResponseWriter, sqldb *sql.DB, id int, status int) {
	c, err := db.GetCard(sqldb, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(toCardResponse(c))
}

// ---- Display layout (public, no auth -- served to the display frontend
// itself, like /api/data) ----

type screenLayoutResponse struct {
	screenResponse
	Cards []cardResponse `json:"cards"`
}

type displayLayoutResponse struct {
	displayResponse
	Theme   json.RawMessage         `json:"theme,omitempty"`
	Screens []screenLayoutResponse `json:"screens"`
}

// handleGetDisplayLayout serves GET /api/display/{slug}: everything the
// display renderer needs for one display in a single request -- its own
// fields, its theme's tokens (resolved from theme_id; omitted if unset,
// letting the frontend fall back to its own built-in default so this
// endpoint doesn't have to duplicate that fallback), and every screen
// with its cards, in rotation order. No auth, same as /api/data -- a
// display is LAN-facing kiosk hardware, not an admin.
func handleGetDisplayLayout(sqldb *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		d, err := db.GetDisplayBySlug(sqldb, slug)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := displayLayoutResponse{displayResponse: toDisplayResponse(d)}
		if d.ThemeID != nil {
			theme, err := db.GetTheme(sqldb, *d.ThemeID)
			if err != nil && !errors.Is(err, db.ErrNotFound) {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if err == nil {
				resp.Theme = toThemeResponse(theme).Tokens
			}
		}

		screens, err := db.ListScreensByDisplay(sqldb, d.ID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp.Screens = make([]screenLayoutResponse, len(screens))
		for i, s := range screens {
			cards, err := db.ListCardsByScreen(sqldb, s.ID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			cardResp := make([]cardResponse, len(cards))
			for j, c := range cards {
				cardResp[j] = toCardResponse(c)
			}
			resp.Screens[i] = screenLayoutResponse{screenResponse: toScreenResponse(s), Cards: cardResp}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
