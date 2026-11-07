package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

var (
	keyCtrlD = tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	keyCtrlU = tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModCtrl)
)

func newOpenHelpApp(t *testing.T, vm ui.ViewMode) *App {
	t.Helper()
	a := newHelpEntriesApp(t)
	a.model.ViewMode = vm
	a.openHelpDialog()
	return a
}

func typeHelpQuery(a *App, text string) {
	for _, r := range text {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func selectedHelpAction(t *testing.T, a *App) string {
	t.Helper()
	st := a.model.HelpView
	if st.Selected < 0 || st.Selected >= len(st.Ranked) {
		t.Fatalf("Selected = %d with %d ranked entries", st.Selected, len(st.Ranked))
	}
	return st.Entries[st.Ranked[st.Selected]].ActionID
}

func TestHelpShortcutMovesCursorWithoutRunningAction(t *testing.T) {
	a := newOpenHelpApp(t, ui.ViewBrowser)
	a.handleKey(keyCtrlD)
	if !a.model.HelpView.Open {
		t.Fatal("help closed")
	}
	if got := selectedHelpAction(t, a); got != keymap.ActionFileDelete {
		t.Fatalf("selected %q, want %q", got, keymap.ActionFileDelete)
	}
	if a.model.ModalDialogOpen() && a.inputMode() != InputModeHelpView {
		t.Fatalf("action ran: input mode %v", a.inputMode())
	}
}

func TestHelpOKAndCancelChordsRevealTheirActions(t *testing.T) {
	for name, tc := range map[string]struct {
		ev   *tcell.EventKey
		want string
	}{
		"Alt+O":     {keyAltO, keymap.ActionPanelOpenDirInOther},
		"Alt+C":     {keyAltC, keymap.ActionCommandsOpen},
		"Alt+Enter": {keyAltEnter, keymap.ActionAppShellInsertPaths},
	} {
		t.Run(name, func(t *testing.T) {
			a := newOpenHelpApp(t, ui.ViewBrowser)
			typeHelpQuery(a, "marigold")
			a.handleKey(tc.ev)
			if !a.model.HelpView.Open {
				t.Fatal("help closed")
			}
			if a.model.HelpView.Query != "" {
				t.Fatalf("Query = %q, want it cleared to reveal the entry", a.model.HelpView.Query)
			}
			if got := selectedHelpAction(t, a); got != tc.want {
				t.Fatalf("selected %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHelpShortcutResolvesInTheViewOverlay(t *testing.T) {
	a := newOpenHelpApp(t, ui.ViewCompare)
	ev, ok := a.keys.Compare.FirstEventKeyForAction(keymap.ActionCompareRefresh)
	if !ok {
		t.Fatal("no chord for compare refresh")
	}
	a.handleKey(ev)
	if got := selectedHelpAction(t, a); got != keymap.ActionCompareRefresh {
		t.Fatalf("selected %q, want %q", got, keymap.ActionCompareRefresh)
	}
}

func TestHelpAnswerBlockerChordReveals(t *testing.T) {
	a := newOpenHelpApp(t, ui.ViewJobs)
	a.handleKey(tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModCtrl))
	if got := selectedHelpAction(t, a); got != keymap.ActionJobsAnswerBlocker {
		t.Fatalf("selected %q, want %q", got, keymap.ActionJobsAnswerBlocker)
	}
}

func TestHelpEditingChordEditsNonEmptyFilterThenReveals(t *testing.T) {
	a := newOpenHelpApp(t, ui.ViewBrowser)
	typeHelpQuery(a, "copy")
	a.handleKey(keyCtrlU)
	if a.model.HelpView.Query != "" {
		t.Fatalf("Query = %q, want the editing chord to clear it", a.model.HelpView.Query)
	}
	a.handleKey(keyCtrlU)
	if got := selectedHelpAction(t, a); got != keymap.ActionPanelClearSelection {
		t.Fatalf("selected %q, want %q", got, keymap.ActionPanelClearSelection)
	}
}

func TestHelpShortcutFromCloseButtonReturnsFocusToFilter(t *testing.T) {
	a := newOpenHelpApp(t, ui.ViewBrowser)
	typeHelpQuery(a, "copy")
	a.model.HelpView.Focus = dialog.HelpClose
	a.handleKey(keyCtrlU)
	if a.model.HelpView.Focus != dialog.HelpFilter {
		t.Fatalf("Focus = %q", a.model.HelpView.Focus)
	}
	if got := selectedHelpAction(t, a); got != keymap.ActionPanelClearSelection {
		t.Fatalf("selected %q, want %q", got, keymap.ActionPanelClearSelection)
	}
}

func TestHelpKeysThatDoNotReveal(t *testing.T) {
	for name, ev := range map[string]*tcell.EventKey{
		"text":         tcell.NewEventKey(tcell.KeyRune, '-', tcell.ModNone),
		"space":        tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone),
		"function key": tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone),
		"motion key":   tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
	} {
		t.Run(name, func(t *testing.T) {
			a := newOpenHelpApp(t, ui.ViewBrowser)
			if a.revealHelpEntry(ev, a.actionFromKeyEvent(ev)) {
				t.Fatal("key revealed an entry")
			}
		})
	}
}

func TestHelpTextEditPageDoesNotReveal(t *testing.T) {
	a := newOpenHelpApp(t, ui.ViewBrowser)
	a.toggleHelpTextEdit()
	if a.revealHelpEntry(keyCtrlD, a.actionFromKeyEvent(keyCtrlD)) {
		t.Fatal("key revealed an entry on the text-editing page")
	}
}
