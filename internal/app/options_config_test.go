package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestNewWithOptionsAppliesConfiguredHiddenFilesToBothPanels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, "visible.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.Panels.ShowHidden = true
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if !app.model.Primary.ShowHidden || !app.model.Secondary.ShowHidden {
		t.Fatalf("ShowHidden left=%v right=%v, want both true", app.model.Primary.ShowHidden, app.model.Secondary.ShowHidden)
	}
	if len(app.model.Primary.Entries) != 2 || len(app.model.Secondary.Entries) != 2 {
		t.Fatalf("entry counts left=%d right=%d, want hidden and visible entries", len(app.model.Primary.Entries), len(app.model.Secondary.Entries))
	}
}

func TestNewWithOptionsAppliesProvidedTheme(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	styles := theme.Default()
	styles.Name = "custom"
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  styles,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if app.styles.Name != "custom" {
		t.Fatalf("app theme name = %q, want custom", app.styles.Name)
	}
}

func TestNewWithOptionsSetsUseNerdfontIconsFromConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.UI.UseNerdfontIcons = false
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.UseNerdfontIcons {
		t.Fatal("UseNerdfontIcons = true, want false from config")
	}
}

func TestNewWithOptionsAppliesDefaultListingFormatFromConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.Panels.DefaultListingFormat = config.ListingFormatBrief
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.Primary.ListFormat != panel.ListFormatBrief || app.model.Secondary.ListFormat != panel.ListFormatBrief {
		t.Fatalf("panels ListFormat = %v/%v, want brief", app.model.Primary.ListFormat, app.model.Secondary.ListFormat)
	}
}

func TestNewWithOptionsAppliesFilterCycleMatchesToPanels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.Filter.CycleMatches = config.FilterCycleMatchesRanked
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.Primary.Filter.CycleMatches != config.FilterCycleMatchesRanked {
		t.Fatalf("Left.Filter.CycleMatches = %q, want ranked", app.model.Primary.Filter.CycleMatches)
	}
	if app.model.Secondary.Filter.CycleMatches != config.FilterCycleMatchesRanked {
		t.Fatalf("Right.Filter.CycleMatches = %q, want ranked", app.model.Secondary.Filter.CycleMatches)
	}
}

func TestOptionsMenuOpensConfigurationDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.Panels.DefaultListingFormat = config.ListingFormatPerm
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(3) // File → Command → Display → Options
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.ConfigDialog.Open {
		t.Fatal("configuration dialog open = false, want true")
	}
	if !app.model.ConfigDialog.UseNerdfontIcons {
		t.Fatal("working copy UseNerdfontIcons = false, want default true")
	}
	if app.model.ConfigDialog.ListFormat != panel.ListFormatPerm {
		t.Fatalf("ConfigDialog.ListFormat = %v, want perm", app.model.ConfigDialog.ListFormat)
	}
}

func TestConfigDialogApplyPersistsUseNerdfontIcons(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-icons")}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.ConfigDialog.Open {
		t.Fatal("config dialog should close after apply")
	}
	if app.model.UseNerdfontIcons {
		t.Fatal("UseNerdfontIcons = true, want false after toggle")
	}
	if app.config.UI.UseNerdfontIcons {
		t.Fatal("config UI UseNerdfontIcons = true, want false")
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.UI.UseNerdfontIcons {
		t.Fatalf("persisted use_nerdfont_icons = true, want false")
	}
}

func TestConfigDialogApplyPersistsZoomActivePanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-zoom")}.WithResolvedLocations()
	cfg := config.Default()
	cfg.UI.Zoom.ActivePanel = false
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

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if !app.config.UI.Zoom.ActivePanel {
		t.Fatal("ZoomActivePanel = false, want true after toggle")
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if !reloaded.UI.Zoom.ActivePanel {
		t.Fatalf("persisted zoom_active_panel = false, want true")
	}
}

func TestConfigDialogApplyPersistsPaneSplitOrientation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-split")}.WithResolvedLocations()
	cfg := config.Default()
	cfg.UI.Zoom.Orientation = config.PaneSplitSideBySide
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

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.config.UI.Zoom.Orientation != config.PaneSplitStacked {
		t.Fatalf("PaneSplitOrientation = %q, want %q", app.config.UI.Zoom.Orientation, config.PaneSplitStacked)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.UI.Zoom.Orientation != config.PaneSplitStacked {
		t.Fatalf("persisted pane_split_orientation = %q, want %q", reloaded.UI.Zoom.Orientation, config.PaneSplitStacked)
	}
}

func TestConfigDialogApplyPersistsScrollMode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-scroll")}.WithResolvedLocations()
	cfg := config.Default()
	cfg.UI.Scroll.Mode = config.DefaultScrollMode
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
	if app.model.Primary.ScrollMode != panel.ScrollModeEdge {
		t.Fatalf("Left.ScrollMode = %q, want edge from config default", app.model.Primary.ScrollMode)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.config.UI.Scroll.Mode != config.ScrollModeCenter {
		t.Fatalf("ScrollMode = %q, want center after selection", app.config.UI.Scroll.Mode)
	}
	if app.model.Primary.ScrollMode != panel.ScrollModeCenter || app.model.Secondary.ScrollMode != panel.ScrollModeCenter {
		t.Fatal("panel ScrollMode not synced after apply")
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.UI.Scroll.Mode != config.ScrollModeCenter {
		t.Fatalf("persisted scroll_mode = %q, want center", reloaded.UI.Scroll.Mode)
	}
}

func TestConfigDialogApplyPersistsDefaultListingFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-lf")}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Primary.ListFormat != panel.ListFormatPerm || app.model.Secondary.ListFormat != panel.ListFormatPerm {
		t.Fatalf("panels ListFormat = %v/%v, want perm", app.model.Primary.ListFormat, app.model.Secondary.ListFormat)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.Panels.DefaultListingFormat != config.ListingFormatPerm {
		t.Fatalf("persisted default_listing_format = %q, want %q", reloaded.Panels.DefaultListingFormat, config.ListingFormatPerm)
	}
}

func TestOptionsMenuOpensThemeDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(3) // File → Command → Display → Options
	// Open pulldown for Options menu, then press shortcut.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = false, want true")
	}
	if app.model.ThemeDialog.Selected != 0 {
		t.Fatalf("theme dialog selected = %d, want current theme index 0", app.model.ThemeDialog.Selected)
	}
}
