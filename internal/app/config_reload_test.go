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
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// newReloadTestApp builds an app whose config directory is a fresh temp dir with no files.
func newReloadTestApp(t *testing.T) (*App, config.Paths) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lantern.txt"))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)
	paths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "reload-cfg")}.WithResolvedLocations()
	if err := os.MkdirAll(paths.ThemesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: config.Default(),
		Paths:  paths,
		Theme:  theme.Default(),
	})
	return app, paths
}

func writeReloadFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeReloadTheme writes the default theme under a new name into the themes dir.
func writeReloadTheme(t *testing.T, paths config.Paths, name string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(raw), `name = "default"`, `name = "`+name+`"`, 1)
	writeReloadFile(t, filepath.Join(paths.ThemesDir, name+".toml"), body)
}

func TestReloadConfigurationAppliesConfigKeysAndTheme(t *testing.T) {
	app, paths := newReloadTestApp(t)
	writeReloadTheme(t, paths, "harbor")
	writeReloadFile(t, paths.ConfigFile, "theme = \"harbor\"\n[ui]\nshow_menu_bar = false\n")
	writeReloadFile(t, filepath.Join(paths.ConfigDir, config.KeybindingsFileName()), "[main]\n\""+keymap.ActionUIOpenConfig+"\" = [\"M-F12\"]\n")
	global := app.keys.Global

	app.reloadConfiguration()

	if app.model.MessageUrgency != ui.MessageUrgencyInfo || app.model.Message != "Configuration reloaded" {
		t.Fatalf("message = %q (%v), want info Configuration reloaded", app.model.Message, app.model.MessageUrgency)
	}
	if app.config.UI.ShowMenuBar || !app.model.HideMenuBar {
		t.Fatalf("show_menu_bar not applied: config=%v HideMenuBar=%v", app.config.UI.ShowMenuBar, app.model.HideMenuBar)
	}
	if app.keys.Global != global {
		t.Fatal("global keymap pointer replaced; handlers holding it would miss the reload")
	}
	ev, ok := keymap.EventForKey("M-F12")
	if !ok {
		t.Fatal("EventForKey(M-F12) failed")
	}
	if id, ok := global.Lookup(ev); !ok || id != keymap.ActionUIOpenConfig {
		t.Fatalf("M-F12 resolves to %q, %v; want %s", id, ok, keymap.ActionUIOpenConfig)
	}
	if app.styles.Name != "harbor" || app.model.ThemeDialog.CurrentName != "harbor" {
		t.Fatalf("theme = %q (dialog %q), want harbor", app.styles.Name, app.model.ThemeDialog.CurrentName)
	}
	if _, ok := app.themes["harbor"]; !ok {
		t.Fatal("reloaded theme missing from theme registry")
	}
}

func TestReloadConfigurationRejectsInvalidFilesWithoutApplying(t *testing.T) {
	cases := []struct {
		name   string
		config string
		keys   string
	}{
		{name: "unknown config field", config: "[ui]\nshow_menu_bar = false\nbogus_field = 1\n"},
		{name: "bad chord", config: "[ui]\nshow_menu_bar = false\n", keys: "[main]\n\"" + keymap.ActionUIOpenConfig + "\" = [\"NotAKey\"]\n"},
		{name: "unknown theme", config: "theme = \"missing-meadow\"\n[ui]\nshow_menu_bar = false\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, paths := newReloadTestApp(t)
			writeReloadFile(t, paths.ConfigFile, tc.config)
			if tc.keys != "" {
				writeReloadFile(t, filepath.Join(paths.ConfigDir, config.KeybindingsFileName()), tc.keys)
			}
			before := app.config
			stylesName := app.styles.Name

			app.reloadConfiguration()

			if app.model.MessageUrgency != ui.MessageUrgencyError {
				t.Fatalf("message = %q (%v), want an error toast", app.model.Message, app.model.MessageUrgency)
			}
			if !app.config.UI.ShowMenuBar || app.model.HideMenuBar || app.config.Theme != before.Theme {
				t.Fatal("configuration applied despite a failed reload")
			}
			if app.styles.Name != stylesName {
				t.Fatalf("theme changed to %q despite a failed reload", app.styles.Name)
			}
		})
	}
}

func TestReloadConfigurationKeepsRestartOnlySettingsAndWarns(t *testing.T) {
	app, paths := newReloadTestApp(t)
	running := app.config.Jobs.ShowFinished
	writeReloadFile(t, paths.ConfigFile, "[jobs]\nshow_finished = "+boolTOML(!running)+"\n")

	app.reloadConfiguration()

	if app.model.MessageUrgency != ui.MessageUrgencyWarn || !strings.Contains(app.model.Message, "restart to apply: jobs") {
		t.Fatalf("message = %q (%v), want warning naming jobs", app.model.Message, app.model.MessageUrgency)
	}
	if app.config.Jobs.ShowFinished != running {
		t.Fatal("restart-only jobs setting applied; it must keep its running value")
	}
}

func TestOptionsMenuEndsWithReloadConfiguration(t *testing.T) {
	for _, def := range menu.BrowserDefinitions(nil, false) {
		if def.ID != menu.TopOptions {
			continue
		}
		last := def.Items[len(def.Items)-1]
		if last.Action != keymap.ActionUIReloadConfig || last.Label != "Reload configuration" || last.Shortcut != 'r' {
			t.Fatalf("last Options item = %+v, want Reload configuration (r)", last)
		}
		return
	}
	t.Fatal("Options pulldown not found")
}

func boolTOML(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
