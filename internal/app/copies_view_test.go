package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// copiesPanelSettle waits for the Other copies panel to list dir with its cursor on harbor.txt.
func copiesPanelSettle(t *testing.T, a *App, screen tcell.SimulationScreen, dir string) {
	t.Helper()
	p := &a.model.CopiesPanel
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		a.reconcileAfterEvent()
		e, ok := p.CurrentEntry()
		return p.PathString() == dir && !p.ListingPending && ok && e.Name == "harbor.txt"
	})
}

// openCopiesFor runs fileindex.other-copies on harbor.txt and waits for the view, with its panel on
// panelDir.
func openCopiesFor(t *testing.T, a *App, screen tcell.SimulationScreen, panelDir string) {
	t.Helper()
	cursorTo(t, a, "harbor.txt")
	a.dispatch(keymap.ActionFileIndexCopies)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.ViewMode == ui.ViewCopies })
	a.reconcileAfterEvent()
	copiesPanelSettle(t, a, screen, panelDir)
}

func copiesKey(a *App, k tcell.Key, r rune, mod tcell.ModMask) {
	a.handleKey(tcell.NewEventKey(k, r, mod))
	a.reconcileAfterEvent()
}

func copiesPaths(a *App) []string {
	var out []string
	for _, f := range a.model.CopiesView.Files {
		out = append(out, f.Path)
	}
	return out
}

func TestCopiesViewListsCopiesAndShowsFirstDirectory(t *testing.T) {
	for _, onIndex := range []bool{false, true} {
		a, screen, dir := copiesAppWith(t, "meadow/harbor.txt", "orchard/harbor.txt", "willow/harbor.txt")
		panelDir := filepath.Join(dir, "meadow")
		if onIndex {
			openIndexPanel(t, a, screen, dir)
			panelDir = "fileindex://" + panelDir
		}
		openCopiesFor(t, a, screen, panelDir)

		view := a.model.CopiesView
		want := []string{
			filepath.Join(dir, "meadow", "harbor.txt"),
			filepath.Join(dir, "orchard", "harbor.txt"),
			filepath.Join(dir, "willow", "harbor.txt"),
		}
		if !slices.Equal(copiesPaths(a), want) || view.Origin != "harbor.txt" || view.IndexPanel != onIndex {
			t.Fatalf("onIndex=%v view = %+v, want copies %v", onIndex, view, want)
		}
		if got := view.Files[0].Size; got != int64(len("meadow/harbor.txt")) {
			t.Fatalf("onIndex=%v size = %d", onIndex, got)
		}
		if a.panelInst(a.model.CopiesPanelID) == nil || a.model.CopiesPanelID == 0 {
			t.Fatalf("onIndex=%v: the panel is not registered", onIndex)
		}
		if a.model.FocusedPanelID() != a.model.ActivePanel {
			t.Fatal("the list has focus at first, not the panel")
		}
		a.render()
		if !strings.Contains(screenText(a), "Other copies of harbor.txt (3)") {
			t.Fatalf("onIndex=%v: no title on screen", onIndex)
		}
	}
}

func TestCopiesViewPanelFollowsOnlyTheListCursor(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt", "orchard/harbor.txt")
	meadow, orchard := filepath.Join(dir, "meadow"), filepath.Join(dir, "orchard")
	openCopiesFor(t, a, screen, meadow)
	p := &a.model.CopiesPanel

	// Switching to the panel and navigating inside it.
	copiesKey(a, tcell.KeyTab, 0, tcell.ModNone)
	if !a.model.CopiesView.FocusPanel || a.focusedPanelID() != a.model.CopiesPanelID {
		t.Fatalf("Tab did not focus the panel: %+v", a.model.CopiesView)
	}
	if err := p.NavigateTo(dir, "", 20); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool { return p.PathString() == dir && !p.ListingPending })

	// Switching back and forth keeps what the user did there.
	copiesKey(a, tcell.KeyTab, 0, tcell.ModNone)
	a.reconcileAfterEvent()
	copiesKey(a, tcell.KeyTab, 0, tcell.ModNone)
	drainScreenInterrupts(a, screen)
	if p.PathString() != dir {
		t.Fatalf("a pane switch moved the panel to %q", p.PathString())
	}
	copiesKey(a, tcell.KeyTab, 0, tcell.ModNone) // back to the list

	// Only a list cursor move re-syncs it.
	copiesKey(a, tcell.KeyDown, 0, tcell.ModNone)
	copiesPanelSettle(t, a, screen, orchard)
	copiesKey(a, tcell.KeyUp, 0, tcell.ModNone)
	copiesPanelSettle(t, a, screen, meadow)
}

