package db

import (
	"database/sql"
	"errors"
	"fmt"

	sqlite "modernc.org/sqlite"
)

// ErrInUse is returned by a delete operation blocked by a foreign key
// still referencing the row (e.g. deleting a theme a display still
// uses).
var ErrInUse = errors.New("in use")

// isForeignKeyViolation reports whether err is a SQLite FOREIGN KEY
// constraint failure -- from a delete that's still referenced, or an
// insert/update pointing at a row that doesn't exist. 19 is SQLite's
// stable, public SQLITE_CONSTRAINT primary result code; the driver's
// error carries an extended code with detail in the high bits, so this
// masks down to just the primary code.
func isForeignKeyViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	const sqliteConstraint = 19
	return sqliteErr.Code()&0xff == sqliteConstraint
}

// Theme is a named set of visual tokens (opaque JSON to this layer),
// assignable to a display and overridable per card.
type Theme struct {
	ID        int
	Name      string
	Tokens    string
	IsDefault bool
}

func scanTheme(row interface{ Scan(...any) error }) (Theme, error) {
	var t Theme
	var isDefault int
	if err := row.Scan(&t.ID, &t.Name, &t.Tokens, &isDefault); err != nil {
		return Theme{}, err
	}
	t.IsDefault = isDefault != 0
	return t, nil
}

const themeColumns = `id, name, tokens, is_default`

// GetTheme returns one theme by ID, or ErrNotFound.
func GetTheme(sqldb *sql.DB, id int) (Theme, error) {
	row := sqldb.QueryRow(`SELECT `+themeColumns+` FROM themes WHERE id = ?`, id)
	t, err := scanTheme(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Theme{}, ErrNotFound
	}
	if err != nil {
		return Theme{}, fmt.Errorf("getting theme %d: %w", id, err)
	}
	return t, nil
}

// ListThemes returns every theme, oldest first.
func ListThemes(sqldb *sql.DB) ([]Theme, error) {
	rows, err := sqldb.Query(`SELECT ` + themeColumns + ` FROM themes ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing themes: %w", err)
	}
	defer rows.Close()

	var themes []Theme
	for rows.Next() {
		t, err := scanTheme(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning theme: %w", err)
		}
		themes = append(themes, t)
	}
	return themes, rows.Err()
}

// CreateTheme inserts a new theme and returns its ID.
func CreateTheme(sqldb *sql.DB, name, tokens string, isDefault bool) (int, error) {
	result, err := sqldb.Exec(
		`INSERT INTO themes (name, tokens, is_default) VALUES (?, ?, ?)`,
		name, tokens, boolToInt(isDefault),
	)
	if err != nil {
		return 0, fmt.Errorf("creating theme: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reading new theme id: %w", err)
	}
	return int(id), nil
}

// UpdateTheme overwrites an existing theme. Returns ErrNotFound if id
// doesn't exist.
func UpdateTheme(sqldb *sql.DB, id int, name, tokens string, isDefault bool) error {
	result, err := sqldb.Exec(
		`UPDATE themes SET name = ?, tokens = ?, is_default = ? WHERE id = ?`,
		name, tokens, boolToInt(isDefault), id,
	)
	if err != nil {
		return fmt.Errorf("updating theme %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// DeleteTheme removes a theme. Returns ErrNotFound if id doesn't exist,
// or ErrInUse if a display still references it.
func DeleteTheme(sqldb *sql.DB, id int) error {
	result, err := sqldb.Exec(`DELETE FROM themes WHERE id = ?`, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("deleting theme %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// Display is a named physical output, routed at /display/{slug}.
type Display struct {
	ID              int
	Name            string
	Slug            string
	ThemeID         *int
	RotationSeconds int
	ShowTopBar      bool
	ShowBottomBar   bool
}

func scanDisplay(row interface{ Scan(...any) error }) (Display, error) {
	var d Display
	var showTopBar, showBottomBar int
	if err := row.Scan(&d.ID, &d.Name, &d.Slug, &d.ThemeID, &d.RotationSeconds, &showTopBar, &showBottomBar); err != nil {
		return Display{}, err
	}
	d.ShowTopBar = showTopBar != 0
	d.ShowBottomBar = showBottomBar != 0
	return d, nil
}

const displayColumns = `id, name, slug, theme_id, rotation_seconds, show_top_bar, show_bottom_bar`

// ListDisplays returns every display, oldest first.
func ListDisplays(sqldb *sql.DB) ([]Display, error) {
	rows, err := sqldb.Query(`SELECT ` + displayColumns + ` FROM displays ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listing displays: %w", err)
	}
	defer rows.Close()

	var displays []Display
	for rows.Next() {
		d, err := scanDisplay(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning display: %w", err)
		}
		displays = append(displays, d)
	}
	return displays, rows.Err()
}

