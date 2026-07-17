package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestThemeDialogAppliesThemeImmediately(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	secondTheme, themePaths := loadTestTheme(t)
	appPaths := config.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "persist-theme"),
		ThemesDir: themePaths.ThemesDir,
	}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  appPaths,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
			{Name: secondTheme.Name, Label: "Test Theme", Theme: secondTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = true, want closed")
	}
	if app.styles.Name != "test-theme" {
		t.Fatalf("theme name = %q, want test-theme", app.styles.Name)
	}
	if app.config.Theme != "test-theme" {
		t.Fatalf("config theme = %q, want test-theme", app.config.Theme)
	}
	if app.model.ThemeDialog.CurrentName != "test-theme" {
		t.Fatalf("theme dialog current name = %q, want test-theme", app.model.ThemeDialog.CurrentName)
	}
	if app.model.Message != "Theme changed to test-theme" {
		t.Fatalf("Message = %q, want theme changed message", app.model.Message)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.Theme != "test-theme" {
		t.Fatalf("persisted Theme = %q, want test-theme", reloaded.Theme)
	}
}

func TestThemeDialogNavigatePreviewsWithoutPersist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	secondTheme, themePaths := loadTestTheme(t)
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  themePaths,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
			{Name: secondTheme.Name, Label: "Test Theme", Theme: secondTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = false, want true")
	}
	if app.styles.Name != "test-theme" {
		t.Fatalf("preview theme name = %q, want test-theme", app.styles.Name)
	}
	if app.config.Theme != defaultTheme.Name {
		t.Fatalf("config theme = %q, want persisted default %q", app.config.Theme, defaultTheme.Name)
	}
	if app.model.ThemeDialog.CurrentName != defaultTheme.Name {
		t.Fatalf("ThemeDialog.CurrentName = %q, want %q (marker for saved theme)", app.model.ThemeDialog.CurrentName, defaultTheme.Name)
	}
}

func TestThemeDialogEscRevertsPreview(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	secondTheme, themePaths := loadTestTheme(t)
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  themePaths,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
			{Name: secondTheme.Name, Label: "Test Theme", Theme: secondTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	if app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = true, want closed")
	}
	if app.styles.Name != defaultTheme.Name {
		t.Fatalf("theme name after cancel = %q, want restored %q", app.styles.Name, defaultTheme.Name)
	}
	if app.config.Theme != defaultTheme.Name {
		t.Fatalf("config theme after cancel = %q, want %q", app.config.Theme, defaultTheme.Name)
	}
	if app.model.ThemeDialog.CurrentName != defaultTheme.Name {
		t.Fatalf("ThemeDialog.CurrentName = %q, want %q", app.model.ThemeDialog.CurrentName, defaultTheme.Name)
	}
}

func newFilePreviewThemePickerTestApp(t *testing.T) (*App, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.Preview.Style = config.DefaultPreviewStyle
	appPaths := config.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "persist-f3-style"),
	}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
		Paths:  appPaths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	app.model.ViewMode = ui.ViewFilePreview
	previewPath := filepath.Join(dir, "alpha.txt")
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Path = previewPath
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = "preview\n"
	})
	return app, appPaths
}

func TestFilePreviewThemePickerOpensOnF9(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))
	if !app.model.FilePreviewThemePicker.Open {
		t.Fatal("style picker open = false, want true after F9")
	}
}

func TestFilePreviewThemePickerNavigatePreviewsWithoutPersist(t *testing.T) {
	app, appPaths := newFilePreviewThemePickerTestApp(t)
	initial := app.config.Preview.Style
	uiTheme := app.config.Theme
	app.openFilePreviewThemePicker()
	app.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	// Style is debounced: config must not change until flush fires.
	if app.config.Preview.Style != initial {
		t.Fatalf("preview style changed to %q before debounce flush, want unchanged", app.config.Preview.Style)
	}
	// Picker selection must have advanced though.
	if app.filePreviewThemePickerSelectedName() == initial {
		t.Fatalf("picker selection still %q after Down, want different style", initial)
	}
	if app.config.Theme != uiTheme {
		t.Fatalf("UI theme changed to %q, want unchanged", app.config.Theme)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if reloaded.Preview.Style != initial {
		t.Fatalf("persisted preview.style = %q, want unchanged %q", reloaded.Preview.Style, initial)
	}
}

func TestFilePreviewThemePickerEnterClosePersists(t *testing.T) {
	app, appPaths := newFilePreviewThemePickerTestApp(t)
	app.openFilePreviewThemePicker()
	app.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	// Read selection from picker state before Enter flushes it.
	selected := app.filePreviewThemePickerSelectedName()
	if selected == "" {
		t.Fatal("no selection after Down")
	}
	app.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FilePreviewThemePicker.Open {
		t.Fatal("style picker still open after Enter save")
	}
	if app.config.Preview.Style != selected {
		t.Fatalf("config preview.style = %q, want %q", app.config.Preview.Style, selected)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if reloaded.Preview.Style != selected {
		t.Fatalf("persisted preview.style = %q, want %q", reloaded.Preview.Style, selected)
	}
}

func TestFilePreviewThemePickerEscReverts(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	initial := app.config.Preview.Style
	app.openFilePreviewThemePicker()
	app.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	// Simulate the debounce firing so the style actually changes before Esc.
	app.applyPreviewStylePickerFlush(previewStylePickerFlushPayload{gen: app.previewStylePickerDebounceGen.Load()})
	if app.config.Preview.Style == initial {
		t.Fatalf("preview style still %q after flush, want new selection", initial)
	}
	app.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FilePreviewThemePicker.Open {
		t.Fatal("style picker still open after Esc")
	}
	if app.config.Preview.Style != initial {
		t.Fatalf("preview.style after Esc = %q, want reverted %q", app.config.Preview.Style, initial)
	}
}

func TestActiveFooterKeysThemeDialogShowsF5Reload(t *testing.T) {
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

	app.openThemeDialog()
	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = false, want true")
	}
	keys := app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("theme dialog footer len = %d, want Esc + F5 + F10", len(keys))
	}
	if keys[0].Key != tcell.KeyEsc || keys[0].Hint != "Close" {
		t.Fatalf("first footer key = %+v, want Esc Close", keys[0])
	}
	if keys[1].Key != tcell.KeyF5 || keys[1].Hint != "Reload" {
		t.Fatalf("second footer key = %+v, want F5 Reload", keys[1])
	}
	if keys[2].Key != tcell.KeyF10 {
		t.Fatalf("third footer key = %+v, want F10", keys[2])
	}
}

