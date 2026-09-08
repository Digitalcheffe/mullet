package db

import (
	"errors"
	"testing"
)

func TestThemeCRUD(t *testing.T) {
	sqldb := newTestDB(t)

	id, err := CreateTheme(sqldb, "Glass Dark", `{"accentColor":"#4f9dff"}`, true)
	if err != nil {
		t.Fatalf("CreateTheme: %v", err)
	}

	themes, err := ListThemes(sqldb)
	if err != nil {
		t.Fatalf("ListThemes: %v", err)
	}
	if len(themes) != 1 || themes[0].Name != "Glass Dark" || !themes[0].IsDefault {
		t.Errorf("themes = %+v, unexpected values", themes)
	}

	if err := UpdateTheme(sqldb, id, "Glass Light", `{"accentColor":"#000"}`, false); err != nil {
		t.Fatalf("UpdateTheme: %v", err)
	}
	themes, _ = ListThemes(sqldb)
	if themes[0].Name != "Glass Light" || themes[0].IsDefault {
		t.Errorf("after update: themes = %+v, unexpected values", themes)
	}

	if err := DeleteTheme(sqldb, id); err != nil {
		t.Fatalf("DeleteTheme: %v", err)
	}
	themes, _ = ListThemes(sqldb)
	if len(themes) != 0 {
		t.Errorf("themes after delete = %+v, want empty", themes)
	}
}

func TestUpdateDeleteMissingThemeReturnsErrNotFound(t *testing.T) {
	sqldb := newTestDB(t)

	if err := UpdateTheme(sqldb, 9999, "X", "{}", false); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateTheme(missing) = %v, want ErrNotFound", err)
	}
	if err := DeleteTheme(sqldb, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteTheme(missing) = %v, want ErrNotFound", err)
	}
}

func TestDeleteThemeInUseReturnsErrInUse(t *testing.T) {
	sqldb := newTestDB(t)

	themeID, err := CreateTheme(sqldb, "Glass Dark", "{}", true)
	if err != nil {
		t.Fatalf("CreateTheme: %v", err)
	}
	if _, err := CreateDisplay(sqldb, "Kitchen", "kitchen", &themeID, 30, true, true); err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}

	if err := DeleteTheme(sqldb, themeID); !errors.Is(err, ErrInUse) {
		t.Errorf("DeleteTheme(in use) = %v, want ErrInUse", err)
	}
}

func TestDisplayCRUD(t *testing.T) {
	sqldb := newTestDB(t)

	id, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true)
	if err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}

	got, err := GetDisplay(sqldb, id)
	if err != nil {
		t.Fatalf("GetDisplay: %v", err)
	}
	if got.Name != "Kitchen" || got.Slug != "kitchen" || got.RotationSeconds != 30 || got.ThemeID != nil {
		t.Errorf("GetDisplay = %+v, unexpected values", got)
	}
	if !got.ShowTopBar || !got.ShowBottomBar {
		t.Errorf("GetDisplay bars = (%v, %v), want (true, true)", got.ShowTopBar, got.ShowBottomBar)
	}

	displays, err := ListDisplays(sqldb)
	if err != nil {
		t.Fatalf("ListDisplays: %v", err)
	}
	if len(displays) != 1 {
		t.Fatalf("displays = %+v, want 1 entry", displays)
	}

	if err := UpdateDisplay(sqldb, id, "Office", "office", nil, 60, false, false); err != nil {
		t.Fatalf("UpdateDisplay: %v", err)
	}
	got, _ = GetDisplay(sqldb, id)
	if got.Name != "Office" || got.Slug != "office" || got.RotationSeconds != 60 {
		t.Errorf("after update: GetDisplay = %+v, unexpected values", got)
	}
	if got.ShowTopBar || got.ShowBottomBar {
		t.Errorf("after update: bars = (%v, %v), want (false, false)", got.ShowTopBar, got.ShowBottomBar)
	}

	if err := DeleteDisplay(sqldb, id); err != nil {
		t.Fatalf("DeleteDisplay: %v", err)
	}
	if _, err := GetDisplay(sqldb, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetDisplay after delete = %v, want ErrNotFound", err)
	}
}

func TestCreateDisplayDuplicateSlugFails(t *testing.T) {
	sqldb := newTestDB(t)

	if _, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true); err != nil {
		t.Fatalf("CreateDisplay (first): %v", err)
	}
	if _, err := CreateDisplay(sqldb, "Kitchen 2", "kitchen", nil, 30, true, true); err == nil {
		t.Error("CreateDisplay with duplicate slug: expected error, got nil")
	}
}

