package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func testLeaderMenuApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	screen := newScreen(t, 80, 50)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  config.Paths{ConfigDir: filepath.Join(dir, "config")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	return app
}

func TestOpenBuiltinLeaderMenuShowsActionsNotUserMenu(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser

	app.openBuiltinLeaderMenu()

	if !app.model.LeaderMenu.Open {
		t.Fatal("expected built-in leader menu open")
	}
	if app.model.LeaderMenu.UserMenu {
		t.Fatal("expected UserMenu=false for Esc menu")
	}
	if len(app.model.LeaderMenu.Items) == 0 {
		t.Fatal("expected built-in leader menu items")
	}
	foundMove := false
	foundMkdir := false
	foundFileGroup := false
	for _, it := range app.model.LeaderMenu.Items {
		if it.GroupTitle == "File" {
			foundFileGroup = true
		}
		if it.Key == 'm' && it.Label == "Move" {
			foundMove = true
		}
		if it.Key == 'M' && it.Label == "Create directory" {
			foundMkdir = true
		}
	}
	if !foundFileGroup {
		t.Fatal("expected File group header")
	}
	if !foundMove {
		t.Fatalf("items = %+v, want move with key m", app.model.LeaderMenu.Items)
	}
	if !foundMkdir {
		t.Fatalf("items = %+v, want mkdir with key M", app.model.LeaderMenu.Items)
	}
}

func TestBuiltinLeaderMenuQuitKeyExits(t *testing.T) {
	app := testLeaderMenuApp(t)

	app.openBuiltinLeaderMenu()
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
	if !quit {
		t.Fatal("handleKey() quit = false, want true for q from leader menu")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after q")
	}
}

func TestBuiltinLeaderMenuLetterDispatchesAction(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after activation")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindMove {
		t.Fatal("expected move dialog after m key")
	}
}

func TestBuiltinLeaderMenuMkdirKey(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'M', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after activation")
	}
	if !app.model.FileDialog.Open {
		t.Fatal("expected mkdir dialog after M key")
	}
	if app.model.FileDialog.MkdirOpenInInactive {
		t.Fatal("MkdirOpenInInactive = true, want false for M leader key")
	}
}

func TestBuiltinLeaderMenuMkdirOpenInOtherKey(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after activation")
	}
	if !app.model.FileDialog.Open {
		t.Fatal("expected mkdir dialog after o key")
	}
	if !app.model.FileDialog.MkdirOpenInInactive {
		t.Fatal("MkdirOpenInInactive = false, want true for o leader key")
	}
}

func TestBuiltinLeaderMenuCasePairFindVsFindDuplicates(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after f")
	}
	if !app.model.FindDialog.Open {
		t.Fatal("expected find dialog after lowercase f")
	}
	app.model.FindDialog = dialog.FindDialogState{}

	app.openBuiltinLeaderMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after F")
	}
	waitDedupDone(t, app)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("view = %v, want dedup after uppercase F", app.model.ViewMode)
	}
}

func TestBuiltinLeaderMenuQuestionMarkOpensHelp(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after ?")
	}
	if !app.model.HelpView.Open {
		t.Fatal("expected help view after ?")
	}
}

func TestBuiltinLeaderMenuHistoryKey(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after h")
	}
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog after h")
	}
}

func TestBuiltinLeaderMenuExtractKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "archive.zip"))
	app := testUserMenuApp(t, dir, dir)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after x")
	}
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogExtract {
		t.Fatalf("dialog = open %v type %v, want extract dialog", app.model.FileDialog.Open, app.model.FileDialog.DialogType)
	}
}

func TestBuiltinLeaderMenuF9Noop(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))

	if !app.model.LeaderMenu.Open {
		t.Fatal("F9 should not close built-in leader menu")
	}
}

func TestBuiltinLeaderMenuHasCasePairKeys(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	var findLower, findUpper, diskUsageKey bool
	for _, it := range app.model.LeaderMenu.Items {
		switch {
		case it.Key == 'f' && it.Label == "Find files":
			findLower = true
		case it.Key == 'F' && it.Label == "Find duplicates":
			findUpper = true
		case it.Key == 'D' && it.Label == "Disk usage scan":
			diskUsageKey = true
		}
	}
	if !findLower || !findUpper || !diskUsageKey {
		t.Fatalf("items missing case pairs: f=%v F=%v D=%v", findLower, findUpper, diskUsageKey)
	}
}

