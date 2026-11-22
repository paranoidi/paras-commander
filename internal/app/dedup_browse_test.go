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
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func dedupBrowseSettle(t *testing.T, app *App, screen tcell.SimulationScreen) {
	t.Helper()
	app.reconcileAfterEvent()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		drainScreenInterrupts(app, screen)
		if !app.model.DedupPanel.ListingPending && len(app.model.DedupPanel.Entries) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("dedup browse panel did not load")
}

func dedupBrowseKey(app *App, k tcell.Key) {
	app.handleKey(tcell.NewEventKey(k, 0, tcell.ModNone))
	app.reconcileAfterEvent()
}

func TestDedupBrowsePanelFollowsSourceRowAndTabCycle(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt"), filepath.Join("meadow", "beacon.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Dirs mode: first row is the collapsed dir, Down selects the root file.
	dedupBrowseKey(app, tcell.KeyDown)
	dedupBrowseSettle(t, app, screen)
	p := &app.model.DedupPanel
	if got := p.PathString(); got != dir {
		t.Fatalf("panel path = %q, want file's parent %q", got, dir)
	}
	if e, ok := p.CurrentEntry(); !ok || e.Name != "lantern.txt" {
		t.Fatalf("panel cursor = %v ok=%v, want lantern.txt", e.Name, ok)
	}
	app.render() // browse panel paint must not panic

	// Tab: Main -> Copies -> Panel -> Main.
	dedupBrowseKey(app, tcell.KeyTab)
	if !app.model.DedupView.FocusCopies || app.model.DedupView.FocusPanel {
		t.Fatalf("first Tab: %+v, want copies focus", app.model.DedupView)
	}
	dedupBrowseKey(app, tcell.KeyTab)
	if !app.model.DedupView.FocusPanel {
		t.Fatal("second Tab did not focus the browse panel")
	}

	// Navigate in the panel: Down moves its cursor, not the main one.
	before := app.model.DedupView.Main.Selected
	p.Top(20)
	dedupBrowseKey(app, tcell.KeyDown)
	if app.model.DedupView.Main.Selected != before {
		t.Fatal("Down in browse panel moved the main cursor")
	}
	if p.Cursor == 0 {
		t.Fatal("panel cursor did not move")
	}

	// Tab returns to main.
	dedupBrowseKey(app, tcell.KeyTab)
	if app.model.DedupView.FocusPanel || app.model.DedupView.FocusCopies {
		t.Fatalf("third Tab: %+v, want main focus", app.model.DedupView)
	}
	// Moving the tree cursor re-syncs the panel (discarding manual cursor).
	dedupBrowseKey(app, tcell.KeyUp)
	dedupBrowseKey(app, tcell.KeyDown)
	dedupBrowseSettle(t, app, screen)
	if e, ok := p.CurrentEntry(); !ok || e.Name != "lantern.txt" {
		t.Fatalf("after tree move panel cursor = %q, want lantern.txt", e.Name)
	}

	// Leaving the view from the panel keeps its focus and location for the return trip.
	dedupBrowseKey(app, tcell.KeyTab)
	dedupBrowseKey(app, tcell.KeyTab)
	wantPath := p.PathString()
	dedupBrowseKey(app, tcell.KeyEsc)
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v", app.model.ViewMode)
	}
	app.reconcileAfterEvent()
	app.dispatch(keymap.ActionDedupOpen)
	waitDedupShown(t, app)
	app.reconcileAfterEvent()
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v after dedup.open, want ViewDedup", app.model.ViewMode)
	}
	if !app.model.DedupView.FocusPanel || p.PathString() != wantPath {
		t.Fatalf("returning lost panel state: focus=%v path=%q, want focus on %q",
			app.model.DedupView.FocusPanel, p.PathString(), wantPath)
	}

	// Groups view keeps the panel and its focus.
	app.dedupCtrl.ToggleTreeMode()
	app.reconcileAfterEvent()
	if app.model.DedupPanel.PathString() != wantPath || !app.model.DedupView.FocusPanel {
		t.Fatalf("Groups view lost the browse panel: %+v", app.model.DedupView)
	}
}

