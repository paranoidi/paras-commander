package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Key-script goldens for the help dialog, driven through App.handleKey with the harness of
// simple_dialog_keys_golden_test.go. The entries are fixtures (not the real keymap) so the goldens
// do not change when shortcuts are added; two of them name real, harmless actions so Enter on an
// entry can be observed (panel.switch flips the active panel, panel.toggle-hidden flips hidden
// files), the others name no action and leave the dialog open. The F2 text-editing page is built
// from the real keymap and is not part of the goldens.

// helpRect is the dialog on a 100x30 screen: 86 wide, 16 tall (10 list rows), centered.
const helpRectX, helpRectY, helpRectW, helpRectH = 7, 7, 86, 16

func helpKeyEntries(n int) []dialog.HelpEntry {
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper"}
	keys := []string{"F5", "Ctrl+Shift+Left", "Alt+O", "Insert", "Ctrl+Alt+PgDn", "F10", "Shift+F6", "Backspace"}
	sections := []string{"Navigation", "Selection", "Files", "Panels", "Views"}
	out := make([]dialog.HelpEntry, n)
	for i := range out {
		e := dialog.HelpEntry{
			ActionID: fmt.Sprintf("area.%s-%d", words[i%len(words)], i),
			Title:    fmt.Sprintf("%s the %s", strings.ToUpper(words[i%len(words)][:1])+words[i%len(words)][1:], words[(i*3+1)%len(words)]),
			Keys:     keys[i%len(keys)],
			Section:  sections[i*len(sections)/max(n, 1)],
		}
		if i%7 == 6 {
			e.Keys = "Ctrl+Shift+Alt+PgDn | Ctrl+Shift+Alt+PgUp | Ctrl+Home"
		}
		e.FuzzyExtra = e.ActionID
		out[i] = e
	}
	if n > 10 {
		out[3].ActionID = keymap.ActionPanelSwitch
		out[10].ActionID = keymap.ActionPanelToggleHidden
	}
	return out
}

// helpFocusName names the dialog focus: the filter / list, or the Close button.
func helpFocusName(st dialog.HelpViewState) string {
	if st.Focus == dialog.HelpClose {
		return "close"
	}
	return "list"
}

func helpKeyDialog(name string, entries int) keyGoldenDialog {
	return keyGoldenDialog{
		name: name, rx: helpRectX, ry: helpRectY, rw: helpRectW, rh: helpRectH,
		setup: func(t *testing.T, dir string) *App {
			app := newApp(t, newScreen(t, 100, 30), dir)
			app.model.Message = ""
			app.openHelpDialog()
			app.model.HelpView.Entries = helpKeyEntries(entries)
			app.syncHelpRanks()
			return app
		},
		state: func(app *App) string {
			st := app.model.HelpView
			return fmt.Sprintf("focus=%s selected=%d scroll=%d query=%q qcursor=%d qscroll=%d ranked=%d title=%q",
				helpFocusName(st), st.Selected, st.ListScroll, st.Query, st.QueryCursor, st.QueryScroll, len(st.Ranked), st.Title)
		},
		open: func(app *App) bool { return app.model.HelpView.Open },
		outcome: func(app *App, quit bool) string {
			return fmt.Sprintf("quit=%v active=%d hidden=%v message=%q", quit, app.model.ActivePanel, app.model.Primary.ShowHidden, app.model.Message)
		},
	}
}

