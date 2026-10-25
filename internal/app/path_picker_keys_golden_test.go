package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the path picker, driven through App.handleKey with the harness of
// simple_dialog_keys_golden_test.go. The navigate picker is the bookmarks dialog over a fixture
// fzf-marks file (XDG_CONFIG_HOME points at an empty temp dir so no real GTK bookmarks load);
// the apply picker is built directly over fixture history rows. Queries avoid path characters so
// no filesystem completion runs.

func pickerFocusName(app *App) string {
	switch app.model.PathPicker.Focus {
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "filter"
}

// pickerKeyBookmarksDialog is pickerKeyDialog over pickerKeyBookmarksApp, kept serial because that
// setup points XDG_CONFIG_HOME at its fixture marks.
func pickerKeyBookmarksDialog(name string, n int) keyGoldenDialog {
	d := pickerKeyDialog(name, func(t *testing.T, dir string) *App { return pickerKeyBookmarksApp(t, dir, n) })
	d.serial = true
	return d
}

// pickerKeyBookmarksApp opens the bookmarks picker over n fixture marks.
func pickerKeyBookmarksApp(t *testing.T, dir string, n int) *App {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
	var marks strings.Builder
	for i := 0; i < n; i++ {
		a, b := listGoldenWords[i%len(listGoldenWords)], listGoldenWords[(i*7+3)%len(listGoldenWords)]
		fmt.Fprintf(&marks, "%s-%s : /srv/%s/%s-%d\n", a, b, a, b, i)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte(marks.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app := newTestApp(t, newScreen(t, 80, 24), Options{CWD: func() (string, error) { return dir, nil }, Config: cfg})
	app.model.Message = ""
	app.dialogCtrl.OpenBookmarkDialog()
	app.model.Message = ""
	return app
}

// pickerKeyApplyApp opens an apply-purpose picker over n fixture history paths.
func pickerKeyApplyApp(t *testing.T, dir string, n int) *App {
	t.Helper()
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.model.Message = ""
	items := make([]dialog.PathPickerItem, n)
	for i := range items {
		a, b := listGoldenWords[i%len(listGoldenWords)], listGoldenWords[(i*7+3)%len(listGoldenWords)]
		items[i] = dialog.PathPickerItem{Path: fmt.Sprintf("/srv/%s/%s-%d", a, b, i)}
	}
	app.model.PathPicker = dialog.PathPickerState{
		Open: true, Title: "History", Purpose: dialog.PathPickerPurposeApplyTransferDestination, Items: items,
	}
	app.dialogCtrl.SyncPathPickerRanks()
	return app
}

func pickerKeyDialog(name string, setup func(t *testing.T, dir string) *App) keyGoldenDialog {
	return keyGoldenDialog{
		name:  name, // the dialog only: the panels behind it show free disk space
		setup: setup,
		state: func(app *App) string {
			st := app.model.PathPicker
			return fmt.Sprintf("focus=%s selected=%d scroll=%d query=%q qcursor=%d ranked=%d items=%d invalid=%v message=%q",
				pickerFocusName(app), st.Selected, st.ListScroll, st.Query, st.QueryCursor, len(st.Ranked), len(st.Items),
				st.QueryPathInvalid, app.model.Message)
		},
		open: func(app *App) bool { return app.model.PathPicker.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v dest=%q marks=%d message=%q", quit, app.model.TransferDialog.Destination.Value,
				len(app.model.PinnedItems), app.model.Message)
		},
	}
}

func pickerListScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"down-walk", "Down Down Down"},
		{"up-at-top", "Up Up Down Up Up"},
		{"down-to-bottom", strings.Repeat("Down ", 22)},
		{"up-from-bottom", "Ctrl+End Up Up Up"},
		{"pgdn-pgup", "PgDn PgDn PgDn PgUp PgUp PgUp PgUp"},
		{"pgup-at-top", "PgUp Down PgUp"},
		{"pgdn-at-bottom", "Ctrl+End PgDn Down"},
		{"ctrl-home-end", "Down Down Ctrl+End Ctrl+Home Ctrl+End"},
		{"home-end", "Down Down End Home"},
		{"home-end-edit-query", "a b c Home End Left Left Home"},
		{"scroll-down-up", strings.Repeat("Down ", 14) + strings.Repeat("Up ", 14)},
		{"type-filter", "e m"},
		{"type-filter-resets-selection", "Down Down Down e"},
		{"type-filter-edit", "a r Left Backspace x Delete Home Delete"},
		{"type-filter-nomatch", "z z z q"},
		{"type-filter-space", "e Space m"},
		{"type-then-down", "e Down Down Up"},
		{"nomatch-down-up", "z z z q Down Up PgDn PgUp Ctrl+End"},
		{"nomatch-tab", "z z z q Tab Tab Tab Tab"},
		{"nomatch-backtab", "z z z q Backtab Backtab Backtab"},
		{"nomatch-enter", "z z z q Enter"},
		{"nomatch-then-clear", "z z z q Backspace Backspace Backspace Backspace"},
		{"left-right-filter", "a b Left Left Left Right Right Right Right"},
		{"tab-from-list-moved", "Down Down Tab Backtab"},
		{"back-to-filter-keeps-cursor", "Down Down Tab Tab Backtab Backtab Up"},
		{"enter-row", "Down Down Enter"},
		{"enter-filtered-row", "e Enter"},
		{"esc", "Down Esc"},
		{"esc-from-button", "Tab Esc"},
		{"esc-after-typing", "a b Esc"},
		{"alt-o", "Down Alt+o"},
		{"alt-c", "Down Alt+c"},
		{"alt-enter", "Down Alt+Enter"},
		{"alt-unknown", "Alt+x Down"},
		{"plain-letters-on-buttons", "Tab o c x"},
		{"space-on-buttons", "Tab Space"},
		{"delete-backspace-empty", "Backspace Delete Down"},
	}
}