func TestBuiltinLeaderMenuJobsKeyOpensJobsView(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("view after o = %v, want jobs", app.model.ViewMode)
	}
}

func TestBuiltinLeaderMenuMessagesKeyOpensMessagesView(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()
	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'L', tcell.ModNone))
	if app.model.ViewMode != ui.ViewMessages {
		t.Fatalf("view after p = %v, want messages", app.model.ViewMode)
	}
}

func TestBuiltinLeaderMenuViewGroup(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	foundViewGroup := false
	var hiddenKey, metaKey bool
	for _, it := range app.model.LeaderMenu.Items {
		if it.GroupTitle == "View" {
			foundViewGroup = true
		}
		if it.Key == '.' && it.Label == "Toggle hidden files" {
			hiddenKey = true
		}
		if it.Key == ',' && it.Label == "Meta column" {
			metaKey = true
		}
	}
	if !foundViewGroup {
		t.Fatal("expected View group header")
	}
	if !hiddenKey || !metaKey {
		t.Fatalf("items missing view keys: hidden=%v meta=%v", hiddenKey, metaKey)
	}
}

func TestBuiltinLeaderMenuPeriodTogglesHidden(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.model.Primary.ShowHidden = true
	app.model.Secondary.ShowHidden = true
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, '.', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after .")
	}
	if app.model.Primary.ShowHidden || app.model.Secondary.ShowHidden {
		t.Fatalf("ShowHidden = (%v, %v), want both false after toggle", app.model.Primary.ShowHidden, app.model.Secondary.ShowHidden)
	}
}

func TestBuiltinLeaderMenuCommaOpensMetaDialog(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, ',', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after ,")
	}
	if !app.model.MetaDialog.Open {
		t.Fatal("expected meta dialog after , key")
	}
}

func TestBuiltinLeaderMenuGOpensSelectGroup(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))

	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after g")
	}
	if !app.model.GroupSelect.Open || app.model.GroupSelect.Mode != "select" {
		t.Fatalf("group select = %+v, want open select mode", app.model.GroupSelect)
	}
}

func TestActiveFooterKeysBuiltinLeaderMenuShowsF3ToggleChords(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	keys := app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("footer len = %d, want Esc + F3 + F10", len(keys))
	}
	if keys[1].Key != tcell.KeyF3 || keys[1].Hint != menu.FunctionKeyLeaderMenuToggleChords.Hint {
		t.Fatalf("second footer key = %+v, want F3 Toggle chords", keys[1])
	}
}

