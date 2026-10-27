package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the list dialogs (theme picker, SFTP connect, dedup empty-dirs confirm).
// They use the harness of simple_dialog_keys_golden_test.go: every script runs through
// App.handleKey and the transcript records the dialog state after each key.

var listGoldenWords = []string{
	"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper",
	"kestrel", "lantern", "meadow", "nectar", "orchard", "pebble", "quartz", "river", "saffron",
	"thistle", "umber", "velvet", "willow", "yarrow", "zephyr", "anchor", "breeze", "copper",
	"drift", "fern",
}

// ---- theme picker ----

// themeKeyApp builds an app whose theme picker offers n choices: the built-in default first, then
// n-1 fixture themes written to a temp themes dir (so previews resolve without the user's config).
func themeKeyApp(t *testing.T, dir string, n int) *App {
	t.Helper()
	themesDir := t.TempDir()
	def := theme.Default()
	choices := []theme.NamedTheme{{Name: def.Name, Label: "Default Theme", Theme: def}}
	for i := 1; i < n; i++ {
		w := listGoldenWords[i%len(listGoldenWords)]
		name := fmt.Sprintf("%s-%d", w, i)
		if err := os.WriteFile(filepath.Join(themesDir, name+".toml"), theme.TestThemeBytesNamed(t, name, nil, nil), 0o644); err != nil {
			t.Fatal(err)
		}
		choices = append(choices, theme.NamedTheme{Name: name, Label: strings.ToUpper(w[:1]) + w[1:] + " Theme", Theme: def})
	}
	opts := testOptions(dir)
	opts.Theme = def
	opts.ThemeChoices = choices
	opts.Paths = config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-theme"), ThemesDir: themesDir}.WithResolvedLocations()
	app := newTestApp(t, newScreen(t, 80, 24), opts)
	app.config.UI.KeyRepeatDebounceMS = 0
	app.config.Shell.Persistent = false
	app.model.Message = ""
	app.openThemeDialog()
	return app
}

func themeFocusName(app *App) string {
	return themeFocusNameOf(app)
}

func themeKeyDialog(name string, n int) keyGoldenDialog {
	return keyGoldenDialog{
		name:  name,
		setup: func(t *testing.T, dir string) *App { return themeKeyApp(t, dir, n) },
		state: func(app *App) string {
			st := app.model.ThemeDialog
			return fmt.Sprintf("focus=%s selected=%d style=%s", themeFocusName(app), st.Selected, app.styles.Name)
		},
		open: func(app *App) bool { return app.model.ThemeDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v config=%s style=%s current=%q message=%q", quit, app.config.Theme, app.styles.Name, app.model.ThemeDialog.CurrentName, app.model.Message)
		},
	}
}