func TestThemeDialogF5ReloadsCurrentPreviewFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	themesDir := t.TempDir()

	base, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatalf("read default theme fixture: %v", err)
	}
	writeDiskDefault := func(hex string) {
		content := strings.Replace(string(base),
			`bar.inactive = { fg = "bright_black" }`,
			fmt.Sprintf(`bar.inactive = { fg = "white", bg = %q }`, hex), 1)
		if err := os.WriteFile(filepath.Join(themesDir, "override.toml"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	writeDiskDefault("#111111")

	paths := config.Paths{ThemesDir: themesDir}.WithResolvedLocations()
	styles, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	choices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config:       config.Default(),
		Theme:        styles,
		ThemeChoices: choices,
		Paths:        paths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	_, bg1, _ := app.styles.MenuBarInactive.Decompose()
	if want := tcell.NewRGBColor(0x11, 0x11, 0x11); bg1 != want {
		t.Fatalf("initial preview bg = %v, want %v", bg1, want)
	}

	writeDiskDefault("#222222")
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(F5) quit = true, want false")
	}
	_, bg2, _ := app.styles.MenuBarInactive.Decompose()
	if want := tcell.NewRGBColor(0x22, 0x22, 0x22); bg2 != want {
		t.Fatalf("after F5 bg = %v, want updated disk theme %v", bg2, want)
	}
}

func TestThemeDialogF5ReloadsMenuDropdownAccentFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	themesDir := t.TempDir()

	base, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatalf("read default theme fixture: %v", err)
	}
	writeAccentFG := func(paletteName string) {
		content := strings.Replace(string(base),
			`dropdown.accent = { fg = "white"}`,
			fmt.Sprintf(`dropdown.accent = { fg = %q, bold = true }`, paletteName), 1)
		if err := os.WriteFile(filepath.Join(themesDir, "override.toml"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	writeAccentFG("bright_red")

	paths := config.Paths{ThemesDir: themesDir}.WithResolvedLocations()
	styles, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	choices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config:       config.Default(),
		Theme:        styles,
		ThemeChoices: choices,
		Paths:        paths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	wantFG1, _, _ := styles.MenuDropdownAccent.Decompose()
	fg1, _, _ := app.styles.MenuDropdownAccent.Decompose()
	if fg1 != wantFG1 {
		t.Fatalf("initial preview menu.dropdown.accent fg = %v, want %v", fg1, wantFG1)
	}

	writeAccentFG("bright_green")
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(F5) quit = true, want false")
	}
	reloaded, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve after F5: %v", err)
	}
	wantFG2, _, _ := reloaded.MenuDropdownAccent.Decompose()
	fg2, _, _ := app.styles.MenuDropdownAccent.Decompose()
	if fg2 != wantFG2 {
		t.Fatalf("after F5 menu.dropdown.accent fg = %v, want %v", fg2, wantFG2)
	}
}

func TestThemePreviewReloadErrorSetsCriticalStatusMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	themesDir := t.TempDir()

	base, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatalf("read default theme fixture: %v", err)
	}
	content := strings.Replace(string(base),
		`bar = { fg = "default" }`,
		`bar = { fg = "white", bg = "#111111" }`, 1)
	if err := os.WriteFile(filepath.Join(themesDir, "override.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	paths := config.Paths{ThemesDir: themesDir}.WithResolvedLocations()
	styles, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	choices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config:       config.Default(),
		Theme:        styles,
		ThemeChoices: choices,
		Paths:        paths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	if err := os.Chmod(themesDir, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(themesDir, 0700) })

	app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if app.model.MessageDialog.Open {
		t.Fatal("expected no message dialog after reload error (transient message instead)")
	}
	if strings.TrimSpace(app.model.Message) == "" {
		t.Fatal("expected non-empty transient message after reload error")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want MessageUrgencyCritical", app.model.MessageUrgency)
	}
	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog should remain open")
	}
}
