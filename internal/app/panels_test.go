package app

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func loadPanelSync(t *testing.T, p *panel.State, dir string) {
	t.Helper()
	sched := p.ScheduleAsyncLoad
	p.ScheduleAsyncLoad = nil
	p.ListingPending = false
	p.ListingPendingPath = ""
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load(%s): %v", dir, err)
	}
	p.ScheduleAsyncLoad = sched
}

func TestRefreshBothPanelsInactiveWalksUpWhenDirectoryDeleted(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	child := filepath.Join(parent, "gone")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, parent)
	loadPanelSync(t, app.inactivePanel(), child)
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	app.dialogCtrl.RefreshBothPanels()
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})

	want := filepath.Clean(parent)
	if got := app.inactivePanel().PathString(); got != want {
		t.Fatalf("inactive path = %q, want %q", got, want)
	}
	if got := app.activePanel().PathString(); got != want {
		t.Fatalf("active path = %q, want unchanged %q", got, want)
	}
}

func TestRefreshBothPanelsActiveWalksUpWhenDirectoryDeleted(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "hollow")
	child := filepath.Join(parent, "pruned")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, parent)
	loadPanelSync(t, app.activePanel(), child)
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	app.dialogCtrl.RefreshBothPanels()
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})

	want := filepath.Clean(parent)
	if got := app.activePanel().PathString(); got != want {
		t.Fatalf("active path = %q, want %q", got, want)
	}
}

func TestRefreshBothPanelsReturnsImmediatelyWhenListingBlocks(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "harbor")
	child := filepath.Join(parent, "pruned")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, parent)
	loadPanelSync(t, app.activePanel(), child)
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	block := make(chan struct{})
	orig := fetchListingForAsyncLoad
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		<-block
		return orig(ctx, snap)
	})

	done := make(chan struct{})
	go func() {
		app.dialogCtrl.RefreshBothPanels()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshBothPanels blocked on listing/Stat")
	}
	if got := app.activePanel().PathString(); got != filepath.Clean(child) {
		t.Fatalf("active path after schedule = %q, want vanished cwd %q", got, child)
	}
	close(block)
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})
	if got := app.activePanel().PathString(); got != filepath.Clean(parent) {
		t.Fatalf("active path after apply = %q, want ancestor %q", got, parent)
	}
}

func TestRefreshBothPanelsStaleClimbDoesNotNavigate(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "harbor")
	child := filepath.Join(parent, "pruned")
	other := t.TempDir()
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, parent)
	active := app.activePanel()
	loadPanelSync(t, active, child)
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	app.dialogCtrl.RefreshBothPanels()
	if err := active.NavigateTo(other, "", 20); err != nil {
		t.Fatalf("NavigateTo other: %v", err)
	}
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})
	if got := active.PathString(); got != filepath.Clean(other) {
		t.Fatalf("path = %q, want %q (stale ancestor climb must not win)", got, other)
	}
}

func setupSFTPStripPanel(t *testing.T) (*App, *panel.State) {
	t.Helper()
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	left := app.panelByID(ui.PrimaryPanel)
	left.Path = pathloc.MustParse("sftp://user@example.com/harbor")
	return app, left
}

func TestNavigateFromSelectionsStripSFTPDirectory(t *testing.T) {
	app, left := setupSFTPStripPanel(t)
	dir := "sftp://user@example.com/meadow"
	left.SelectedPaths = map[string]bool{dir: true}
	left.SelectedDirPaths = map[string]bool{dir: true}
	left.SelectionsStripOrder = []string{dir}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	left.SelectionsStripCursor = 0

	var captured panel.AsyncLoadRequest
	left.ScheduleAsyncLoad = func(req panel.AsyncLoadRequest) bool {
		captured = req
		return true
	}

	app.navigateFromSelectionsStrip()
	if want := "sftp://user@example.com/"; captured.Loc.String() != want {
		t.Fatalf("NavigateTo loc = %q, want containing directory %q (must not os.Stat / filepath.Dir)", captured.Loc.String(), want)
	}
	if captured.SelectedName != "meadow" {
		t.Fatalf("SelectedName = %q, want meadow", captured.SelectedName)
	}
	if app.model.ActiveSubFocus != ui.SubFocusFileList {
		t.Fatalf("ActiveSubFocus = %d, want file list", app.model.ActiveSubFocus)
	}
}

func TestNavigateFromSelectionsStripSFTPFile(t *testing.T) {
	app, left := setupSFTPStripPanel(t)
	file := "sftp://user@example.com/meadow/ember.txt"
	parent := "sftp://user@example.com/meadow"
	left.SelectedPaths = map[string]bool{file: true}
	left.SelectionsStripOrder = []string{file}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	left.SelectionsStripCursor = 0

	var captured panel.AsyncLoadRequest
	left.ScheduleAsyncLoad = func(req panel.AsyncLoadRequest) bool {
		captured = req
		return true
	}

	app.navigateFromSelectionsStrip()
	if captured.Loc.String() != parent {
		t.Fatalf("NavigateTo loc = %q, want sftp parent %q", captured.Loc.String(), parent)
	}
	if captured.SelectedName != "ember.txt" {
		t.Fatalf("SelectedName = %q, want ember.txt", captured.SelectedName)
	}
}