func helpScripts() []simpleKeyScript {
	return []simpleKeyScript{
		{"down-walk", "Down Down Down Down"},
		{"up-at-top", "Up Down Up Up"},
		{"down-through-end", strings.Repeat("Down ", 12)},
		{"down-to-bottom", strings.Repeat("Down ", 29)},
		{"down-past-bottom-focuses-close", strings.Repeat("Down ", 31)},
		{"down-on-close-noop", strings.Repeat("Down ", 30) + "Down Down"},
		{"up-from-close-selects-last", strings.Repeat("Down ", 30) + "Up"},
		{"up-from-close-after-tab", "Tab Up"},
		{"up-from-close-then-up", "Tab Up Up Up"},
		{"pgdn-pgup", "PgDn PgDn PgDn PgUp PgUp PgUp PgUp"},
		{"pgdn-first-selects-view-bottom", "PgDn"},
		{"pgdn-at-bottom", "Ctrl+End PgDn Down"},
		{"pgup-at-top", "PgUp Down PgUp"},
		{"pgup-from-bottom", "Ctrl+End PgUp PgUp"},
		{"ctrl-home-end", "Down Down Ctrl+End Ctrl+Home Ctrl+End"},
		{"home-end-are-text", "a b c Home End Left Left Home"},
		{"home-end-plain-after-moving", "Down Down End Home"},
		{"scroll-down-up", strings.Repeat("Down ", 20) + strings.Repeat("Up ", 20)},
		{"scroll-reveals-header", strings.Repeat("Down ", 12) + strings.Repeat("Up ", 4)},
		{"type-filter", "a r"},
		{"type-filter-resets-selection", "Down Down Down e"},
		{"type-filter-edit", "a r Left Backspace x Delete Home Delete"},
		{"type-filter-nomatch", "z z z q"},
		{"type-filter-space", "a r Space e"},
		{"type-then-down", "a r Down Down Up"},
		{"nomatch-down-up", "z z z q Down Up PgDn PgUp Ctrl+End"},
		{"nomatch-tab", "z z z q Tab Tab Tab"},
		{"nomatch-enter-closes", "z z z q Enter"},
		{"nomatch-then-clear", "z z z q Backspace Backspace Backspace Backspace"},
		{"left-right-filter", "a b Left Left Left Right Right Right Right"},
		{"tab", "Tab Tab Tab Tab"},
		{"backtab", "Backtab Backtab Backtab"},
		{"tab-from-list-moved", "Down Down Tab Backtab"},
		{"back-to-filter-keeps-cursor", "Down Down Tab Tab Down"},
		{"left-right-on-close", "Tab Left Right Right Left"},
		{"letters-on-close", "Tab c o x"},
		{"plain-space-on-close", "Tab Space Space"},
		{"enter-fake-entry-stays-open", "Down Enter"},
		{"enter-real-entry-switches-panel", "Down Down Down Enter"},
		{"enter-real-entry-hidden", strings.Repeat("Down ", 10) + "Enter"},
		{"enter-filtered-entry", "t o g g Enter"},
		{"enter-filtered-second-entry", "t o g g Down Enter"},
		{"enter-on-close", "Tab Enter"},
		{"enter-on-close-after-moving", "Down Down Tab Enter"},
		{"esc", "Down Esc"},
		{"esc-from-close", "Tab Esc"},
		{"esc-after-typing", "a b Esc"},
		{"alt-c", "Down Alt+c"},
		{"alt-c-on-close", "Tab Alt+c"},
		{"alt-o-activates-entry", "Down Down Down Alt+o"},
		{"alt-o-nomatch", "z z z q Alt+o"},
		{"alt-enter", "Down Alt+Enter"},
		{"alt-unknown", "Alt+x Down"},
		{"alt-upper-c", "Alt+C"},
		{"delete-backspace-empty", "Backspace Delete Down"},
		{"f5-ignored", "F5 Down"},
	}
}

func TestHelpDialogKeyScripts(t *testing.T) {
	runKeyGoldenScripts(t, helpKeyDialog("help_dialog", 30), helpScripts())
	runKeyGoldenScripts(t, helpKeyDialog("help_dialog_short", 5), []simpleKeyScript{
		{"walk", "Down Down Down Down Down Down Up Up Up Up Up Up"},
		{"pages", "PgDn PgUp PgDn Ctrl+End Ctrl+Home"},
		{"filter", "a r"},
		{"tab-around", "Tab Tab Tab"},
		{"down-focuses-close", "Down Down Down Down Down Down Down"},
		{"enter", "Enter"},
	})
	runKeyGoldenScripts(t, helpKeyDialog("help_dialog_big", 120), []simpleKeyScript{
		{"down-walk", strings.Repeat("Down ", 25)},
		{"pgdn-walk", strings.Repeat("PgDn ", 14)},
		{"pgup-walk", "Ctrl+End " + strings.Repeat("PgUp ", 14)},
		{"ctrl-home-end", "Ctrl+End Ctrl+Home"},
		{"filter-then-pages", "e PgDn PgDn PgUp"},
	})
	runKeyGoldenScripts(t, helpKeyDialog("help_dialog_empty", 0), []simpleKeyScript{
		{"nav-noop", "Down Up PgDn PgUp Ctrl+End Ctrl+Home"},
		{"down-focuses-close", "Down"},
		{"tab", "Tab Tab"},
		{"enter-closes", "Enter"},
		{"type", "a b"},
	})
}
