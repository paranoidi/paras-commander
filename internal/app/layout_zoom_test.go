package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestRuntimeZoomToggleChangesLayoutAndDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = false
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "runtime-zoom-persist")}.WithResolvedLocations()
	if err := os.MkdirAll(appPaths.ConfigDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	initToml := `[ui.zoom]
active_panel = false
active_percent = 70
inactive_percent = 30
`
	if err := os.WriteFile(appPaths.ConfigFile, []byte(initToml), 0o644); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}

	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})

	layBefore := app.layoutForTerminalSize(100, 30)
	if layBefore.Primary.Width != 50 || layBefore.Secondary.Width != 50 {
		t.Fatalf("before toggle Left=%d Right=%d want 50/50", layBefore.Primary.Width, layBefore.Secondary.Width)
	}

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride == nil || !*app.zoomActivePanelOverride {
		t.Fatal("expected runtime override zoom on")
	}
	if app.config.UI.Zoom.ActivePanel {
		t.Fatal("saved ZoomActivePanel must stay false")
	}

	layAfter := app.layoutForTerminalSize(100, 30)
	if layAfter.Primary.Width != 70 || layAfter.Secondary.Width != 30 {
		t.Fatalf("after toggle Left=%d Right=%d want 70/30", layAfter.Primary.Width, layAfter.Secondary.Width)
	}

	app.openConfigDialog()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt)); quit {
		t.Fatal("handleKey quit")
	}
	if app.zoomActivePanelOverride != nil {
		t.Fatal("override should clear after Configuration OK")
	}

	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if reloaded.UI.Zoom.ActivePanel {
		t.Fatalf("persisted zoom_active_panel leaked true, want false")
	}
}

func TestLayoutForTerminalSizeIgnoresZoomInAuxiliaryViews(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = true
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})

	layBrowser := app.layoutForTerminalSize(100, 30)
	if layBrowser.Primary.Width != 70 || layBrowser.Secondary.Width != 30 {
		t.Fatalf("browser Left=%d Right=%d want 70/30", layBrowser.Primary.Width, layBrowser.Secondary.Width)
	}

	for _, vm := range []ui.ViewMode{ui.ViewJobs, ui.ViewCommands, ui.ViewMessages, ui.ViewFilePreview} {
		app.model.ViewMode = vm
		lay := app.layoutForTerminalSize(100, 30)
		if lay.Primary.Width != 50 || lay.Secondary.Width != 50 {
			t.Fatalf("view %v with zoom on: Left=%d Right=%d want 50/50", vm, lay.Primary.Width, lay.Secondary.Width)
		}
	}
}

func TestLayoutForTerminalSizeIgnoresHideInactivePanelInAuxiliaryViews(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	app := newTestApp(t, screen, testOptions(t.TempDir()))

	app.model.HideInactivePanel = true
	app.model.ActivePanel = ui.PrimaryPanel

	layBrowser := app.layoutForTerminalSize(100, 30)
	if layBrowser.Primary.Width != 100 || layBrowser.Secondary.Width != 0 {
		t.Fatalf("browser with hide: Left=%d Right=%d want 100/0", layBrowser.Primary.Width, layBrowser.Secondary.Width)
	}

	for _, vm := range []ui.ViewMode{ui.ViewJobs, ui.ViewCommands, ui.ViewMessages} {
		app.model.ViewMode = vm
		lay := app.layoutForTerminalSize(100, 30)
		if lay.Primary.Width != 50 || lay.Secondary.Width != 50 {
			t.Fatalf("view %v with hide inactive: Left=%d Right=%d want 50/50", vm, lay.Primary.Width, lay.Secondary.Width)
		}
	}
	if !app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel cleared when switching auxiliary views")
	}
}