func TestCopiesViewPanelActsAsFileList(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt")
	meadow, dest := filepath.Join(dir, "meadow"), t.TempDir()
	if err := a.otherBrowserPanel().Load(dest); err != nil {
		t.Fatal(err)
	}
	openCopiesFor(t, a, screen, meadow)
	copiesKey(a, tcell.KeyTab, 0, tcell.ModNone)
	p := &a.model.CopiesPanel
	copy := filepath.Join(meadow, "harbor.txt")

	// Space selects in the panel; the list's own marks stay untouched.
	copiesKey(a, tcell.KeyRune, ' ', tcell.ModNone)
	if !p.SelectedPaths[copy] || len(a.model.CopiesView.Marked) != 0 {
		t.Fatalf("selection = %v, view marks = %v", p.SelectedPaths, a.model.CopiesView.Marked)
	}
	// Insert selects too.
	copiesKey(a, tcell.KeyInsert, 0, tcell.ModNone)
	if p.SelectedPaths[copy] {
		t.Fatalf("Insert did not toggle the selection: %v", p.SelectedPaths)
	}

	// Quick filter.
	a.dispatch(keymap.ActionPanelFilterOpen)
	for _, r := range "harb" {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if p.Filter.Query != "harb" {
		t.Fatalf("panel filter = %+v", p.Filter)
	}
	copiesKey(a, tcell.KeyEsc, 0, tcell.ModNone)
	if p.Filter.Active || p.Filter.Editing || a.model.ViewMode != ui.ViewCopies {
		t.Fatalf("Esc: filter = %+v, view = %v", p.Filter, a.model.ViewMode)
	}

	// Its destination is the browser's inactive panel.
	copiesKey(a, tcell.KeyF5, 0, tcell.ModNone)
	if !a.model.TransferDialog.Open || !strings.HasPrefix(a.model.TransferDialog.Destination.Value, dest) {
		t.Fatalf("copy dialog from the panel: %+v", a.model.TransferDialog)
	}
}

func TestCopiesViewDeleteTakesTheMarkedCopies(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt", "orchard/harbor.txt", "willow/harbor.txt")
	openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
	copiesKey(a, tcell.KeyRune, ' ', tcell.ModNone) // meadow, cursor moves to orchard
	copiesKey(a, tcell.KeyRune, ' ', tcell.ModNone) // orchard
	if len(a.model.CopiesView.Marked) != 2 || a.model.CopiesView.MarkedBytes == 0 {
		t.Fatalf("marks = %v (%d bytes)", a.model.CopiesView.Marked, a.model.CopiesView.MarkedBytes)
	}

	copiesKey(a, tcell.KeyF8, 0, tcell.ModNone)
	fd := a.model.FileDialog
	want := []string{filepath.Join(dir, "meadow", "harbor.txt"), filepath.Join(dir, "orchard", "harbor.txt")}
	var got []string
	for _, e := range fd.DeleteEntries {
		got = append(got, e.Path)
	}
	if !fd.Open || fd.DialogType != dialog.FileDialogDelete || !slices.Equal(got, want) {
		t.Fatalf("delete dialog = %+v, want entries %v", fd, want)
	}
	a.dialogCtrl.ExecuteDelete()

	all := a.jobState.AllJobs()
	if len(all) != 1 || all[0].Type != jobs.TypeDelete || len(all[0].Sources) != 2 {
		t.Fatalf("jobs = %+v", all)
	}
	if rest := copiesPaths(a); !slices.Equal(rest, []string{filepath.Join(dir, "willow", "harbor.txt")}) {
		t.Fatalf("copies after the delete = %v", rest)
	}
	if len(a.model.CopiesView.Marked) != 0 || a.model.ViewMode != ui.ViewCopies {
		t.Fatalf("marks = %v, view = %v", a.model.CopiesView.Marked, a.model.ViewMode)
	}
}

func TestCopiesViewTransfersCursorCopyToInactivePanel(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt", "orchard/harbor.txt")
	dest := t.TempDir()
	if err := a.otherBrowserPanel().Load(dest); err != nil {
		t.Fatal(err)
	}
	openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
	first := filepath.Join(dir, "meadow", "harbor.txt")

	copiesKey(a, tcell.KeyF5, 0, tcell.ModNone)
	if !a.model.TransferDialog.Open || !strings.HasPrefix(a.model.TransferDialog.Destination.Value, dest) {
		t.Fatalf("copy dialog: %+v", a.model.TransferDialog)
	}
	a.dialogCtrl.ConfirmCopy()
	if !slices.ContainsFunc(a.jobState.AllJobs(), func(j *jobs.Job) bool {
		return j.Type == jobs.TypeCopy && len(j.Sources) == 1 && j.Sources[0].String() == first
	}) {
		t.Fatalf("no copy job of %s", first)
	}
	if len(a.model.CopiesView.Files) != 2 {
		t.Fatalf("a copy dropped a row: %v", copiesPaths(a))
	}

	copiesKey(a, tcell.KeyF6, 0, tcell.ModNone)
	if !a.model.TransferDialog.Open {
		t.Fatal("the move dialog did not open")
	}
	a.dialogCtrl.ConfirmCopy()
	if rest := copiesPaths(a); !slices.Equal(rest, []string{filepath.Join(dir, "orchard", "harbor.txt")}) {
		t.Fatalf("copies after the move = %v", rest)
	}
}

func TestCopiesViewCloseClosesThePanel(t *testing.T) {
	for _, fromPanel := range []bool{false, true} {
		a, screen, dir := copiesAppWith(t, "meadow/harbor.txt")
		openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
		id := a.model.CopiesPanelID
		if fromPanel {
			copiesKey(a, tcell.KeyTab, 0, tcell.ModNone)
		}
		copiesKey(a, tcell.KeyEsc, 0, tcell.ModNone)
		if a.model.ViewMode != ui.ViewBrowser || a.model.CopiesView.Open {
			t.Fatalf("fromPanel=%v: view = %v", fromPanel, a.model.ViewMode)
		}
		if a.panelInst(id) != nil || a.model.CopiesPanelID != 0 || a.model.FocusedPanelID() != a.model.ActivePanel {
			t.Fatalf("fromPanel=%v: the panel outlived the view", fromPanel)
		}
	}
}

func TestCopiesViewKeepsItsPanelBehindTheViewer(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt")
	openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
	id := a.model.CopiesPanelID
	copiesKey(a, tcell.KeyF3, 0, tcell.ModNone)
	if !a.model.FullscreenFilePreview.Open || a.panelInst(id) == nil {
		t.Fatalf("viewer open = %v, panel alive = %v", a.model.FullscreenFilePreview.Open, a.panelInst(id) != nil)
	}
	copiesKey(a, tcell.KeyEsc, 0, tcell.ModNone)
	if a.model.ViewMode != ui.ViewCopies || a.panelInst(id) == nil {
		t.Fatalf("after the viewer: view = %v, panel alive = %v", a.model.ViewMode, a.panelInst(id) != nil)
	}
}

func TestCopiesViewOpensCopyInBrowserPanel(t *testing.T) {
	for _, right := range []bool{false, true} {
		a, screen, dir := copiesAppWith(t, "meadow/harbor.txt")
		openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
		target := filepath.Join(dir, "meadow")
		key := tcell.KeyLeft
		wantPanel := a.panelByID(a.model.FirstSlotPanel())
		if right {
			key, wantPanel = tcell.KeyRight, a.panelByID(a.model.SecondSlotPanel())
		}
		// Elsewhere first, so the jump is visible.
		if err := wantPanel.Load(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		a.handleKey(tcell.NewEventKey(key, 0, tcell.ModShift))
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
			e, ok := wantPanel.CurrentEntry()
			return wantPanel.PathString() == target && !wantPanel.ListingPending && ok && e.Name == "harbor.txt"
		})
		if a.model.ViewMode != ui.ViewCopies {
			t.Fatalf("right=%v: opening in a panel left the view", right)
		}
	}
}

