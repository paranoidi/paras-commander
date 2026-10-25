package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

// Key-script goldens for the Pin dialog, driven through App.handleKey with the harness of
// simple_dialog_keys_golden_test.go. Pins are fixture paths: the first is "/" (a directory every
// system has, so Enter on it navigates), the rest do not exist.

func pinKeyApp(t *testing.T, dir string, n int) *App {
	t.Helper()
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.model.Message = ""
	items := make([]ui.PinnedItem, n)
	for i := range items {
		a, b := listGoldenWords[i%len(listGoldenWords)], listGoldenWords[(i*7+3)%len(listGoldenWords)]
		items[i] = ui.PinnedItem{Path: fmt.Sprintf("/srv/%s/%s-%d", a, b, i), IsDir: i%3 != 1}
	}
	if n > 0 {
		items[0] = ui.PinnedItem{Path: "/", IsDir: true}
	}
	app.model.PinnedItems = items
	app.pinCtrl.OpenDialog()
	app.model.Message = ""
	return app
}

func pinKeyDialog(name string, n int) keyGoldenDialog {
	return keyGoldenDialog{
		name:  name, // the dialog only: the panels behind it show free disk space
		setup: func(t *testing.T, dir string) *App { return pinKeyApp(t, dir, n) },
		state: func(app *App) string {
			st := app.model.PinDialog
			return fmt.Sprintf("selected=%d scroll=%d query=%q qcursor=%d ranked=%d pins=%d message=%q",
				st.Selected, st.ListScroll, st.Query, st.QueryCursor, len(st.Ranked), len(app.model.PinnedItems), app.model.Message)
		},
		open: func(app *App) bool { return app.model.PinDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v pins=%d active=%d message=%q", quit, len(app.model.PinnedItems),
				app.model.ActivePanel, app.model.Message)
		},
	}
}

func TestPinDialogKeyScripts(t *testing.T) {
	long := pinKeyDialog("pin_dialog", 20)
	runKeyGoldenScripts(t, long, []simpleKeyScript{
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
		{"nomatch-tab", "z z z q Tab Tab Backtab"},
		{"nomatch-enter", "z z z q Enter"},
		{"nomatch-then-clear", "z z z q Backspace Backspace Backspace Backspace"},
		{"left-right-filter", "a b Left Left Left Right Right Right Right"},
		{"tab-from-list-moved", "Down Down Tab Backtab"},
		{"enter-root", "Enter"},
		{"enter-missing", "Down Enter"},
		{"enter-filtered", "e Enter"},
		{"esc", "Down Esc"},
		{"esc-after-typing", "a b Esc"},
		{"alt-o", "Down Alt+o"},
		{"alt-o-root", "Alt+o"},
		{"alt-c", "Down Alt+c"},
		{"alt-enter", "Alt+Enter"},
		{"alt-unknown", "Alt+x Down"},
		{"alt-letters-in-filter", "a Alt+a Alt+i"},
		{"f3-dir", "F3"},
		{"f3-file", "Down F3"},
		{"f8-remove", "Down Down F8"},
		{"f8-remove-last", "Ctrl+End F8 F8"},
		{"f8-remove-filtered", "e F8"},
		{"shift-f8-remove-all", "Down Shift+F8"},
		{"open-in-primary", "Down Shift+Left"},
		{"open-in-secondary", "Down Shift+Right"},
		{"open-in-primary-root", "Shift+Left"},
		{"open-in-secondary-root", "Shift+Right"},
		{"delete-backspace-empty", "Backspace Delete Down"},
	})
	short := pinKeyDialog("pin_dialog_short", 3)
	runKeyGoldenScripts(t, short, []simpleKeyScript{
		{"walk", "Down Down Down Up Up Up PgDn PgUp Ctrl+End Ctrl+Home"},
		{"filter", "s r"},
		{"remove-down-to-empty", "F8 F8 F8 Down"},
	})
	one := pinKeyDialog("pin_dialog_single", 1)
	runKeyGoldenScripts(t, one, []simpleKeyScript{
		{"nav", "Down Up PgDn PgUp Ctrl+End Ctrl+Home Tab Enter"},
	})
}
