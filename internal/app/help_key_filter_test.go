package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

var keyF4 = tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone)

func helpFilteredBy(t *testing.T, vm ui.ViewMode, ev *tcell.EventKey) *App {
	t.Helper()
	a := newHelpEntriesApp(t)
	a.model.ViewMode = vm
	a.openHelpDialog()
	a.handleKey(keyF4)
	if !a.model.HelpView.KeyPrompt {
		t.Fatal("F4 did not open the prompt")
	}
	a.handleKey(ev)
	if a.model.HelpView.KeyPrompt || !a.model.HelpView.Open {
		t.Fatalf("after the key: %+v", a.model.HelpView)
	}
	return a
}

// helpFilterSections maps each listed action to its section.
func helpFilterSections(a *App) map[string]string {
	out := map[string]string{}
	for _, e := range a.model.HelpView.Entries {
		out[e.ActionID] = e.Section
	}
	return out
}

func screenText(a *App) string {
	var b strings.Builder
	w, h := a.screen.Size()
	for y := range h {
		for x := range w {
			text, _, _ := a.screen.Get(x, y)
			b.WriteString(text)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestHelpFooterOffersFindByKeypress(t *testing.T) {
	a := newHelpEntriesApp(t)
	a.handleKey(tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone))
	if !strings.Contains(screenText(a), "Find action by keypress") {
		t.Fatalf("footer misses the item:\n%s", screenText(a))
	}
}

func TestHelpKeyPromptDrawsAndCapturesQuit(t *testing.T) {
	a := newHelpEntriesApp(t)
	a.openHelpDialog()
	a.handleKey(keyF4)
	if !strings.Contains(screenText(a), "Press any key") {
		t.Fatalf("prompt not drawn:\n%s", screenText(a))
	}
	quit, _ := a.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if quit {
		t.Fatal("F10 quit from the prompt")
	}
	if a.model.HelpView.KeyFilter != "F10" {
		t.Fatalf("KeyFilter = %q", a.model.HelpView.KeyFilter)
	}
	if _, ok := helpFilterSections(a)[keymap.ActionAppQuit]; !ok {
		t.Fatalf("entries = %+v", a.model.HelpView.Entries)
	}
}

func TestHelpKeyFilterReportsEveryContext(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, keyAltC)
	got := helpFilterSections(a)
	if first := a.model.HelpView.Entries[a.model.HelpView.Ranked[0]].ActionID; first != keymap.ActionCommandsOpen {
		t.Fatalf("first entry %q, want the action the key runs here", first)
	}
	if got[keymap.ActionDialogCancel] != helpOtherContextsSection {
		t.Fatalf("Cancel section = %q in %v", got[keymap.ActionDialogCancel], got)
	}
}

func TestHelpKeyFilterListsEveryActionOnAChord(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl))
	got := helpFilterSections(a)
	if s := got[keymap.ActionFileDelete]; s == "" || s == helpOtherContextsSection {
		t.Fatalf("delete section = %q in %v", s, got)
	}
	if got[keymap.ActionDialogInputRestoreDefault] != helpOtherContextsSection {
		t.Fatalf("restore default missing: %v", got)
	}
	if len(got) < 3 {
		t.Fatalf("want the preview binding too: %v", got)
	}
}

func TestHelpKeyFilterShadowedActionIsNotActive(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewCompare, tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	got := helpFilterSections(a)
	if s := got[keymap.ActionCompareFilterDialog]; s == "" || s == helpOtherContextsSection {
		t.Fatalf("compare filter section = %q", s)
	}
	if s := got[keymap.ActionPanelFilterDialog]; s != helpOtherContextsSection {
		t.Fatalf("shadowed panel filter section = %q", s)
	}
}

func TestHelpKeyFilterListsActionsWithoutHelpView(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, tcell.NewEventKey(tcell.KeyF1, 0, tcell.ModNone))
	if s := helpFilterSections(a)[keymap.ActionAppShowHelp]; s == "" || s == helpOtherContextsSection {
		t.Fatalf("show-help section = %q", s)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !a.model.HelpView.Open {
		t.Fatal("Enter on an entry that cannot run closed help")
	}
}

