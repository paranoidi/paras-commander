package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the Configuration dialog and its Yes/No sub-confirms, driven through
// App.handleKey with the harness of simple_dialog_keys_golden_test.go. The app has a temp config
// directory (nofile: no config.toml yet; withfile: the default stub exists) and a fake external
// editor that only counts its calls. Closing the dialog reports what it did to App.config.

var configKeyEditorCalls = map[*App]*int{}

func configFocusName(st dialog.ConfigDialogState) string {
	id := dlg.Focused(dialog.ConfigDialogView(st, 80), st.Focus)
	for _, p := range []struct{ prefix, name string }{{"scroll-mode-", "mode"}, {"scrollbar-", "bar"}, {"listing-", "listing"}, {"split-", "split"}} {
		if rest, ok := strings.CutPrefix(string(id), p.prefix); ok {
			return p.name + rest
		}
	}
	return string(id)
}

// configConfirmIndex is the old 0=Yes / 1=No encoding of a sub-confirm's focus.
func configConfirmIndex(f dlg.ID) int {
	if f == dialog.ConfigConfirmNo {
		return 1
	}
	return 0
}

func configKeyState(app *App) string {
	st := app.model.ConfigDialog
	switch {
	case st.EditStubConfirm:
		return fmt.Sprintf("edit-stub-confirm focus=%d", configConfirmIndex(st.EditStubConfirmFocus))
	case st.ResetDefaultsConfirm:
		return fmt.Sprintf("reset-confirm focus=%d", configConfirmIndex(st.ResetDefaultsConfirmFocus))
	}
	return fmt.Sprintf("focus=%s nerd=%v zoom=%v stacked=%v mouse=%v mode=%s bar=%s format=%d split=[%s|%s|%s] cursors=[%d|%d|%d] msg=%q",
		configFocusName(st), st.UseNerdfontIcons, st.ZoomActivePanel, st.PaneSplitStacked, st.Mouse, st.ScrollMode, st.PanelScrollbar, st.ListFormat,
		st.Split[0].Value, st.Split[1].Value, st.Split[2].Value, st.Split[0].Cursor, st.Split[1].Cursor, st.Split[2].Cursor, app.model.Message)
}

func configKeyOutcome(app *App, quit bool) string {
	c := app.config
	_, statErr := os.Stat(app.paths.ConfigFile)
	return fmt.Sprintf("quit=%v nerd=%v zoom=%v orientation=%s mouse=%v mode=%s bar=%s format=%s split=%v editor-calls=%d file-exists=%v message=%q",
		quit, c.UI.UseNerdfontIcons, c.UI.Zoom.ActivePanel, c.UI.Zoom.Orientation, c.UI.Mouse, c.UI.Scroll.Mode, c.UI.Scroll.Scrollbar,
		c.Panels.DefaultListingFormat, c.Carousel.Split, *configKeyEditorCalls[app], statErr == nil, app.model.Message)
}

func configKeyDialog(name string, withFile bool) keyGoldenDialog {
	return keyGoldenDialog{
		name:   name,
		serial: true, // setup swaps the package-level externalEditorRunner
		setup: func(t *testing.T, dir string) *App {
			cfgDir := filepath.Join(dir, "cfg")
			if err := os.MkdirAll(cfgDir, 0o755); err != nil {
				t.Fatal(err)
			}
			paths := config.Paths{ConfigDir: cfgDir}.WithResolvedLocations()
			if withFile {
				if err := config.WriteDefaultStub(paths.ConfigFile); err != nil {
					t.Fatal(err)
				}
			}
			cfg := config.Default()
			app := newTestApp(t, newScreen(t, 80, 30), Options{
				CWD:    func() (string, error) { return dir, nil },
				Config: cfg,
				Paths:  paths,
				Theme:  theme.Default(),
			})
			app.config.Shell.Persistent = false
			app.model.Message = ""
			calls := new(int)
			configKeyEditorCalls[app] = calls
			prev := externalEditorRunner
			externalEditorRunner = func(_ context.Context, _ string, _ bool) error { *calls++; return nil }
			t.Cleanup(func() { externalEditorRunner = prev; delete(configKeyEditorCalls, app) })
			app.openConfigDialog()
			return app
		},
		state:   configKeyState,
		open:    func(app *App) bool { return app.model.ConfigDialog.Open },
		outcome: configKeyOutcome,
	}
}

func TestConfigDialogKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"space-checkboxes", "Space Down Space Down Space"},
		{"enter-checkbox", "Enter Down Enter"},
		{"space-scroll-radios", "Down Down Down Space Right Space Down Space Left Down Space"},
		{"enter-radio", "Down Down Down Down Enter Right Enter"},
		{"space-listing", "Down Down Down Down Down Down Right Space Right Space Down Space"},
		{"enter-ok", "Down Down Down Down Down Down Down Down Down Down Enter"},
		{"space-ok", "Space Down Space Tab Tab Tab Tab Tab Tab Tab Space"},
		{"enter-cancel", "Space Down Down Down Down Down Down Down Down Down Down Down Right Enter"},
		{"space-cancel", "Space Backtab Right Space"},
		{"alt-d", "Down Down Down Alt+d"},
		{"alt-z", "Alt+z"},
		{"alt-s", "Alt+s Alt+s Alt+s Alt+o"},
		{"alt-h", "Down Down Down Down Alt+h"},
		{"alt-i", "Alt+i"},
		{"alt-e", "Alt+e Alt+t"},
		{"alt-t-n-u-r", "Alt+t Alt+n Alt+u Alt+r"},
		{"alt-m-p-b", "Alt+p Alt+b Alt+m"},
		{"alt-upper", "Alt+D Alt+Z Alt+P"},
		{"alt-unknown", "Alt+x Alt+q Alt+y Down"},
		{"alt-from-input", "Down Down Down Down Down Down Down Down Down Alt+d Alt+p Alt+u"},
		{"alt-o-applies", "Alt+d Alt+z Alt+o"},
		{"alt-c-cancels", "Alt+d Alt+c"},
		{"esc", "Alt+d Esc"},
		{"alt-enter", "Alt+z Alt+Enter"},
		{"plain-o", "Alt+d o"},
		{"plain-c", "Alt+d c"},
		{"plain-letters", "d z h i e t n u r m p b x Down"},
		{"plain-o-c-in-input", "Down Down Down Down Down Down Down Down o c"},
		{"input-typing", "Down Down Down Down Down Down Down Down Down x y z Backspace"},
		{"input-typing-replaces-default", "Tab Tab Tab Tab a"},
		{"input-cursor", "Tab Tab Tab Tab Left Left Right Home End Delete Backspace"},
		{"input-second", "Tab Tab Tab Tab Tab 5 0 %"},
		{"input-third", "Tab Tab Tab Tab Tab Tab Backspace x"},
		{"input-invalid-ok", "Tab Tab Tab Tab x Alt+o"},
		{"input-invalid-enter", "Tab Tab Tab Tab Tab Tab Tab x Enter"},
		{"input-enter-applies", "Tab Tab Tab Tab Enter"},
		{"input-valid-edit-ok", "Tab Tab Tab Tab Tab Tab Backspace Backspace 3 0 % Alt+o"},
		{"input-ctrl-keys", "Tab Tab Tab Tab Ctrl+K a b Ctrl+W"},
		{"input-ctrl-u", "Tab Tab Tab Tab Ctrl+U a"},
		{"input-ctrl-y", "Tab Tab Tab Tab Ctrl+K Ctrl+Y"},
		{"input-home-end", "Tab Tab Tab Tab Home End"},
		{"input-restore-default", "Tab Tab Tab Tab Backspace Backspace Backspace Ctrl+L"},
		{"f8-nofile", "F8"},
		{"f9-stub-confirm", "F9"},
		{"all-options-then-ok", "Space Down Space Down Space Down Space Down Down Space Down Down Down Space Alt+o"},
	}
	runKeyGoldenScripts(t, configKeyDialog("config_nofile", false), scripts)
	fileScripts := []simpleKeyScript{
		{"f9-edits", "F9"},
		{"f9-then-state", "Alt+d F9 Down"},
		{"f8-reset-confirm", "F8"},
		{"f8-then-navigation", "F8 Down Tab Left Right"},
		{"f9-alt-o", "Alt+d F9 Alt+o"},
	}
	runKeyGoldenScripts(t, configKeyDialog("config_withfile", true), fileScripts)
}

func TestConfigDialogSubConfirmKeyScripts(t *testing.T) {
	confirm := func(opener string) []simpleKeyScript {
		return []simpleKeyScript{
			{"open", opener},
			{"left-right", opener + " Right Right Left Left Right"},
			{"up-down", opener + " Down Right Up Left Down"},
			{"tab", opener + " Tab Tab Backtab Backtab"},
			{"enter-yes", opener + " Enter"},
			{"enter-no", opener + " Right Enter"},
			{"space-yes", opener + " Space"},
			{"space-no", opener + " Right Space"},
			{"alt-y", opener + " Alt+y"},
			{"alt-n", opener + " Alt+n"},
			{"alt-upper-y", opener + " Alt+Y"},
			{"alt-upper-n", opener + " Alt+N"},
			{"alt-o-yes", opener + " Alt+o"},
			{"alt-c-no", opener + " Alt+c"},
			{"alt-enter-yes", opener + " Right Alt+Enter"},
			{"esc-no", opener + " Esc"},
			{"f9-inside", opener + " F9"},
			{"f8-inside", opener + " F8"},
			{"plain-letters", opener + " y n o c x"},
			{"alt-unknown", opener + " Alt+x Alt+d Alt+z Right"},
			{"enter-after-no-focus-change", opener + " Right Left Enter"},
			{"typing-ignored", opener + " a b Backspace Delete Home End"},
		}
	}
	// The edit-config confirm needs no config.toml; the reset confirm needs one.
	runKeyGoldenScripts(t, configKeyDialog("config_editstub", false), confirm("F9"))
	runKeyGoldenScripts(t, configKeyDialog("config_reset", true), confirm("F8"))
	// A rejected edit-stub confirm leaves the config dialog usable.
	runKeyGoldenScripts(t, configKeyDialog("config_editstub_after", false), []simpleKeyScript{
		{"no-then-toggle", "F9 Right Enter Alt+d Down"},
		{"esc-then-esc", "F9 Esc Esc"},
	})
}
