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

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

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
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
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

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

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

	app, err := New(screen, func() (string, error) {
		return t.TempDir(), nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

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

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

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

func TestLayoutForTerminalSizeDisablesZoomAtOrAboveDisabledAboveWidth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(160, 30)

	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = true
	cfg.UI.Zoom.DisabledAboveWidth = 155
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	lay := app.layoutForTerminalSize(160, 30)
	if lay.Primary.Width != 80 || lay.Secondary.Width != 80 {
		t.Fatalf("wide terminal Left=%d Right=%d want 50/50", lay.Primary.Width, lay.Secondary.Width)
	}

	layNarrow := app.layoutForTerminalSize(154, 30)
	if layNarrow.Primary.Width != 107 || layNarrow.Secondary.Width != 47 {
		t.Fatalf("below gate Left=%d Right=%d want 70%%/30%% of width", layNarrow.Primary.Width, layNarrow.Secondary.Width)
	}

	layBoundary := app.layoutForTerminalSize(155, 30)
	if layBoundary.Primary.Width != 77 || layBoundary.Secondary.Width != 78 {
		t.Fatalf("width == gate Left=%d Right=%d want ~50/50", layBoundary.Primary.Width, layBoundary.Secondary.Width)
	}
}

func TestLayoutForTerminalSizeZoomNotSuppressedWhenDisabledAboveWidthIsZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(300, 30)

	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = true
	cfg.UI.Zoom.DisabledAboveWidth = 0
	cfg.UI.Zoom.ActivePercent = 70
	cfg.UI.Zoom.InactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	lay := app.layoutForTerminalSize(300, 30)
	if lay.Primary.Width != 210 || lay.Secondary.Width != 90 {
		t.Fatalf("Left=%d Right=%d want 70/30 split", lay.Primary.Width, lay.Secondary.Width)
	}
}

func TestPanelToggleZoomNoOpOnWideTerminal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(160, 30)

	cfg := config.Default()
	cfg.UI.Zoom.DisabledAboveWidth = 155

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride != nil {
		t.Fatalf("zoom override = %v, want nil (toggle ignored)", app.zoomActivePanelOverride)
	}
	if !strings.Contains(app.model.Message, "≥ 155") {
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

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

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