// GetDisplay returns one display by ID, or ErrNotFound.
func GetDisplay(sqldb *sql.DB, id int) (Display, error) {
	row := sqldb.QueryRow(`SELECT `+displayColumns+` FROM displays WHERE id = ?`, id)
	d, err := scanDisplay(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Display{}, ErrNotFound
	}
	if err != nil {
		return Display{}, fmt.Errorf("getting display %d: %w", id, err)
	}
	return d, nil
}

// GetDisplayBySlug returns one display by its routing slug (the
// /display/{slug} path segment), or ErrNotFound.
func GetDisplayBySlug(sqldb *sql.DB, slug string) (Display, error) {
	row := sqldb.QueryRow(`SELECT `+displayColumns+` FROM displays WHERE slug = ?`, slug)
	d, err := scanDisplay(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Display{}, ErrNotFound
	}
	if err != nil {
		return Display{}, fmt.Errorf("getting display %q: %w", slug, err)
	}
	return d, nil
}

// CreateDisplay inserts a new display and returns its ID. Returns
// ErrInUse if slug is already taken, or if themeID is set but doesn't
// exist.
func CreateDisplay(sqldb *sql.DB, name, slug string, themeID *int, rotationSeconds int, showTopBar, showBottomBar bool) (int, error) {
	result, err := sqldb.Exec(
		`INSERT INTO displays (name, slug, theme_id, rotation_seconds, show_top_bar, show_bottom_bar) VALUES (?, ?, ?, ?, ?, ?)`,
		name, slug, themeID, rotationSeconds, boolToInt(showTopBar), boolToInt(showBottomBar),
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return 0, ErrInUse
		}
		return 0, fmt.Errorf("creating display: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reading new display id: %w", err)
	}
	return int(id), nil
}