func TestDedupBrowsePanelIgnoresPaneSwitch(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	dedupBrowseKey(app, tcell.KeyDown) // root file lantern.txt
	dedupBrowseSettle(t, app, screen)
	p := &app.model.DedupPanel
	if got := p.PathString(); got != dir {
		t.Fatalf("panel path = %q, want %q", got, dir)
	}
	dedupBrowseKey(app, tcell.KeyTab) // Copies
	dedupBrowseKey(app, tcell.KeyTab) // panel
	dedupBrowseSettle(t, app, screen)
	if got := p.PathString(); got != dir {
		t.Fatalf("after Tab x2 panel path = %q, want %q", got, dir)
	}
	dedupBrowseKey(app, tcell.KeyTab) // main
	dedupBrowseKey(app, tcell.KeyTab) // Copies
	// Move the Copies cursor to a different copy: panel follows it.
	want := ""
	for i := range app.model.DedupCopiesList {
		app.model.DedupView.Copies.Selected = i
		if path, _, ok := app.dedupCtrl.PaneTarget(true); ok && filepath.Dir(path) != dir {
			want = filepath.Dir(path)
			break
		}
	}
	if want == "" {
		t.Fatal("no copy outside the root found")
	}
	app.reconcileAfterEvent()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && p.PathString() != want {
		drainScreenInterrupts(app, screen)
		time.Sleep(5 * time.Millisecond)
	}
	if got := p.PathString(); got != want {
		t.Fatalf("after Copies move panel path = %q, want %q", got, want)
	}
}

func TestDedupBrowsePanelHeldNavDefersReload(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt"), filepath.Join("meadow", "beacon.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	dedupBrowseKey(app, tcell.KeyDown) // root file row: panel shows dir
	dedupBrowseSettle(t, app, screen)

	app.config.UI.KeyRepeatDebounceMS = 200
	fpBefore := slices.Clone(app.dedupFollow.prev)
	dedupBrowseKey(app, tcell.KeyUp) // meadow dir row, first press: applies at once (no hint flicker)
	if app.model.DedupView.SourceStale || app.model.DedupView.SourceRow != app.model.DedupView.Main.Selected {
		t.Fatalf("first press deferred: stale=%v src=%d sel=%d",
			app.model.DedupView.SourceStale, app.model.DedupView.SourceRow, app.model.DedupView.Main.Selected)
	}
	if slices.Equal(app.dedupFollow.prev, fpBefore) {
		t.Fatal("first press did not retarget the browse panel")
	}
	srcBefore := app.model.DedupView.SourceRow
	fpHeld := slices.Clone(app.dedupFollow.prev)
	dedupBrowseKey(app, tcell.KeyDown) // repeat inside the window: back to root file row, deferred
	if app.model.DedupView.SourceRow != srcBefore {
		t.Fatalf("copies source changed on repeat: %d->%d", srcBefore, app.model.DedupView.SourceRow)
	}
	if !app.model.DedupView.SourceStale {
		t.Fatal("SourceStale = false on repeat; stale group hints would stay painted")
	}
	if !slices.Equal(app.dedupFollow.prev, fpHeld) {
		t.Fatal("browse panel retargeted on repeat")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (app.model.DedupView.SourceStale || app.model.DedupPanel.PathString() != dir) {
		drainScreenInterrupts(app, screen)
		time.Sleep(5 * time.Millisecond)
	}
	if got := app.model.DedupPanel.PathString(); got != dir {
		t.Fatalf("panel path after flush = %q, want %q", got, dir)
	}
	if got, want := app.model.DedupView.SourceRow, app.model.DedupView.Main.Selected; got != want {
		t.Fatalf("SourceRow after flush = %d, want %d", got, want)
	}
	if app.model.DedupView.SourceStale {
		t.Fatal("SourceStale still set after flush")
	}
}

func TestDedupViewAltFOpensFilterDialog(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.ViewMode = ui.ViewDedup

	if !app.tryDispatchDedup(keymap.ActionDedupFilterDialog) {
		t.Fatal("dedup.filter-dialog not dispatched")
	}
	if !app.model.FilterDialog.Open || app.model.FilterDialog.Target != dialog.FilterTargetDedup {
		t.Fatal("dedup.filter-dialog should open the Filter dialog for dedup groups")
	}
}

// TestDedupAltArrowsWalkHistoryOnlyInBrowsePanel: Alt+Left/Right are directory history in the
// browse panel, and do nothing in the trees, while Ctrl+Left/Right collapse and expand like the
// file list.
func TestDedupAltArrowsWalkHistoryOnlyInBrowsePanel(t *testing.T) {
	dir := t.TempDir()
	meadow := filepath.Join(dir, "meadow")
	if err := os.MkdirAll(meadow, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	alt := func(k tcell.Key) {
		app.handleKey(tcell.NewEventKey(k, 0, tcell.ModAlt))
		app.reconcileAfterEvent()
	}

	dedupBrowseKey(app, tcell.KeyDown)
	dedupBrowseSettle(t, app, screen)
	treeRow := app.model.DedupView.Main.Selected
	alt(tcell.KeyRight)
	alt(tcell.KeyLeft)
	if app.model.DedupView.Main.Selected != treeRow || app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("Alt arrows acted on the tree: row %d, view %v", app.model.DedupView.Main.Selected, app.model.ViewMode)
	}
	if got, _ := app.keys.Dedup.Lookup(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl)); got != keymap.ActionDedupCollapse {
		t.Fatalf("C-left in dedup = %q", got)
	}
	if got, _ := app.keys.Dedup.Lookup(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl)); got != keymap.ActionDedupToggleNode {
		t.Fatalf("C-right in dedup = %q", got)
	}

	dedupBrowseKey(app, tcell.KeyTab)
	dedupBrowseKey(app, tcell.KeyTab)
	p := &app.model.DedupPanel
	if err := p.NavigateTo(meadow, "", 20); err != nil {
		t.Fatal(err)
	}
	waitPanelPath := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			drainScreenInterrupts(app, screen)
			if !p.ListingPending && p.PathString() == want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("panel path = %q, want %q", p.PathString(), want)
	}
	waitPanelPath(meadow)
	alt(tcell.KeyLeft)
	waitPanelPath(dir)
	alt(tcell.KeyRight)
	waitPanelPath(meadow)
}