func TestCopiesViewMouseFocusesAndSelects(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt", "orchard/harbor.txt")
	openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
	width, height := screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	list, panelRect := ui.CopiesRects(layout, a.model.ActivePanel)

	// A click on the panel focuses it; a click on the second row focuses the list and selects it.
	click(a, panelRect.X+3, ui.PanelListTop(panelRect), tcell.Button1)
	if !a.model.CopiesView.FocusPanel {
		t.Fatal("clicking the panel did not focus it")
	}
	click(a, list.X+3, ui.PanelListTop(list)+1, tcell.Button1)
	a.reconcileAfterEvent()
	if a.model.CopiesView.FocusPanel || a.model.CopiesView.Pane.Selected != 1 {
		t.Fatalf("clicking row 2: %+v", a.model.CopiesView)
	}
	copiesPanelSettle(t, a, screen, filepath.Join(dir, "orchard"))
}

func TestCopiesViewFooterAndHelp(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt")
	openCopiesFor(t, a, screen, filepath.Join(dir, "meadow"))
	hints := map[string]bool{}
	keys, ok := a.auxiliaryViewFooterKeys()
	if !ok {
		t.Fatal("the view has no footer")
	}
	for _, k := range keys {
		hints[k.Hint] = true
	}
	for _, want := range []string{"Help", "View", "Copy", "Move", "Delete"} {
		if !hints[want] {
			t.Errorf("footer misses %q: %v", want, hints)
		}
	}
	a.openHelpDialog()
	if !a.model.HelpView.Open || a.model.HelpView.Title != "Help — Other copies" || len(a.model.HelpView.Entries) == 0 {
		t.Fatalf("help = %+v", a.model.HelpView)
	}
}

