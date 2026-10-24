package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Key-script goldens for the History dialog, driven through App.handleKey with the harness of
// simple_dialog_keys_golden_test.go. The panel history and the zoxide directories are fixture
// paths that do not exist; the zoxide payload is applied directly so no zoxide binary is run.

func historyFocusName(app *App) string {
	switch app.model.HistoryDialog.Focus {
	case dialog.HistoryZoxide:
		return "zoxide"
	case dlg.OK:
		return "ok"
	case dlg.Cancel:
		return "cancel"
	}
	return "filter"
}

// historyKeyApp opens the dialog over a primary panel history of nHist fixture paths and, when
// nZox > 0, zoxide directories (two of them also in the history, which the dialog dedupes).
func historyKeyApp(t *testing.T, dir string, nHist, nZox int) *App {
	t.Helper()
	app := newApp(t, newScreen(t, 80, 24), dir)
	app.model.Message = ""
	hist := make([]string, nHist)
	for i := range hist {
		a, b := listGoldenWords[i%len(listGoldenWords)], listGoldenWords[(i*7+3)%len(listGoldenWords)]
		hist[i] = fmt.Sprintf("/srv/%s/%s-%d", a, b, i)
	}
	app.model.Primary.History = hist
	app.openHistoryDialog(ui.PrimaryPanel)
	if nZox > 0 {
		zox := make([]string, nZox)
		for i := range zox {
			a, b := listGoldenWords[(i+11)%len(listGoldenWords)], listGoldenWords[(i*5+1)%len(listGoldenWords)]
			zox[i] = fmt.Sprintf("/opt/%s/%s-%d", a, b, i)
		}
		if nHist > 2 {
			zox[0], zox[1] = hist[1], hist[2]
		}
		app.applyHistoryZoxide(historyZoxidePayload{Paths: zox, Installed: true})
	}
	app.model.Message = ""
	return app
}

func historyKeyDialog(name string, nHist, nZox int) keyGoldenDialog {
	ry, rh := 3, 18 // the dialog only: the panels behind it show free disk space
	if nZox > 0 {
		ry, rh = 2, 19
	}
	return keyGoldenDialog{
		name: name, rx: 1, ry: ry, rw: 78, rh: rh,
		setup: func(t *testing.T, dir string) *App { return historyKeyApp(t, dir, nHist, nZox) },
		state: func(app *App) string {
			st := app.model.HistoryDialog
			return fmt.Sprintf("focus=%s selected=%d scroll=%d query=%q qcursor=%d ranked=%d paths=%d zoxide=%v/%v both=%v message=%q",
				historyFocusName(app), st.Selected, st.ListScroll, st.Query, st.QueryCursor, len(st.Ranked), len(st.Paths),
				st.ZoxideAvailable, st.IncludeZoxide, st.BothPanels, app.model.Message)
		},
		open: func(app *App) bool { return app.model.HistoryDialog.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v active=%d pins=%d message=%q", quit, app.model.ActivePanel, len(app.model.PinnedItems), app.model.Message)
		},
	}
}