// dedupBrowseFocusApp opens the duplicates view over dir (lantern.txt, meadow/lantern.txt,
// meadow/beacon.txt, meadow/thicket/) with the browser's inactive panel on its own dest directory,
// and gives the browse panel focus inside meadow.
func dedupBrowseFocusApp(t *testing.T) (app *App, screen tcell.SimulationScreen, dir, meadow, dest string) {
	t.Helper()
	dir = t.TempDir()
	meadow = filepath.Join(dir, "meadow")
	if err := os.MkdirAll(filepath.Join(meadow, "thicket"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt"), filepath.Join("meadow", "beacon.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dest = t.TempDir()
	screen = newScreen(t, 100, 30)
	app = newApp(t, screen, dir)
	if err := app.otherBrowserPanel().Load(dest); err != nil {
		t.Fatal(err)
	}
	app.openFindDuplicates()
	waitDedupDone(t, app)
	dedupBrowseKey(app, tcell.KeyDown)
	dedupBrowseSettle(t, app, screen)
	dedupBrowseKey(app, tcell.KeyTab) // Copies
	dedupBrowseKey(app, tcell.KeyTab) // browse panel
	if !app.model.EmbeddedPanelFocused() || app.focusedPanelID() != app.model.DedupPanelID {
		t.Fatalf("browse panel not focused: %+v", app.model.DedupView)
	}
	p := &app.model.DedupPanel
	if err := p.NavigateTo(meadow, "beacon.txt", 20); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return p.PathString() == meadow && !p.ListingPending })
	return app, screen, dir, meadow, dest
}

func dedupBrowseCursorName(t *testing.T, app *App) string {
	t.Helper()
	e, ok := app.model.DedupPanel.CurrentEntry()
	if !ok {
		t.Fatal("browse panel has no cursor entry")
	}
	return e.Name
}