// A hidden view panel is not re-listed by a job refresh, only marked stale, and re-lists when
// its view is shown again.
func TestCopiesPanelRefreshDeferredWhileHidden(t *testing.T) {
	a, screen, dir := copiesAppWith(t, "meadow/harbor.txt", "orchard/harbor.txt")
	panelDir := filepath.Join(dir, "meadow")
	openCopiesFor(t, a, screen, panelDir)
	has := func(name string) bool {
		return slices.ContainsFunc(a.model.CopiesPanel.Entries, func(e localfs.Entry) bool { return e.Name == name })
	}
	if err := os.WriteFile(filepath.Join(panelDir, "lantern.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	a.model.ViewMode = ui.ViewBrowser
	a.dialogCtrl.RefreshBothPanels()
	if !a.panelInst(a.model.CopiesPanelID).stale || a.model.CopiesPanel.ListingPending || has("lantern.txt") {
		t.Fatalf("hidden panel: stale=%v pending=%v", a.panelInst(a.model.CopiesPanelID).stale, a.model.CopiesPanel.ListingPending)
	}

	a.model.ViewMode = ui.ViewCopies
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		a.reconcileAfterEvent()
		return !a.model.CopiesPanel.ListingPending && has("lantern.txt")
	})
	if a.panelInst(a.model.CopiesPanelID).stale {
		t.Fatal("stale flag survives the re-list")
	}
}