func TestCreateDisplayUnknownThemeReturnsErrInUse(t *testing.T) {
	sqldb := newTestDB(t)

	missing := 9999
	if _, err := CreateDisplay(sqldb, "Kitchen", "kitchen", &missing, 30, true, true); !errors.Is(err, ErrInUse) {
		t.Errorf("CreateDisplay(unknown theme) = %v, want ErrInUse", err)
	}
}

func TestUpdateDeleteMissingDisplayReturnsErrNotFound(t *testing.T) {
	sqldb := newTestDB(t)

	if err := UpdateDisplay(sqldb, 9999, "X", "x", nil, 30, true, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateDisplay(missing) = %v, want ErrNotFound", err)
	}
	if err := DeleteDisplay(sqldb, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteDisplay(missing) = %v, want ErrNotFound", err)
	}
}

func TestScreenCRUD(t *testing.T) {
	sqldb := newTestDB(t)

	displayID, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true)
	if err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}

	id, err := CreateScreen(sqldb, displayID, "Main", 0, 16, 40, 8)
	if err != nil {
		t.Fatalf("CreateScreen: %v", err)
	}

	got, err := GetScreen(sqldb, id)
	if err != nil {
		t.Fatalf("GetScreen: %v", err)
	}
	if got.DisplayID != displayID || got.Name != "Main" || got.Columns != 16 {
		t.Errorf("GetScreen = %+v, unexpected values", got)
	}

	screens, err := ListScreensByDisplay(sqldb, displayID)
	if err != nil {
		t.Fatalf("ListScreensByDisplay: %v", err)
	}
	if len(screens) != 1 {
		t.Fatalf("screens = %+v, want 1 entry", screens)
	}

	if err := UpdateScreen(sqldb, id, "Detail", 1, 12, 50, 10); err != nil {
		t.Fatalf("UpdateScreen: %v", err)
	}
	got, _ = GetScreen(sqldb, id)
	if got.Name != "Detail" || got.Position != 1 || got.Columns != 12 || got.RowHeight != 50 || got.Gap != 10 {
		t.Errorf("after update: GetScreen = %+v, unexpected values", got)
	}

	if err := DeleteScreen(sqldb, id); err != nil {
		t.Fatalf("DeleteScreen: %v", err)
	}
	if _, err := GetScreen(sqldb, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetScreen after delete = %v, want ErrNotFound", err)
	}
}

func TestCreateScreenUnknownDisplayReturnsErrInUse(t *testing.T) {
	sqldb := newTestDB(t)

	if _, err := CreateScreen(sqldb, 9999, "Main", 0, 16, 40, 8); !errors.Is(err, ErrInUse) {
		t.Errorf("CreateScreen(unknown display) = %v, want ErrInUse", err)
	}
}

func TestUpdateDeleteMissingScreenReturnsErrNotFound(t *testing.T) {
	sqldb := newTestDB(t)

	if err := UpdateScreen(sqldb, 9999, "X", 0, 16, 40, 8); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateScreen(missing) = %v, want ErrNotFound", err)
	}
	if err := DeleteScreen(sqldb, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteScreen(missing) = %v, want ErrNotFound", err)
	}
}

func TestDeletingDisplayCascadesToScreensAndCards(t *testing.T) {
	sqldb := newTestDB(t)

	displayID, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true)
	if err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}
	screenID, err := CreateScreen(sqldb, displayID, "Main", 0, 16, 40, 8)
	if err != nil {
		t.Fatalf("CreateScreen: %v", err)
	}
	cardID, err := CreateCard(sqldb, screenID, "clock", nil, 1, 1, 4, 3, "{}", nil)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if err := DeleteDisplay(sqldb, displayID); err != nil {
		t.Fatalf("DeleteDisplay: %v", err)
	}

	if _, err := GetScreen(sqldb, screenID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetScreen after display delete = %v, want ErrNotFound (cascade)", err)
	}
	if _, err := GetCard(sqldb, cardID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetCard after display delete = %v, want ErrNotFound (cascade)", err)
	}
}