func TestBuiltinLeaderMenuF3TogglesDirectKeysAndPersists(t *testing.T) {
	dir := t.TempDir()
	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-leader-chords")}.WithResolvedLocations()
	screen := newScreen(t, 80, 50)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  appPaths,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	if !leaderMenuItemDirectKey(app, 'f', "Find files", "C-f") {
		t.Fatal("expected Find files DirectKey C-f before toggle")
	}

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if app.config.UI.LeaderMenuShowDirectKeys {
		t.Fatal("LeaderMenuShowDirectKeys = true, want false after F3")
	}
	if !app.model.LeaderMenu.Open {
		t.Fatal("leader menu should stay open after F3")
	}
	for _, it := range app.model.LeaderMenu.Items {
		if it.GroupTitle == "" && it.DirectKey != "" {
			t.Fatalf("DirectKey = %q for %q, want empty after toggle off", it.DirectKey, it.Label)
		}
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.UI.LeaderMenuShowDirectKeys {
		t.Fatal("persisted leader_menu_show_direct_keys = true, want false")
	}

	app.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if !app.config.UI.LeaderMenuShowDirectKeys {
		t.Fatal("LeaderMenuShowDirectKeys = false, want true after second F3")
	}
	if !leaderMenuItemDirectKey(app, 'f', "Find files", "C-f") {
		t.Fatal("expected Find files DirectKey C-f after toggle back on")
	}
}

func leaderMenuItemDirectKey(app *App, key rune, label, want string) bool {
	for _, it := range app.model.LeaderMenu.Items {
		if it.Key == key && it.Label == label {
			return it.DirectKey == want
		}
	}
	return false
}

func TestBuiltinLeaderMenuSetsDirectKey(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	var findDirectKey string
	for _, it := range app.model.LeaderMenu.Items {
		if it.Key == 'f' && it.Label == "Find files" {
			findDirectKey = it.DirectKey
			break
		}
	}
	if findDirectKey != "C-f" {
		t.Fatalf("Find files DirectKey = %q, want C-f", findDirectKey)
	}
}

func TestBuiltinLeaderMenuHiddenEntryStillActivatesByKey(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 16)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  config.Paths{ConfigDir: filepath.Join(dir, "config")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()
	if !app.model.LeaderMenu.Open {
		t.Fatal("expected leader menu open")
	}
	w, h := screen.Size()
	layout := app.layoutForTerminalSize(w, h)
	if hidden := ui.LeaderMenuHiddenActionCount(layout, app.model.LeaderMenu.Items); hidden == 0 {
		t.Fatalf("test setup: hidden = 0 at %dx%d, want >0", w, h)
	}
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
	if !quit {
		t.Fatal("handleKey() quit = false, want true for hidden q (quit)")
	}
	if app.model.LeaderMenu.Open {
		t.Fatal("leader menu should close after q")
	}
}

func TestBuiltinLeaderMenuOmitsDirectKeysWhenHeightConstrained(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 16)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  config.Paths{ConfigDir: filepath.Join(dir, "config")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()
	if !app.model.LeaderMenu.Open {
		t.Fatal("expected leader menu open")
	}
	w, h := app.screen.Size()
	layout := app.layoutForTerminalSize(w, h)
	if hidden := ui.LeaderMenuHiddenActionCount(layout, app.model.LeaderMenu.Items); hidden == 0 {
		t.Fatalf("test setup: hidden = 0 at %dx%d, want >0", w, h)
	}
	app.render()
	if leaderMenuScreenHasDirectKeySuffix(t, screen) {
		t.Fatal("expected direct key suffixes omitted when entries are hidden")
	}
}

func TestBuiltinLeaderMenuOmitsDirectKeysWhenWidthConstrained(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 40, 50)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  config.Paths{ConfigDir: filepath.Join(dir, "config")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()
	app.render()
	if leaderMenuScreenHasDirectKeySuffix(t, screen) {
		t.Fatal("expected direct key suffixes omitted at minimum terminal width")
	}
}

func leaderMenuScreenHasDirectKeySuffix(t *testing.T, screen tcell.SimulationScreen) bool {
	t.Helper()
	wantArrowFG, _, _ := theme.Default().LeaderMenuArrow.Decompose()
	w, h := screen.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mainc, style, _ := screen.Get(x, y)
			if mainc == "" || mainc == " " {
				continue
			}
			fg, _, _ := style.Decompose()
			if fg != wantArrowFG {
				continue
			}
			if mainc == "C" || mainc == "F" || mainc == "M" || mainc == "S" {
				return true
			}
		}
	}
	return false
}

func TestBuiltinLeaderMenuDirectKeyDisabledWhenConfigOff(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.UI.LeaderMenuShowDirectKeys = false
	screen := newScreen(t, 80, 50)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Paths:  config.Paths{ConfigDir: filepath.Join(dir, "config")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.model.Primary.Load(dir); err != nil {
		t.Fatal(err)
	}
	app.model.ViewMode = ui.ViewBrowser
	app.openBuiltinLeaderMenu()

	for _, it := range app.model.LeaderMenu.Items {
		if it.GroupTitle != "" {
			continue
		}
		if it.DirectKey != "" {
			t.Fatalf("DirectKey = %q for %q, want empty when config disabled", it.DirectKey, it.Label)
		}
	}
}

func TestColonKeyOpensBuiltinLeaderMenu(t *testing.T) {
	app := testLeaderMenuApp(t)
	id, ok := app.keys.Global.Lookup(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !ok || id != keymap.ActionAppLeaderMenu {
		t.Fatalf("lookup = %q %v, want %q", id, ok, keymap.ActionAppLeaderMenu)
	}
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionAppLeaderMenu)
	if !app.builtinLeaderMenuOpen() {
		t.Fatal("expected builtin leader menu open after :")
	}
}

func TestColonKeyTogglesBuiltinLeaderMenuClosed(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.openBuiltinLeaderMenu()
	if !app.builtinLeaderMenuOpen() {
		t.Fatal("expected menu open")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if app.model.LeaderMenu.Open {
		t.Fatal("expected menu closed after second :")
	}
}

func TestCtrlSOpensSortDialog(t *testing.T) {
	app := testLeaderMenuApp(t)
	app.model.ViewMode = ui.ViewBrowser

	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone))

	if !app.model.SortDialog.Open {
		t.Fatal("expected sort dialog after Ctrl+S")
	}
}