func TestLayoutForTerminalSizeDisablesZoomWhileFilePreviewOpen(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = true
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})

	layZoomed := app.layoutForTerminalSize(100, 30)
	if layZoomed.Primary.Width != 70 || layZoomed.Secondary.Width != 30 {
		t.Fatalf("without preview Left=%d Right=%d want 70/30", layZoomed.Primary.Width, layZoomed.Secondary.Width)
	}

	app.commandsMu.Lock()
	app.model.FilePreview.Open = true
	app.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	app.model.FilePreview.CombinedText = "hello"
	app.commandsMu.Unlock()

	layEven := app.layoutForTerminalSize(100, 30)
	if layEven.Primary.Width != 50 || layEven.Secondary.Width != 50 {
		t.Fatalf("with preview Left=%d Right=%d want 50/50", layEven.Primary.Width, layEven.Secondary.Width)
	}

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{}
	app.commandsMu.Unlock()
	app.model.QuickViewEnabled = true
	layQV := app.layoutForTerminalSize(100, 30)
	if layQV.Primary.Width != 50 || layQV.Secondary.Width != 50 {
		t.Fatalf("with quick view armed Left=%d Right=%d want 50/50", layQV.Primary.Width, layQV.Secondary.Width)
	}
}

// newNameWidthZoomApp returns an app with default zoom config, icons explicitly on and a
// terminal of the given width, so the name-column arithmetic is deterministic: with the default
// brief listing and no meta columns the even-split name column is width/2 - 11 cells.
func newNameWidthZoomApp(t *testing.T, width int) *App {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(width, 30)

	cfg := config.Default()
	cfg.UI.UseNerdfontIcons = true
	cfg.UI.Zoom.ActivePanel = true
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	app.model.UseNerdfontIcons = true
	return app
}

// wideMetaColumn is a meta column whose rendered cells are 15 cells wide (plus a two-cell gap).
func wideMetaColumn() []ui.MetaColumnState {
	return []ui.MetaColumnState{{
		EntryName:   "bramble",
		ColumnTitle: "Bramble",
		Results:     map[string]string{"/lantern/a": strings.Repeat("x", 15)},
	}}
}

func TestLayoutForTerminalSizeDisablesZoomAtOrAboveDisabledAboveNameWidth(t *testing.T) {
	app := newNameWidthZoomApp(t, 160)

	lay := app.layoutForTerminalSize(160, 30)
	if lay.Primary.Width != 80 || lay.Secondary.Width != 80 {
		t.Fatalf("wide terminal Left=%d Right=%d want 50/50", lay.Primary.Width, lay.Secondary.Width)
	}

	// With the default listing, icons on and no meta columns the default threshold falls at a
	// 142-cell terminal: 142 is even, 141 zooms.
	layBoundary := app.layoutForTerminalSize(142, 30)
	if layBoundary.Primary.Width != 71 || layBoundary.Secondary.Width != 71 {
		t.Fatalf("width 142 Left=%d Right=%d want 50/50", layBoundary.Primary.Width, layBoundary.Secondary.Width)
	}
	layBelow := app.layoutForTerminalSize(141, 30)
	if layBelow.Primary.Width <= layBelow.Secondary.Width || layBelow.Primary.Width+layBelow.Secondary.Width != 141 {
		t.Fatalf("width 141 Left=%d Right=%d want zoomed split", layBelow.Primary.Width, layBelow.Secondary.Width)
	}
}

func TestLayoutForTerminalSizeZoomsWhenMetaColumnsSqueezeNameBelowThreshold(t *testing.T) {
	app := newNameWidthZoomApp(t, 160)
	if lay := app.layoutForTerminalSize(160, 30); lay.Primary.Width != 80 {
		t.Fatalf("without meta Left=%d want 80", lay.Primary.Width)
	}

	app.model.MetaResults[ui.PrimaryPanel] = wideMetaColumn()
	lay := app.layoutForTerminalSize(160, 30)
	if lay.Primary.Width != 112 || lay.Secondary.Width != 48 {
		t.Fatalf("with meta Left=%d Right=%d want 70/30 of 160", lay.Primary.Width, lay.Secondary.Width)
	}

	// Meta columns on the inactive panel do not count: the active panel's name column decides.
	app.model.MetaResults[ui.PrimaryPanel] = nil
	app.model.MetaResults[ui.SecondaryPanel] = wideMetaColumn()
	if lay := app.layoutForTerminalSize(160, 30); lay.Primary.Width != 80 {
		t.Fatalf("meta on inactive panel Left=%d want 80", lay.Primary.Width)
	}
}