func TestPathPickerNavigateKeyScripts(t *testing.T) {
	d := pickerKeyBookmarksDialog("path_picker_navigate", 20)
	scripts := append(pickerListScripts(),
		simpleKeyScript{"tab-cancel-enter", "Tab Enter"},
		simpleKeyScript{"tab-cancel-space", "Down Tab Space"},
		simpleKeyScript{"tab-cancel-then-right-left", "Tab Tab Tab Enter"},
		simpleKeyScript{"pin-toggle", "Down Alt+Insert Alt+Insert"},
		simpleKeyScript{"open-in-primary", "Down Shift+Left"},
		simpleKeyScript{"open-in-secondary", "Down Shift+Right"},
		simpleKeyScript{"f8-delete", "Down F8"},
		simpleKeyScript{"f8-delete-filtered", "e F8"},
	)
	runKeyGoldenScripts(t, d, scripts)
	runKeyGoldenScripts(t, pickerKeyBookmarksDialog("path_picker_navigate_short", 3),
		[]simpleKeyScript{
			{"walk", "Down Down Down Up Up Up PgDn PgUp Ctrl+End Ctrl+Home"},
			{"filter", "s r"},
		})
	runKeyGoldenScripts(t, pickerKeyBookmarksDialog("path_picker_navigate_empty", 0),
		[]simpleKeyScript{
			{"typing", "x y z Up Tab Backspace"},
			{"enter", "Enter"},
			{"enter-cancel", "Tab Enter"},
		})
}

func TestPathPickerApplyKeyScripts(t *testing.T) {
	d := pickerKeyDialog("path_picker_apply", func(t *testing.T, dir string) *App { return pickerKeyApplyApp(t, dir, 20) })
	scripts := append(pickerListScripts(),
		simpleKeyScript{"tab-ok-enter", "Down Tab Enter"},
		simpleKeyScript{"tab-cancel-enter", "Down Tab Tab Enter"},
		simpleKeyScript{"tab-cancel-enter-right", "Down Tab Right Enter"},
		simpleKeyScript{"space-ok", "Down Tab Space"},
		simpleKeyScript{"space-cancel", "Down Tab Tab Space"},
		simpleKeyScript{"space-cancel-right", "Down Tab Right Space"},
		simpleKeyScript{"plain-o-ok", "Down Tab o"},
		simpleKeyScript{"plain-c-cancel", "Down Tab Tab c"},
		simpleKeyScript{"plain-c-cancel-right", "Down Tab Right c"},
		simpleKeyScript{"plain-o-cancel-right", "Down Tab Right o"},
		simpleKeyScript{"esc-from-cancel-right", "Tab Right Esc"},
		simpleKeyScript{"alt-o-on-cancel-right", "Down Tab Right Alt+o"},
	)
	runKeyGoldenScripts(t, d, scripts)
	runKeyGoldenScripts(t, pickerKeyDialog("path_picker_apply_single", func(t *testing.T, dir string) *App { return pickerKeyApplyApp(t, dir, 1) }),
		[]simpleKeyScript{{"nav", "Down Up PgDn PgUp Ctrl+End Ctrl+Home Tab Tab Tab Enter"}})
}