// The focused browse panel is a full file list: selection, file operations on its own selection with
// the browser's inactive panel as destination, quick filter, sort and navigation, while the view's
// close key still closes the view.
func TestDedupBrowsePanelActsAsFileList(t *testing.T) {
	app, screen, _, meadow, dest := dedupBrowseFocusApp(t)
	p := &app.model.DedupPanel
	beacon := filepath.Join(meadow, "beacon.txt")
	if got := dedupBrowseCursorName(t, app); got != "beacon.txt" {
		t.Fatalf("cursor = %q", got)
	}

	// Space is the view's mark-keep key; with the panel focused it selects like in the browser.
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if !p.SelectedPaths[beacon] || app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("selection = %v, dedup marks = %d", p.SelectedPaths, app.model.DedupView.MarkedCount)
	}

	// Delete confirms the panel's selection, not the duplicates marks, and deletes it.
	dedupBrowseKey(app, tcell.KeyF8)
	fd := app.model.FileDialog
	if !fd.Open || fd.DialogType != dialog.FileDialogDelete || len(fd.DeleteEntries) != 1 || fd.DeleteEntries[0].Path != beacon {
		t.Fatalf("delete dialog: %+v", fd)
	}
	app.dialogCtrl.ExecuteDelete()
	all := app.jobState.AllJobs()
	if len(all) != 1 || all[0].Type != jobs.TypeDelete || all[0].Sources[0].String() != beacon {
		t.Fatalf("jobs after delete = %+v", all)
	}
	if app.model.ViewMode != ui.ViewDedup || !app.model.DedupView.FocusPanel {
		t.Fatalf("view = %v focus panel = %v", app.model.ViewMode, app.model.DedupView.FocusPanel)
	}

	// Copy takes the panel's files as sources and the browser's inactive panel as destination.
	if !p.SelectVisibleEntry("lantern.txt") {
		t.Fatal("no lantern.txt in the browse panel")
	}
	dedupBrowseKey(app, tcell.KeyF5)
	if !app.model.TransferDialog.Open || !strings.HasPrefix(app.model.TransferDialog.Destination.Value, dest) {
		t.Fatalf("copy dialog: %+v", app.model.TransferDialog)
	}
	app.dialogCtrl.ConfirmCopy()
	lantern := filepath.Join(meadow, "lantern.txt")
	copied := slices.ContainsFunc(app.jobState.AllJobs(), func(j *jobs.Job) bool {
		return j.Type == jobs.TypeCopy && len(j.Sources) == 1 && j.Sources[0].String() == lantern
	})
	if !copied {
		t.Fatalf("no copy job of %s: %d jobs", lantern, len(app.jobState.AllJobs()))
	}

	// The quick filter runs on the browse panel: its query lands there and the cursor jumps to the
	// match.
	app.dispatch(keymap.ActionPanelFilterOpen)
	for _, r := range "thick" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if p.Filter.Query != "thick" || dedupBrowseCursorName(t, app) != "thicket" {
		t.Fatalf("browse panel filter = %+v cursor = %q", p.Filter, dedupBrowseCursorName(t, app))
	}
	width, height := screen.Size()
	_, browse := ui.DedupSecondaryRects(app.layoutForTerminalSize(width, height).Secondary)
	if x, y := screenTextCell(t, app, "thick"); y != browse.Y || x < browse.X {
		t.Fatalf("filter query at %d,%d, want on the browse panel's top row %+v", x, y, browse)
	}
	dedupBrowseKey(app, tcell.KeyEsc)
	if p.Filter.Active || p.Filter.Editing || app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("Esc: filter = %+v view = %v", p.Filter, app.model.ViewMode)
	}

	// A sort action re-sorts the browse panel; Alt+S opens the sort dialog for it.
	reverse := p.Sort.Reverse
	app.dispatch(keymap.ActionPanelReverseSort)
	if p.Sort.Reverse == reverse {
		t.Fatal("reverse sort did not change the browse panel")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if !app.model.SortDialog.Open || app.model.SortDialog.PanelID != app.model.DedupPanelID {
		t.Fatalf("sort dialog: %+v", app.model.SortDialog)
	}
	app.closeSortDialog()

	// The history dialog targets the browse panel.
	app.dispatch(keymap.ActionPanelHistoryDialog)
	if !app.model.HistoryDialog.Open || app.model.HistoryDialog.PanelID != app.model.DedupPanelID {
		t.Fatalf("history dialog: open=%v panel=%d, want panel %d", app.model.HistoryDialog.Open, app.model.HistoryDialog.PanelID, app.model.DedupPanelID)
	}
	app.closeHistoryDialog()

	// Navigation: Enter enters a directory, Backspace returns, Right/Left do the same.
	if !p.SelectVisibleEntry("thicket") {
		t.Fatal("no thicket entry")
	}
	dedupBrowseKey(app, tcell.KeyEnter)
	thicket := filepath.Join(meadow, "thicket")
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return p.PathString() == thicket && !p.ListingPending })
	dedupBrowseKey(app, tcell.KeyBackspace2)
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return p.PathString() == meadow && !p.ListingPending })
	if !p.SelectVisibleEntry("thicket") {
		t.Fatal("no thicket entry after parent")
	}
	dedupBrowseKey(app, tcell.KeyRight)
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return p.PathString() == thicket && !p.ListingPending })
	dedupBrowseKey(app, tcell.KeyLeft)
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return p.PathString() == meadow && !p.ListingPending })

	// The view's close key still closes the view.
	dedupBrowseKey(app, tcell.KeyEsc)
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("Esc left view = %v", app.model.ViewMode)
	}
}