func TestSyncFollowTargetPathSFTPStripRow(t *testing.T) {
	app, left := setupSFTPStripPanel(t)
	dir := "sftp://user@example.com/meadow"
	file := "sftp://user@example.com/meadow/ember.txt"
	left.SelectedPaths = map[string]bool{dir: true, file: true}
	left.SelectedDirPaths = map[string]bool{dir: true}
	left.SelectionsStripOrder = []string{dir, file}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip

	left.SelectionsStripCursor = 0
	got, ok := app.syncFollowTargetPath(left)
	if !ok || got != dir {
		t.Fatalf("sync dir row = %q ok=%v, want %q", got, ok, dir)
	}

	left.SelectionsStripCursor = 1
	got, ok = app.syncFollowTargetPath(left)
	if !ok || got != dir {
		t.Fatalf("sync file row = %q ok=%v, want parent %q", got, ok, dir)
	}
}

func TestQuickViewUpdatesAfterDeletedDirectoryRefresh(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(alpha, "a.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.model.ActivePanel = ui.PrimaryPanel
	left := app.panelByID(ui.PrimaryPanel)
	selectPanelEntryByName(t, left, "alpha")
	app.model.QuickViewEnabled = true
	app.model.QuickViewPanel = ui.PrimaryPanel
	app.previewCtrl.ApplyQuickViewPreviewImmediately()
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.QuickViewDirOverlay.ListingPending
	})
	if got := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()); got != filepath.Clean(alpha) {
		t.Fatalf("overlay path = %q, want %q", got, alpha)
	}

	if err := os.RemoveAll(alpha); err != nil {
		t.Fatal(err)
	}
	app.dialogCtrl.RefreshBothPanels()
	// Both panel reloads race an unrelated leftover async reload of the quick-view overlay's
	// pre-delete snapshot (scheduled by the setup ApplyQuickViewPreviewImmediately above, before
	// alpha was removed) for the same bounded screen event queue, so drain until both panels have
	// actually landed rather than assuming a fixed 2-event order.
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		return !app.model.Primary.ListingPending && !app.model.Secondary.ListingPending
	})

	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("quick view overlay should stay active after delete refresh")
	}
	if got := filepath.Clean(app.model.QuickViewDirOverlay.Path.String()); got != filepath.Clean(beta) {
		t.Fatalf("overlay path = %q, want next highlight %q", got, beta)
	}
}

// TestLocalTreeRestoreUsesChildSchedulerKeepsLoopResponsive covers returning to a previously
// expanded local tree: a child whose prefetch misses the time budget must go through
// ScheduleTreeChildLoad instead of ReadDir on the event goroutine, so a still-blocked child
// fetch cannot stall further input.
func TestLocalTreeRestoreUsesChildSchedulerKeepsLoopResponsive(t *testing.T) {
	root := t.TempDir()
	harbor := filepath.Join(root, "harbor")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(harbor, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "cinder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	left := app.panelByID(ui.PrimaryPanel)
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen) // tree child load for harbor
	if !left.TreeExpanded[harbor] {
		t.Fatal("expected harbor expanded before leaving")
	}
	if err := left.NavigateTo(other, "", 20); err != nil {
		t.Fatalf("NavigateTo other: %v", err)
	}
	applyNextInterruptEvent(t, app, screen)

	app.config.SFTP.ListTimeoutSecs = 1 // prefetch budget is half of the remaining timeout
	block := make(chan struct{})
	t.Cleanup(func() { close(block) })
	swapFetchListingForAsyncLoad(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		if filepath.Clean(snap.Loc.String()) == harbor {
			<-block // wedged child: the prefetch must give up on it
		}
		return panel.FetchListing(ctx, snap)
	})
	var scheduled atomic.Bool
	left.ScheduleTreeChildLoad = func(req panel.TreeChildLoadRequest) bool {
		scheduled.Store(true)
		return true
	}

	if err := left.NavigateTo(root, "", 20); err != nil {
		t.Fatalf("NavigateTo root: %v", err)
	}
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return scheduled.Load() }) // listing apply + restore dispatch
	if !scheduled.Load() {
		t.Fatal("returning to a locally expanded tree must use ScheduleTreeChildLoad")
	}
	if got := left.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount while restore is in flight = %d, want 2", got)
	}

	prior := left.Cursor
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if left.Cursor == prior {
		t.Fatal("Down during in-flight local tree restore did not move the cursor")
	}
}

// TestTreeReentryPrefetchesExpandedChildrenInOneApply covers returning to an expanded tree: the
// remembered subtree is fetched with the root listing, so the single apply shows the whole tree
// and no per-child async load is dispatched.
func TestTreeReentryPrefetchesExpandedChildrenInOneApply(t *testing.T) {
	root := t.TempDir()
	harbor := filepath.Join(root, "harbor")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(harbor, "willow.txt"), filepath.Join(root, "beacon.txt")} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "cinder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	left := app.panelByID(ui.PrimaryPanel)
	app.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelToggleTree)
	applyNextInterruptEvent(t, app, screen)
	if err := left.NavigateTo(other, "", 20); err != nil {
		t.Fatal(err)
	}
	applyNextInterruptEvent(t, app, screen)

	left.ScheduleTreeChildLoad = func(panel.TreeChildLoadRequest) bool {
		t.Error("ScheduleTreeChildLoad called despite prefetch")
		return true
	}
	if err := left.NavigateTo(root, "", 20); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return !left.ListingPending })
	if got := left.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount = %d, want 3 (tree fully restored in one apply)", got)
	}
}
