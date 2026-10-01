package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	app.handleDedupViewKey(tcell.NewEventKey(k, 0, tcell.ModNone))
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

	// Groups view has no panel: it is cleared and focus falls back to the main tree.
	app.dedupCtrl.ToggleTreeMode()
	app.reconcileAfterEvent()
	if app.model.DedupPanel.PathString() != "" || app.model.DedupView.FocusPanel || app.model.DedupView.FocusCopies {
		t.Fatalf("Groups view kept the browse panel: %+v", app.model.DedupView)
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
	fpBefore := app.dedupBrowseFP
	dedupBrowseKey(app, tcell.KeyUp) // meadow dir row, first press: applies at once (no hint flicker)
	if app.model.DedupView.SourceStale || app.model.DedupView.SourceRow != app.model.DedupView.Main.Selected {
		t.Fatalf("first press deferred: stale=%v src=%d sel=%d",
			app.model.DedupView.SourceStale, app.model.DedupView.SourceRow, app.model.DedupView.Main.Selected)
	}
	if app.dedupBrowseFP == fpBefore {
		t.Fatal("first press did not retarget the browse panel")
	}
	srcBefore := app.model.DedupView.SourceRow
	fpHeld := app.dedupBrowseFP
	dedupBrowseKey(app, tcell.KeyDown) // repeat inside the window: back to root file row, deferred
	if app.model.DedupView.SourceRow != srcBefore {
		t.Fatalf("copies source changed on repeat: %d->%d", srcBefore, app.model.DedupView.SourceRow)
	}
	if !app.model.DedupView.SourceStale {
		t.Fatal("SourceStale = false on repeat; stale group hints would stay painted")
	}
	if app.dedupBrowseFP != fpHeld {
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
