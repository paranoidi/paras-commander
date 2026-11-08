package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
	})

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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  styles,
	})

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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if app.model.UseNerdfontIcons {
		t.Fatal("UseNerdfontIcons = true, want false from config")
	}
}

func TestNewWithOptionsAppliesDefaultColumnProfileFromConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.Panels.DefaultColumnProfile = config.ColumnProfileBrief
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if app.model.Primary.Columns != panel.ColumnsBrief || app.model.Secondary.Columns != panel.ColumnsBrief {
		t.Fatalf("panel columns = %v/%v, want brief", app.model.Primary.Columns.Names(), app.model.Secondary.Columns.Names())
	}
	if app.model.Primary.ColumnProfile != config.ColumnProfileBrief {
		t.Fatalf("panel profile = %q", app.model.Primary.ColumnProfile)
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if !app.model.Primary.FilterOptions().CycleRanked {
		t.Fatal("Left FilterOptions().CycleRanked = false, want true")
	}
	if !app.model.Secondary.FilterOptions().CycleRanked {
		t.Fatal("Right FilterOptions().CycleRanked = false, want true")
	}
	app.config.Filter.CycleMatches = config.FilterCycleMatchesVisual
	if app.model.Primary.FilterOptions().CycleRanked {
		t.Fatal("hook must read live config")
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
	cfg.Panels.DefaultColumnProfile = config.ColumnProfilePermissions
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})

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
	if app.model.Primary.Columns != panel.ColumnsPermissions {
		t.Fatalf("panel columns = %v, want permissions", app.model.Primary.Columns.Names())
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt))
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt))
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt))
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if app.model.Primary.ScrollMode != panel.ScrollModeEdge {
		t.Fatalf("Left.ScrollMode = %q, want edge from config default", app.model.Primary.ScrollMode)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModAlt))
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
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
		},
	})

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

func TestConfigDialogApplyCarouselSplit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 30)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-split")}.WithResolvedLocations()
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	typeInto := func(idx int, text string) {
		st := &app.model.ConfigDialog
		st.Focus = dialog.ConfigSplitID(idx)
		st.Split[idx].Clear()
		for _, r := range text {
			app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		}
	}

	app.openConfigDialog()
	typeInto(2, "<5")
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))
	if !app.model.ConfigDialog.Open || app.model.ConfigDialog.Focus != dialog.ConfigSplitID(2) {
		t.Fatalf("invalid split: open=%v focus=%q, want open focus split-2", app.model.ConfigDialog.Open, app.model.ConfigDialog.Focus)
	}
	if got := app.config.Carousel.Split[2]; got != "*" {
		t.Fatalf("config split[2] = %q, want unchanged", got)
	}

	typeInto(2, "*")
	typeInto(1, "<30%")
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))
	if app.model.ConfigDialog.Open {
		t.Fatal("dialog should close after valid apply")
	}
	if got := app.config.Carousel.Split[1]; got != "<30%" {
		t.Fatalf("config split[1] = %q, want <30%%", got)
	}
	want, _ := panelcarousel.ParseLayout(app.config.Carousel.Split, app.config.Carousel.ShowSize)
	if app.model.CarouselLayout != want {
		t.Fatalf("CarouselLayout = %+v, want %+v", app.model.CarouselLayout, want)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if reloaded.Carousel.Split[1] != "<30%" {
		t.Fatalf("persisted split = %v", reloaded.Carousel.Split)
	}
}

func TestStartupProfileActivatesItsMetaColumnsAndSavedFile(t *testing.T) {
	dir := t.TempDir()
	cfgDir := t.TempDir()
	saved := []columnprofile.Profile{{Name: "orchard", Columns: []string{"size", "owner"}, Meta: []string{config.FileIndexTagsColumn}}}
	if err := columnprofile.Save(filepath.Join(cfgDir, config.DefaultColumnProfilesFileName), saved); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Panels.DefaultColumnProfile = "orchard"
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(120, 30)
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Paths:  config.Paths{ConfigDir: cfgDir},
		Theme:  theme.Default(),
	})
	want := panel.ColumnsBrief.With(panel.ColumnOwner)
	if app.model.Primary.Columns != want || app.model.Secondary.Columns != want || app.model.Primary.ColumnProfile != "orchard" {
		t.Fatalf("columns = %v/%v, profile = %q", app.model.Primary.Columns.Names(), app.model.Secondary.Columns.Names(), app.model.Primary.ColumnProfile)
	}
	if got := app.model.MetaResults[ui.PrimaryPanel]; len(got) != 1 || got[0].EntryName != config.FileIndexTagsColumn {
		t.Fatalf("meta columns = %+v", got)
	}
}

func TestUnknownStartupProfileWarnsAndFallsBack(t *testing.T) {
	cfg := config.Default()
	cfg.Panels.DefaultColumnProfile = "lantern"
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(120, 30)
	dir := t.TempDir()
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
		Theme:  theme.Default(),
	})
	if app.model.Primary.ColumnProfile != config.DefaultColumnProfile || app.model.Primary.Columns != panel.ColumnsBrief {
		t.Fatalf("profile = %q, columns = %v", app.model.Primary.ColumnProfile, app.model.Primary.Columns.Names())
	}
	if !strings.Contains(app.model.Message, "lantern") {
		t.Fatalf("message = %q, want the unknown name", app.model.Message)
	}
}