func TestCardCRUD(t *testing.T) {
	sqldb := newTestDB(t)

	displayID, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true)
	if err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}
	screenID, err := CreateScreen(sqldb, displayID, "Main", 0, 16, 40, 8)
	if err != nil {
		t.Fatalf("CreateScreen: %v", err)
	}
	instanceID, err := CreatePluginInstance(sqldb, "openweathermap", "Home", 900, false, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}

	override := `{"accentColor":"#f00"}`
	id, err := CreateCard(sqldb, screenID, "weather-forecast", &instanceID, 1, 1, 8, 4, `{"days":5}`, &override)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	got, err := GetCard(sqldb, id)
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if got.ScreenID != screenID || got.UIPluginID != "weather-forecast" || got.DataPluginInstanceID == nil || *got.DataPluginInstanceID != instanceID {
		t.Errorf("GetCard = %+v, unexpected values", got)
	}
	if got.X != 1 || got.Y != 1 || got.W != 8 || got.H != 4 {
		t.Errorf("GetCard position = (%d,%d,%d,%d), want (1,1,8,4)", got.X, got.Y, got.W, got.H)
	}
	if got.ThemeOverride == nil || *got.ThemeOverride != override {
		t.Errorf("GetCard.ThemeOverride = %v, want %q", got.ThemeOverride, override)
	}

	cards, err := ListCardsByScreen(sqldb, screenID)
	if err != nil {
		t.Fatalf("ListCardsByScreen: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("cards = %+v, want 1 entry", cards)
	}

	if err := UpdateCard(sqldb, id, "weather-forecast", nil, 2, 2, 6, 3, `{"days":3}`, nil); err != nil {
		t.Fatalf("UpdateCard: %v", err)
	}
	got, _ = GetCard(sqldb, id)
	if got.DataPluginInstanceID != nil {
		t.Errorf("GetCard.DataPluginInstanceID = %v, want nil after update", got.DataPluginInstanceID)
	}
	if got.X != 2 || got.Y != 2 || got.W != 6 || got.H != 3 {
		t.Errorf("after update: position = (%d,%d,%d,%d), want (2,2,6,3)", got.X, got.Y, got.W, got.H)
	}
	if got.ThemeOverride != nil {
		t.Errorf("GetCard.ThemeOverride = %v, want nil after update", got.ThemeOverride)
	}

	if err := DeleteCard(sqldb, id); err != nil {
		t.Fatalf("DeleteCard: %v", err)
	}
	if _, err := GetCard(sqldb, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetCard after delete = %v, want ErrNotFound", err)
	}
}

func TestCreateCardUnknownScreenReturnsErrInUse(t *testing.T) {
	sqldb := newTestDB(t)

	if _, err := CreateCard(sqldb, 9999, "clock", nil, 1, 1, 4, 3, "{}", nil); !errors.Is(err, ErrInUse) {
		t.Errorf("CreateCard(unknown screen) = %v, want ErrInUse", err)
	}
}

func TestCreateCardUnknownDataPluginInstanceReturnsErrInUse(t *testing.T) {
	sqldb := newTestDB(t)

	displayID, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true)
	if err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}
	screenID, err := CreateScreen(sqldb, displayID, "Main", 0, 16, 40, 8)
	if err != nil {
		t.Fatalf("CreateScreen: %v", err)
	}

	missing := 9999
	if _, err := CreateCard(sqldb, screenID, "weather-forecast", &missing, 1, 1, 4, 3, "{}", nil); !errors.Is(err, ErrInUse) {
		t.Errorf("CreateCard(unknown data plugin instance) = %v, want ErrInUse", err)
	}
}

func TestUpdateDeleteMissingCardReturnsErrNotFound(t *testing.T) {
	sqldb := newTestDB(t)

	if err := UpdateCard(sqldb, 9999, "clock", nil, 1, 1, 4, 3, "{}", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateCard(missing) = %v, want ErrNotFound", err)
	}
	if err := DeleteCard(sqldb, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteCard(missing) = %v, want ErrNotFound", err)
	}
}

func TestDeletingDataPluginInstanceNullsCardReference(t *testing.T) {
	sqldb := newTestDB(t)

	displayID, err := CreateDisplay(sqldb, "Kitchen", "kitchen", nil, 30, true, true)
	if err != nil {
		t.Fatalf("CreateDisplay: %v", err)
	}
	screenID, err := CreateScreen(sqldb, displayID, "Main", 0, 16, 40, 8)
	if err != nil {
		t.Fatalf("CreateScreen: %v", err)
	}
	instanceID, err := CreatePluginInstance(sqldb, "openweathermap", "Home", 900, false, "{}")
	if err != nil {
		t.Fatalf("CreatePluginInstance: %v", err)
	}
	cardID, err := CreateCard(sqldb, screenID, "weather-forecast", &instanceID, 1, 1, 4, 3, "{}", nil)
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if err := DeletePluginInstance(sqldb, instanceID); err != nil {
		t.Fatalf("DeletePluginInstance: %v", err)
	}

	got, err := GetCard(sqldb, cardID)
	if err != nil {
		t.Fatalf("GetCard after data plugin instance delete: %v", err)
	}
	if got.DataPluginInstanceID != nil {
		t.Errorf("GetCard.DataPluginInstanceID = %v, want nil (SET NULL, card should survive)", got.DataPluginInstanceID)
	}
}