// UpdateDisplay overwrites an existing display's editable fields.
// Returns ErrNotFound if id doesn't exist.
func UpdateDisplay(sqldb *sql.DB, id int, name, slug string, themeID *int, rotationSeconds int, showTopBar, showBottomBar bool) error {
	result, err := sqldb.Exec(
		`UPDATE displays SET name = ?, slug = ?, theme_id = ?, rotation_seconds = ?, show_top_bar = ?, show_bottom_bar = ? WHERE id = ?`,
		name, slug, themeID, rotationSeconds, boolToInt(showTopBar), boolToInt(showBottomBar), id,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("updating display %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// DeleteDisplay removes a display and, via ON DELETE CASCADE, its
// screens and their cards. Returns ErrNotFound if id doesn't exist.
func DeleteDisplay(sqldb *sql.DB, id int) error {
	result, err := sqldb.Exec(`DELETE FROM displays WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting display %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// Screen is a page within a display; a display rotates through its
// screens.
type Screen struct {
	ID         int
	DisplayID  int
	Name       string
	Position   int
	Columns    int
	RowHeight  int
	Gap        int
	LayoutMode string
}

func scanScreen(row interface{ Scan(...any) error }) (Screen, error) {
	var s Screen
	if err := row.Scan(&s.ID, &s.DisplayID, &s.Name, &s.Position, &s.Columns, &s.RowHeight, &s.Gap, &s.LayoutMode); err != nil {
		return Screen{}, err
	}
	return s, nil
}

const screenColumns = `id, display_id, name, position, columns, row_height, gap, layout_mode`

// ListScreensByDisplay returns every screen belonging to displayID, in
// rotation order.
func ListScreensByDisplay(sqldb *sql.DB, displayID int) ([]Screen, error) {
	rows, err := sqldb.Query(`SELECT `+screenColumns+` FROM screens WHERE display_id = ? ORDER BY position, id`, displayID)
	if err != nil {
		return nil, fmt.Errorf("listing screens for display %d: %w", displayID, err)
	}
	defer rows.Close()

	var screens []Screen
	for rows.Next() {
		s, err := scanScreen(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning screen: %w", err)
		}
		screens = append(screens, s)
	}
	return screens, rows.Err()
}

// GetScreen returns one screen by ID, or ErrNotFound.
func GetScreen(sqldb *sql.DB, id int) (Screen, error) {
	row := sqldb.QueryRow(`SELECT `+screenColumns+` FROM screens WHERE id = ?`, id)
	s, err := scanScreen(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Screen{}, ErrNotFound
	}
	if err != nil {
		return Screen{}, fmt.Errorf("getting screen %d: %w", id, err)
	}
	return s, nil
}

// CreateScreen inserts a new screen under displayID and returns its ID.
// Returns ErrInUse if displayID doesn't exist.
func CreateScreen(sqldb *sql.DB, displayID int, name string, position, columns, rowHeight, gap int, layoutMode string) (int, error) {
	result, err := sqldb.Exec(
		`INSERT INTO screens (display_id, name, position, columns, row_height, gap, layout_mode) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		displayID, name, position, columns, rowHeight, gap, layoutMode,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return 0, ErrInUse
		}
		return 0, fmt.Errorf("creating screen: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reading new screen id: %w", err)
	}
	return int(id), nil
}

// UpdateScreen overwrites an existing screen's editable fields (not
// including display_id -- a screen doesn't move between displays).
// Returns ErrNotFound if id doesn't exist.
func UpdateScreen(sqldb *sql.DB, id int, name string, position, columns, rowHeight, gap int, layoutMode string) error {
	result, err := sqldb.Exec(
		`UPDATE screens SET name = ?, position = ?, columns = ?, row_height = ?, gap = ?, layout_mode = ? WHERE id = ?`,
		name, position, columns, rowHeight, gap, layoutMode, id,
	)
	if err != nil {
		return fmt.Errorf("updating screen %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// DeleteScreen removes a screen and, via ON DELETE CASCADE, its cards.
// Returns ErrNotFound if id doesn't exist.
func DeleteScreen(sqldb *sql.DB, id int) error {
	result, err := sqldb.Exec(`DELETE FROM screens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting screen %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// Card is a positioned UI plugin on a screen's grid, optionally bound to
// a data plugin instance it reads from.
type Card struct {
	ID                   int
	ScreenID             int
	UIPluginID           string
	DataPluginInstanceID *int
	X, Y, W, H           int
	Config               string
	ThemeOverride        *string
}

func scanCard(row interface{ Scan(...any) error }) (Card, error) {
	var c Card
	if err := row.Scan(&c.ID, &c.ScreenID, &c.UIPluginID, &c.DataPluginInstanceID, &c.X, &c.Y, &c.W, &c.H, &c.Config, &c.ThemeOverride); err != nil {
		return Card{}, err
	}
	return c, nil
}

const cardColumns = `id, screen_id, ui_plugin_id, data_plugin_instance_id, x, y, w, h, config, theme_override`

// ListCardsByScreen returns every card placed on screenID.
func ListCardsByScreen(sqldb *sql.DB, screenID int) ([]Card, error) {
	rows, err := sqldb.Query(`SELECT `+cardColumns+` FROM cards WHERE screen_id = ? ORDER BY id`, screenID)
	if err != nil {
		return nil, fmt.Errorf("listing cards for screen %d: %w", screenID, err)
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning card: %w", err)
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// GetCard returns one card by ID, or ErrNotFound.
func GetCard(sqldb *sql.DB, id int) (Card, error) {
	row := sqldb.QueryRow(`SELECT `+cardColumns+` FROM cards WHERE id = ?`, id)
	c, err := scanCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, fmt.Errorf("getting card %d: %w", id, err)
	}
	return c, nil
}

// CreateCard inserts a new card on screenID and returns its ID. Returns
// ErrInUse if screenID or a non-nil dataPluginInstanceID doesn't exist.
func CreateCard(sqldb *sql.DB, screenID int, uiPluginID string, dataPluginInstanceID *int, x, y, w, h int, config string, themeOverride *string) (int, error) {
	result, err := sqldb.Exec(
		`INSERT INTO cards (screen_id, ui_plugin_id, data_plugin_instance_id, x, y, w, h, config, theme_override)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		screenID, uiPluginID, dataPluginInstanceID, x, y, w, h, config, themeOverride,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return 0, ErrInUse
		}
		return 0, fmt.Errorf("creating card: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reading new card id: %w", err)
	}
	return int(id), nil
}

// UpdateCard overwrites an existing card's editable fields (not
// including screen_id -- a card doesn't move between screens; remove
// and recreate it instead). Returns ErrNotFound if id doesn't exist, or
// ErrInUse if a non-nil dataPluginInstanceID doesn't exist.
func UpdateCard(sqldb *sql.DB, id int, uiPluginID string, dataPluginInstanceID *int, x, y, w, h int, config string, themeOverride *string) error {
	result, err := sqldb.Exec(
		`UPDATE cards SET ui_plugin_id = ?, data_plugin_instance_id = ?, x = ?, y = ?, w = ?, h = ?, config = ?, theme_override = ? WHERE id = ?`,
		uiPluginID, dataPluginInstanceID, x, y, w, h, config, themeOverride, id,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("updating card %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}

// DeleteCard removes a card. Returns ErrNotFound if id doesn't exist.
func DeleteCard(sqldb *sql.DB, id int) error {
	result, err := sqldb.Exec(`DELETE FROM cards WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting card %d: %w", id, err)
	}
	return checkRowsAffected(result, id)
}