func TestThemeDialogKeyScripts(t *testing.T) {
	long := themeKeyDialog("theme_dialog", 25)
	runKeyGoldenScripts(t, long, []simpleKeyScript{
		{"down-walk", "Down Down Down Down"},
		{"up-wrap", "Up Up Down"},
		{"down-wrap", "End Down Down Up"},
		{"up-walk", "Down Down Down Up Up Up Up"},
		{"pgdn-pgup", "PgDn PgDn PgUp PgUp PgUp"},
		{"pgup-from-top", "PgUp PgDn"},
		{"pgdn-from-bottom", "End PgDn PgDn"},
		{"home-end", "End Home End Up Home"},
		{"ctrl-home-end", "Down Down Ctrl+End Ctrl+Home"},
		{"scroll-down-through-list", strings.Repeat("Down ", 22)},
		{"scroll-up-through-list", "End " + strings.Repeat("Up ", 22)},
		{"tab-then-nav", "Down Down Tab Down Up Left Right Tab Left Right"},
		{"back-to-list", "Down Tab Up Tab Tab Up"},
		{"enter-list", "Down Down Enter"},
		{"enter-ok", "Down Tab Enter"},
		{"enter-cancel", "Down Down Tab Tab Enter"},
		{"enter-cancel-right", "Down Tab Right Enter"},
		{"space-cancel-right", "Down Tab Right Space"},
		{"esc-from-cancel", "Tab Right Esc"},
		{"esc", "Down Down Esc"},
		{"esc-from-button", "Down Tab Tab Esc"},
		{"space-list", "Down Space"},
		{"space-ok", "Down Tab Space"},
		{"space-cancel", "Down Tab Tab Space"},
		{"plain-o", "Down o"},
		{"plain-c", "Down c"},
		{"plain-other", "x q y Down"},
		{"alt-o", "Down Alt+o"},
		{"alt-c", "Down Alt+c"},
		{"alt-upper-o", "Down Alt+O"},
		{"alt-enter", "Down Alt+Enter"},
		{"alt-unknown", "Alt+x Down"},
		{"f5", "Down Down F5 Down"},
		{"backspace-delete", "Backspace Delete Down"},
	})
	short := themeKeyDialog("theme_dialog_short", 4)
	runKeyGoldenScripts(t, short, []simpleKeyScript{
		{"walk", "Down Down Down Down Up Up Up Up Up"},
		{"page-home-end", "PgDn PgUp End Home PgDn End PgUp"},
		{"tab-enter-ok", "Down Tab Enter"},
	})
	single := themeKeyDialog("theme_dialog_single", 1)
	runKeyGoldenScripts(t, single, []simpleKeyScript{
		{"nav", "Down Up PgDn PgUp End Home Tab Up Enter"},
	})
}