func TestLayoutForTerminalSizeZoomDecisionIsStable(t *testing.T) {
	app := newNameWidthZoomApp(t, 143)
	app.model.MetaResults[ui.PrimaryPanel] = wideMetaColumn()

	first := app.layoutForTerminalSize(143, 30)
	second := app.layoutForTerminalSize(143, 30)
	if first.Primary != second.Primary || first.Secondary != second.Secondary {
		t.Fatalf("layout changed between calls: %+v vs %+v", first, second)
	}

	app.model.MetaResults[ui.PrimaryPanel] = nil
	for _, width := range []int{141, 142, 143} {
		var want bool
		for i, active := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
			app.model.ActivePanel = active
			got := app.effectiveZoomActivePanelLayout(width, 30, false)
			if i == 0 {
				want = got
			} else if got != want {
				t.Fatalf("width %d: switching the active panel flipped zoom %v -> %v", width, want, got)
			}
		}
	}
}

func TestRenderBrowserListNavUpdateFullRendersWhenZoomDecisionChanged(t *testing.T) {
	app := newNameWidthZoomApp(t, 160)
	app.render()
	if app.model.PanelZoomEnabled {
		t.Fatal("zoom should be off at 160 cells without meta columns")
	}

	app.model.MetaResults[ui.PrimaryPanel] = wideMetaColumn()
	if !app.zoomLayoutChanged(160, 30) {
		t.Fatal("zoomLayoutChanged = false after meta columns squeezed the name column")
	}
	app.renderBrowserListNavUpdate(ui.PrimaryPanel, false)
	if !app.model.PanelZoomEnabled {
		t.Fatal("PanelZoomEnabled not updated: partial paint ran over a stale split")
	}
	if app.zoomLayoutChanged(160, 30) {
		t.Fatal("zoomLayoutChanged = true after the full render")
	}
}

func TestLayoutForTerminalSizeZoomNotSuppressedWhenDisabledAboveNameWidthIsZero(t *testing.T) {
	app := newNameWidthZoomApp(t, 300)
	app.config.UI.Zoom.DisabledAboveNameWidth = 0

	lay := app.layoutForTerminalSize(300, 30)
	if lay.Primary.Width != 210 || lay.Secondary.Width != 90 {
		t.Fatalf("Left=%d Right=%d want 70/30 split", lay.Primary.Width, lay.Secondary.Width)
	}
}

func TestPanelToggleZoomNoOpWhenNameColumnWide(t *testing.T) {
	app := newNameWidthZoomApp(t, 160)

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride != nil {
		t.Fatalf("zoom override = %v, want nil (toggle ignored)", app.zoomActivePanelOverride)
	}
	if !strings.Contains(app.model.Message, "≥ 60") {
		t.Fatalf("transient message = %q, want threshold mention", app.model.Message)
	}
}

func TestPanelToggleZoomNoOpWhileFilePreviewOpen(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = false

	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})

	app.commandsMu.Lock()
	app.model.FilePreview.Open = true
	app.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	app.model.FilePreview.CombinedText = "x"
	app.commandsMu.Unlock()

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride != nil {
		t.Fatalf("zoom override = %v, want nil (toggle ignored)", app.zoomActivePanelOverride)
	}
	if !strings.Contains(app.model.Message, "Zoom disabled") {
		t.Fatalf("transient message = %q, want mention of zoom disabled", app.model.Message)
	}
}