func TestHelpKeyFilterEscCaptureThenClearThenClose(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, keyEsc)
	if a.model.HelpView.KeyFilter != "Esc" {
		t.Fatalf("KeyFilter = %q", a.model.HelpView.KeyFilter)
	}
	if _, ok := helpFilterSections(a)[keymap.ActionDialogCancel]; !ok {
		t.Fatalf("entries = %+v", a.model.HelpView.Entries)
	}
	a.handleKey(keyEsc)
	if !a.model.HelpView.Open || a.model.HelpView.KeyFilter != "" || len(a.model.HelpView.Entries) < 50 {
		t.Fatalf("Esc did not restore the page: %+v", a.model.HelpView.KeyFilter)
	}
	a.handleKey(keyEsc)
	if a.model.HelpView.Open {
		t.Fatal("second Esc did not close help")
	}
}

func TestHelpKeyFilterUnboundKey(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone))
	if len(a.model.HelpView.Entries) != 0 || !strings.Contains(a.model.HelpView.Title, "Nothing bound to z") {
		t.Fatalf("state = %+v", a.model.HelpView)
	}
}

func TestHelpKeyFilterEnterRunsTheActiveAction(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if a.model.HelpView.Open || !a.model.FileDialog.Open {
		t.Fatalf("help %v file dialog %v", a.model.HelpView.Open, a.model.FileDialog.Open)
	}
}

func TestHelpKeyFilterQueryNarrowsFurther(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, keyEsc)
	all := len(a.model.HelpView.Ranked)
	for _, r := range "cancel" {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	st := a.model.HelpView
	if st.KeyFilter != "Esc" || len(st.Ranked) == 0 || len(st.Ranked) >= all {
		t.Fatalf("ranked %d of %d, filter %q", len(st.Ranked), all, st.KeyFilter)
	}
}

func TestHelpKeyFilterFromTextEditPageReturnsThere(t *testing.T) {
	a := newOpenHelpTextEdit(t)
	a.handleKey(keyF4)
	a.handleKey(keyAltC)
	a.handleKey(keyEsc)
	if !a.model.HelpView.TextEdit || a.model.HelpView.KeyFilter != "" {
		t.Fatalf("state = %+v", a.model.HelpView)
	}
}

func newOpenHelpTextEdit(t *testing.T) *App {
	t.Helper()
	a := newHelpEntriesApp(t)
	a.openHelpDialog()
	a.toggleHelpTextEdit()
	return a
}

func TestHelpKeyPromptCancelButton(t *testing.T) {
	a := newHelpEntriesApp(t)
	a.openHelpDialog()
	a.handleKey(keyF4)
	d, ok := a.activeDialog(keyEsc)
	if !ok {
		t.Fatal("prompt is not the active dialog")
	}
	d.apply(dlg.Event{ID: dialog.HelpKeyPromptCancel})
	if a.model.HelpView.KeyPrompt || !a.model.HelpView.Open {
		t.Fatalf("state = %+v", a.model.HelpView)
	}
}

func TestHelpKeyFilterFooterNamesEscAction(t *testing.T) {
	a := helpFilteredBy(t, ui.ViewBrowser, keyAltC)
	a.render()
	if !strings.Contains(screenText(a), "Esc Clear") {
		t.Fatalf("footer:\n%s", screenText(a))
	}
}

func TestBundleOverlaysCoverEveryTable(t *testing.T) {
	a := newHelpEntriesApp(t)
	overlays := a.keys.Overlays()
	if n := len(keymap.OverlayTableNames()); len(overlays) != n {
		t.Fatalf("%d overlay maps, %d overlay tables", len(overlays), n)
	}
	if slices.Contains(overlays, a.keys.Global) {
		t.Fatal("Overlays includes the global map")
	}
}