func historyScripts() []simpleKeyScript {
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
		{"nomatch-tab", "z z z q Tab Tab Tab Tab Tab"},
		{"nomatch-backtab", "z z z q Backtab Backtab Backtab"},
		{"nomatch-enter", "z z z q Enter"},
		{"nomatch-then-clear", "z z z q Backspace Backspace Backspace Backspace"},
		{"left-right-filter", "a b Left Left Left Right Right Right Right"},
		{"tab", "Tab Tab Tab Tab Tab Backtab Backtab Backtab Backtab Backtab"},
		{"tab-from-list-moved", "Down Down Tab Backtab"},
		{"back-to-filter-keeps-cursor", "Down Down Tab Tab Backtab Backtab Up"},
		{"down-up-through-controls", "Tab Down Down Down Down Up Up Up Up"},
		{"left-right-controls", "Tab Left Right Tab Left Right Right Left Left"},
		{"letters-on-controls", "Tab o c Tab o c x"},
		{"plain-space-on-controls", "Tab Space Space Tab Space"},
		{"enter-row", "Down Down Enter"},
		{"enter-filtered-row", "e Enter"},
		{"enter-on-zoxide", "Tab Enter Enter"},
		{"enter-on-ok", "Down Tab Tab Enter"},
		{"enter-on-cancel", "Down Tab Tab Tab Enter"},
		{"enter-on-cancel-right", "Down Tab Tab Right Enter"},
		{"right-left-on-buttons", "Tab Tab Right Right Left Left Left"},
		{"tab-from-cancel", "Tab Tab Right Tab Tab"},
		{"up-from-cancel", "Tab Tab Right Up Up"},
		{"down-from-cancel", "Tab Tab Right Down"},
		{"space-on-ok", "Down Tab Tab Space"},
		{"space-on-cancel", "Down Tab Tab Tab Space"},
		{"space-on-cancel-right", "Down Tab Tab Right Space"},
		{"plain-o-on-ok", "Down Tab Tab o"},
		{"plain-c-on-cancel", "Down Tab Tab Tab c"},
		{"plain-c-on-cancel-right", "Down Tab Tab Right c"},
		{"plain-o-on-cancel-right", "Down Tab Tab Right o"},
		{"plain-c-on-ok", "Tab Tab c"},
		{"esc", "Down Esc"},
		{"esc-from-button", "Tab Tab Tab Esc"},
		{"esc-from-cancel-right", "Tab Tab Right Esc"},
		{"esc-after-typing", "a b Esc"},
		{"alt-o", "Down Alt+o"},
		{"alt-o-on-cancel", "Down Tab Tab Tab Alt+o"},
		{"alt-o-on-cancel-right", "Down Tab Tab Right Alt+o"},
		{"alt-c-on-ok", "Down Tab Tab Alt+c"},
		{"alt-c", "Down Alt+c"},
		{"alt-enter", "Down Alt+Enter"},
		{"alt-unknown", "Alt+x Down"},
		{"alt-i", "Alt+i Alt+i"},
		{"alt-upper-i", "Alt+I"},
		{"alt-i-from-ok", "Tab Tab Alt+i Tab"},
		{"alt-i-keeps-cursor", "Down Down Alt+i Down"},
		{"f5-both-panels", "Down F5 Down F5"},
		{"open-in-primary", "Down Shift+Left"},
		{"open-in-secondary", "Down Shift+Right"},
		{"open-in-primary-from-ok", "Down Tab Tab Shift+Left"},
		{"pin-toggle", "Down Alt+Insert Alt+Insert"},
		{"delete-backspace-empty", "Backspace Delete Down"},
	}
}

func TestHistoryDialogKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, historyKeyDialog("history_dialog", 12, 8), historyScripts())
	runKeyGoldenScripts(t, historyKeyDialog("history_dialog_nozoxide", 20, 0), []simpleKeyScript{
		{"down-walk", "Down Down Down Up"},
		{"to-bottom", strings.Repeat("Down ", 22)},
		{"pgdn-pgup", "PgDn PgDn PgUp"},
		{"type-filter", "e m"},
		{"tab", "Tab Tab Tab Tab Backtab Backtab Backtab"},
		{"down-up-controls", "Tab Down Down Down Up Up Up"},
		{"left-right-controls", "Tab Left Right Tab Left Right"},
		{"enter-row", "Down Enter"},
		{"enter-ok", "Tab Enter"},
		{"enter-cancel", "Tab Tab Enter"},
		{"enter-cancel-right", "Tab Right Enter"},
		{"tab-from-cancel", "Tab Right Tab Tab"},
		{"space-ok", "Tab Space"},
		{"plain-letters", "Tab o"},
		{"plain-letters-cancel", "Tab Tab c"},
		{"plain-letters-cancel-right", "Tab Right c"},
		{"alt-i-ignored", "Alt+i Down"},
		{"alt-o", "Alt+o"},
		{"alt-c", "Alt+c"},
		{"esc", "Esc"},
		{"f5", "F5"},
	})
	runKeyGoldenScripts(t, historyKeyDialog("history_dialog_short", 3, 2), []simpleKeyScript{
		{"walk", "Down Down Down Up Up Up PgDn PgUp Ctrl+End Ctrl+Home"},
		{"filter", "s r"},
		{"tab-around", "Tab Tab Tab Tab"},
		{"toggle-zoxide", "Alt+i Alt+i"},
	})
	runKeyGoldenScripts(t, historyKeyDialog("history_dialog_single", 1, 0), []simpleKeyScript{
		{"nav", "Down Up PgDn PgUp Ctrl+End Ctrl+Home Tab Tab Tab Enter"},
	})
}