func themeFocusNameOf(app *App) string {
	switch app.model.ThemeDialog.Focus {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "list"
}

// ---- SFTP connect ----

func sftpFocusNameOf(app *App) string {
	switch app.model.SFTPConnectDialog.Focus {
	case dialog.SFTPLocation:
		return "location"
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "filter"
}

// sftpKeyApp opens the SFTP connect dialog over a fixture ssh config of n hosts (never the user's
// ~/.ssh). The connection attempt is stubbed so Enter on OK never touches the network.
func sftpKeyApp(t *testing.T, dir string, n int) *App {
	t.Helper()
	var cfg strings.Builder
	for i := 0; i < n; i++ {
		w := listGoldenWords[i%len(listGoldenWords)]
		fmt.Fprintf(&cfg, "Host %s\n  HostName %s.example.net\n  User user%d\n\n", w, w, i)
	}
	sshConfig := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(sshConfig, []byte(cfg.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.sftp.touchConn = func(context.Context, pathloc.Path) error { return fmt.Errorf("stubbed") }
	app.config.SFTP.SSHConfigFile = sshConfig
	app.model.Message = ""
	app.openSFTPConnectDialogForPanel(ui.PrimaryPanel)
	return app
}

func sftpKeyDialog(name string, n int) keyGoldenDialog {
	return keyGoldenDialog{
		name:  name, // the dialog only: the panels behind it show the host's free disk space
		setup: func(t *testing.T, dir string) *App { return sftpKeyApp(t, dir, n) },
		state: func(app *App) string {
			st := app.model.SFTPConnectDialog
			return fmt.Sprintf("focus=%s selected=%d scroll=%d query=%q qcursor=%d ranked=%d location=%q lcursor=%d pending=%v",
				sftpFocusNameOf(app), st.Selected, st.ListScroll, st.Query, st.QueryCursor, len(st.Ranked),
				st.Location.Value, st.Location.Cursor, st.Location.PrefillPending)
		},
		open: func(app *App) bool { return app.model.SFTPConnectDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v message=%q", quit, app.model.Message)
		},
	}
}

func TestSFTPConnectKeyScripts(t *testing.T) {
	long := sftpKeyDialog("sftp_connect", 20)
	runKeyGoldenScripts(t, long, []simpleKeyScript{
		{"down-walk", "Down Down Down"},
		{"up-at-top", "Up Up Down Up Up"},
		{"down-to-bottom", strings.Repeat("Down ", 22)},
		{"up-from-bottom", "Ctrl+End Up Up Up"},
		{"pgdn-pgup", "PgDn PgDn PgDn PgUp PgUp PgUp PgUp"},
		{"pgup-at-top", "PgUp Down PgUp"},
		{"pgdn-at-bottom", "Ctrl+End PgDn Down"},
		{"ctrl-home-end", "Down Down Ctrl+End Ctrl+Home Ctrl+End"},
		{"home-end-edit-query", "a b c Home End Left Left Home"},
		{"scroll-down-up", strings.Repeat("Down ", 12) + strings.Repeat("Up ", 12)},
		{"type-filter", "e m"},
		{"type-filter-resets-selection", "Down Down Down e"},
		{"type-filter-edit", "a r Left Backspace x Delete Home Delete"},
		{"type-filter-nomatch", "z z z q"},
		{"type-filter-space", "e Space m"},
		{"type-then-down", "e Down Down Up"},
		{"nomatch-down-up", "z z z q Down Up"},
		{"nomatch-tab", "z z z q Tab Tab Tab Tab Tab"},
		{"nomatch-backtab", "z z z q Backtab Backtab Backtab"},
		{"nomatch-enter", "z z z q Enter"},
		{"nomatch-then-clear", "z z z q Backspace Backspace Backspace Backspace"},
		{"left-right-filter", "a b Left Left Left Right Right Right Right"},
		{"tab-from-list-moved", "Down Down Tab Backtab"},
		{"down-up-buttons", "Tab Tab Down Up Tab Tab Down Up Up"},
		{"left-right-location", "Tab Left Right Left Left"},
		{"type-location", "Tab x y Left Backspace"},
		{"type-location-prefilled", "Tab Backspace"},
		{"enter-host", "Down Down Enter"},
		{"enter-host-then-enter", "Down Enter Enter"},
		{"enter-ok", "Tab Tab Enter"},
		{"enter-ok-after-host", "Down Enter Tab Enter"},
		{"enter-cancel", "Tab Tab Right Enter"},
		{"enter-location", "Tab Enter"},
		{"enter-location-typed", "Tab x Enter"},
		{"esc", "Down Esc"},
		{"esc-from-location", "Tab Esc"},
		{"esc-from-button", "Tab Tab Right Esc"},
		{"alt-o", "Down Alt+o"},
		{"alt-c", "Down Alt+c"},
		{"alt-upper", "Alt+O"},
		{"alt-enter", "Alt+Enter"},
		{"alt-unknown", "Alt+x Down"},
		{"plain-letters-on-buttons", "Tab Tab o c y n"},
		{"space-on-buttons", "Tab Tab Space"},
		{"space-on-cancel", "Tab Tab Right Space"},
		{"delete-backspace-empty", "Backspace Delete Down"},
	})
	short := sftpKeyDialog("sftp_connect_short", 3)
	runKeyGoldenScripts(t, short, []simpleKeyScript{
		{"walk", "Down Down Down Up Up Up PgDn PgUp Ctrl+End Ctrl+Home"},
		{"filter", "w i l"},
	})
	empty := sftpKeyDialog("sftp_connect_empty", 0)
	runKeyGoldenScripts(t, empty, []simpleKeyScript{
		{"typing", "x y z Up Tab Backspace"},
		{"enter", "Enter"},
		{"enter-ok", "Tab Enter"},
	})
}

// ---- dedup empty-dirs confirm ----

func dedupEmptyFocusName(app *App) string {
	switch app.model.DedupEmptyDirsConfirm.Focus {
	case dialog.DedupEmptyDirsNo:
		return "no"
	case dlg.Cancel:
		return "cancel"
	}
	return "yes"
}

// dedupEmptyFixture is the scan root and the script's *testing.T for the outcome's job waits.
type dedupEmptyFixture struct {
	sub string
	t   *testing.T
}

// dedupEmptyKeyDialog opens the confirmation through the real flow (a scan root holding only a
// duplicate pair, so deleting both leaves it empty) and reports whether the root survived.
func dedupEmptyKeyDialog(name string) keyGoldenDialog {
	var fixtures perApp[dedupEmptyFixture]
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, _ string) *App {
			sub := filepath.Join(t.TempDir(), "orchard")
			if err := os.Mkdir(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, n := range []string{"meadow.txt", "willow.txt"} {
				if err := os.WriteFile(filepath.Join(sub, n), []byte("dup"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			app := newApp(t, newScreen(t, 80, 24), sub)
			app.model.Message = ""
			fixtures.set(t, app, dedupEmptyFixture{sub: sub, t: t})
			app.openFindDuplicates()
			waitDedupDone(t, app)
			app.handleDedupViewKey(goldenKeyEvent(t, "Insert"))
			app.handleDedupViewKey(goldenKeyEvent(t, "Insert"))
			app.handleDedupViewKey(goldenKeyEvent(t, "F8"))
			app.dialogCtrl.ExecuteDelete()
			if !app.model.DedupEmptyDirsConfirm.Open {
				t.Fatal("empty-dirs confirm did not open")
			}
			app.model.Message = ""
			return app
		},
		state: func(app *App) string {
			return fmt.Sprintf("focus=%s dirs=%v", dedupEmptyFocusName(app), app.model.DedupEmptyDirsConfirm.Dirs)
		},
		open: func(app *App) bool { return app.model.DedupEmptyDirsConfirm.Open },
		outcome: func(app *App, quit bool) string {
			fx := fixtures.get(app)
			flushBackgroundJobs(fx.t, app)
			_, err := os.Stat(fx.sub)
			return fmt.Sprintf("quit=%v root_removed=%v message=%q", quit, os.IsNotExist(err), app.model.Message)
		},
	}
}

// dedupEmptyLongKeyDialog shows a list past the visible cap; closing it only reports the answer.
func dedupEmptyLongKeyDialog(name string, n int) keyGoldenDialog {
	return keyGoldenDialog{
		name: name,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 80, 24), dir)
			app.model.Message = ""
			dirs := make([]string, n)
			for i := range dirs {
				dirs[i] = listGoldenWords[i%len(listGoldenWords)] + "/" + listGoldenWords[(i*7+3)%len(listGoldenWords)]
			}
			app.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{Open: true, Dirs: dirs}
			return app
		},
		state: func(app *App) string {
			return fmt.Sprintf("focus=%s dirs=%d", dedupEmptyFocusName(app), len(app.model.DedupEmptyDirsConfirm.Dirs))
		},
		open:    func(app *App) bool { return app.model.DedupEmptyDirsConfirm.Open },
		outcome: func(app *App, quit bool) string { return fmt.Sprintf("quit=%v message=%q", quit, app.model.Message) },
	}
}

func TestDedupEmptyDirsConfirmKeyScripts(t *testing.T) {
	scripts := []simpleKeyScript{
		{"enter-yes", "Enter"},
		{"enter-no", "Right Enter"},
		{"enter-cancel", "Right Right Enter"},
		{"space", "Space"},
		{"space-no", "Right Space"},
		{"alt-y", "Right Alt+y"},
		{"alt-n", "Alt+n"},
		{"alt-upper-y", "Right Alt+Y"},
		{"alt-upper-n", "Alt+N"},
		{"alt-c", "Alt+c"},
		{"alt-o", "Right Alt+o"},
		{"alt-o-cancel", "Right Right Alt+o"},
		{"alt-enter", "Right Alt+Enter"},
		{"alt-unknown", "Alt+x Alt+q Right"},
		{"esc", "Right Esc"},
		{"plain-letters", "y n c o x q"},
	}
	runKeyGoldenScripts(t, dedupEmptyKeyDialog("dedup_empty_dirs"), scripts)
	long := []simpleKeyScript{
		{"enter-yes", "Enter"},
		{"alt-n", "Alt+n"},
	}
	runKeyGoldenScripts(t, dedupEmptyLongKeyDialog("dedup_empty_dirs_more", 14), long)
	runKeyGoldenScripts(t, dedupEmptyLongKeyDialog("dedup_empty_dirs_none", 0), long[:1])
}
