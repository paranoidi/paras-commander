package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Regression: disk-total resort after Parent must not undo centered scroll from ApplyListing.
func TestParentStaysCenteredAfterDiskUsageResort(t *testing.T) {
	root := t.TempDir()
	bar := filepath.Join(root, "bar")
	asdf := filepath.Join(bar, "asdf")
	if err := os.MkdirAll(asdf, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(bar, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(bar, "00_dir", "leaf.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, bar)
	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskSorter = app.disk.engine.Size
	app.setDiskUsageScanScope(bar, []string{bar})
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !left.ListingFullyDiskCached() {
		t.Fatal("listing not fully disk-cached")
	}

	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, left, "asdf")
	if _, err := left.Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	app.dispatch(keymap.ActionNavParent)
	app.resortPanelsDiskUsageSorted()

	vr := app.activeViewportRows()
	p := app.activePanel()
	row := p.Cursor - p.ScrollOffset
	mid := vr / 2
	if row != mid && row != vr-1 {
		t.Fatalf("after Parent+disk resort: viewport row = %d, want %d or %d; cursor=%d scroll=%d",
			row, mid, vr-1, p.Cursor, p.ScrollOffset)
	}
}

// reconcileAfterEvent walks both panels. Uncached listings must not enable IdleDiskTotalsSort;
// per-panel idle timers are only armed once ListingFullyDiskCached holds ( subtree events ).
func TestDiskUsageIdleArmingSurvivesPanelSwitchViaReconciler(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	right := app.panelByID(ui.SecondaryPanel)
	right.Sort.DiskUsageIdleSizeSort = true
	right.DiskUsageIdleSortActivated = true
	right.IdleDiskTotalsSort = false

	if app.disk.idleSort[ui.SecondaryPanel].timer != nil {
		t.Fatal("right idle timer should be nil before reconcile")
	}

	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.ActivePanel != ui.SecondaryPanel {
		t.Fatalf("ActivePanel after switch = %d, want SecondaryPanel", app.model.ActivePanel)
	}
	app.reconcileAfterEvent()

	if app.disk.idleSort[ui.SecondaryPanel].timer != nil {
		t.Fatal("uncached listing must not arm idle-sort timer from reconcile alone")
	}
	if right.IdleDiskTotalsSort {
		t.Fatalf("IdleDiskTotalsSort = true; want false until listing is fully disk-cached")
	}
}

func TestApplyIdleDiskSortRequiresFullListingCache(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	alphaPath := filepath.Join(root, "alpha")

	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) {
		if filepath.Clean(abs) == filepath.Clean(alphaPath) {
			return 42, true
		}
		return 0, false
	}

	app.applyIdleDiskSort(ui.PrimaryPanel, app.disk.idleSort[ui.PrimaryPanel].epoch)

	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should stay false when listing is not fully disk-cached")
	}
}

func TestHandlePanelDirChangedRightDoesNotInvalidateLeftIdleTimer(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "f.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	leftRoot := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String())
	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }
	app.setDiskUsageShown(true)
	app.setDiskUsageScanScope(leftRoot, []string{filepath.Clean(alpha)})

	app.disk.idleNavPath[ui.PrimaryPanel] = leftRoot
	app.armIdleDiskSortTimer(ui.PrimaryPanel)
	if app.disk.idleSort[ui.PrimaryPanel].timer == nil {
		t.Fatal("expected idle timer armed")
	}

	right := app.panelByID(ui.SecondaryPanel)
	right.Sort.DiskUsageIdleSizeSort = true
	right.DiskUsageIdleSortActivated = true
	right.IdleDiskTotalsSort = false
	vr := app.panelViewportRows(ui.SecondaryPanel)
	if err := right.NavigateTo(filepath.Clean(alpha), "", vr); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}

	app.handlePanelDirChanged(ui.SecondaryPanel)

	if app.disk.idleSort[ui.PrimaryPanel].timer == nil {
		t.Fatal("only the navigated panel should reset idle-sort debounce")
	}
	app.invalidateIdleDiskSortPanel(ui.PrimaryPanel)
}

func TestHandlePanelDirChangedLeftClearsIdleTimerOnChdir(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "f.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	leftRoot := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String())

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }
	app.setDiskUsageShown(true)
	app.setDiskUsageScanScope(leftRoot, []string{filepath.Clean(alpha)})

	app.disk.idleNavPath[ui.PrimaryPanel] = leftRoot
	app.armIdleDiskSortTimer(ui.PrimaryPanel)

	vr := app.panelViewportRows(ui.PrimaryPanel)
	if err := left.NavigateTo(filepath.Clean(alpha), "", vr); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load lands before handlePanelDirChanged checks Path
	app.handlePanelDirChanged(ui.PrimaryPanel)

	if app.disk.idleSort[ui.PrimaryPanel].timer != nil {
		t.Fatal("idle timer should clear when panel cwd changes")
	}
}

func TestDiskIdleSortActivatesAfterScanWhenListingCached(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false

	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !left.ListingFullyDiskCached() {
		t.Fatalf("listing not fully cached after scan busy=%v", app.diskUsageScanBusy())
	}

	ep := app.disk.idleSort[ui.PrimaryPanel].epoch
	app.applyIdleDiskSort(ui.PrimaryPanel, ep)

	if !left.IdleDiskTotalsSort {
		t.Fatalf("IdleDiskTotalsSort still false epoch=%d busy=%v", ep, app.diskUsageScanBusy())
	}
}

// Regression: a scan dominated by many small top-level files posts one EventSubtreeIndexed per
// file, which can queue up faster than pollDiskUsageUpdates drains them. Each drained event used
// to trigger its own O(panel entries) ListingFullyDiskCached recheck, so a backlog of N events
// cost O(N * panel entries) in a single pollDiskUsageUpdates call — on top of the legitimate,
// one-time O(panel entries) recheck(s) and O(entries*log(entries)) sort that activating idle-sort
// itself requires. The recheck must run at most once per call regardless of how many events were
// queued, so total DiskSorter calls must stay near that legitimate one-time cost, not scale with
// the number of queued events.
func TestPollDiskUsageUpdatesRechecksIdleSortOnceRegardlessOfEventBacklog(t *testing.T) {
	root := t.TempDir()
	const numFiles = 30
	for i := range numFiles {
		name := fmt.Sprintf("file-%02d.dat", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.IdleDiskTotalsSort = false

	var diskSorterCalls atomic.Int64
	realSorter := left.DiskSorter
	left.DiskSorter = func(abs string) (int64, bool) {
		diskSorterCalls.Add(1)
		return realSorter(abs)
	}

	app.startDiskUsageScanForPanel(ui.PrimaryPanel)

	// Let the scan finish and queue all its EventSubtreeIndexed events without draining them, so
	// they back up in the channel exactly like a burst of many small files completing faster than
	// the UI goroutine polls.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && app.disk.engine.DiskScanBusy() {
		time.Sleep(2 * time.Millisecond)
	}
	if app.disk.engine.DiskScanBusy() {
		t.Fatal("scan did not finish before deadline")
	}

	diskSorterCalls.Store(0)
	app.pollDiskUsageUpdates()

	// One event-triggered recheck pass, one immediate reconcile-triggered recheck pass, and one
	// O(n*log n) sort once idle-sort activates: measured ~5x numFiles, so 8x leaves headroom
	// without hiding a regression. The old per-event bug reran a full recheck pass per queued
	// event (>= numFiles events for this scan shape), which would blow well past this bound.
	if got, want := diskSorterCalls.Load(), int64(8*numFiles); got > want {
		t.Fatalf("DiskSorter called %d times by one pollDiskUsageUpdates after a %d-file scan, want <= %d; this indicates the per-event recheck regression (cost scaling with queued events, not with panel entries)", got, numFiles, want)
	}
	if !left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should have activated once the listing was fully cached")
	}
}

func TestHandlePanelDirChangedAppliesDiskSortWhenUsageSortEnabledWithoutActivatedLatch(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = false // must not deadlock idle disk ordering
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }
	app.setDiskUsageShown(true)
	app.setDiskUsageScanScope(left.PathString(), []string{filepath.Join(left.PathString(), "x")})

	app.handlePanelDirChanged(ui.PrimaryPanel)

	if !left.IdleDiskTotalsSort {
		t.Fatal("expected IdleDiskTotalsSort when DiskUsageIdleSizeSort is on and listing is fully cached")
	}
}

// Regression: invert-selection ("*") drives selection-size scans that fill the disk cache.
// Idle size-sort must stay off until the user runs disk-usage analysis (DiskUsageShown).
func TestInvertSelectionDoesNotActivateIdleDiskSort(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"maple", "cedar", "birch"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "leaf.dat"))
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = app.disk.engine.Size

	if app.model.DiskUsageShown {
		t.Fatal("DiskUsageShown should be false before any user-initiated scan")
	}

	namesBefore := make([]string, len(left.Entries))
	for i, e := range left.Entries {
		namesBefore[i] = e.Name
	}

	app.dispatch(keymap.ActionPanelInvertSelection)
	app.reconcileAfterEvent()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		// reconcileSelectionSizeScans now computes what needs scanning on a debounced
		// background goroutine and posts the result as an interrupt event; drain it so
		// StartScanFromListing actually gets called before checking scan completion.
		for screen.HasPendingEvent() {
			if ev, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				app.handleInterruptPayload(ev.Data())
			}
		}
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !left.ListingFullyDiskCached() {
		t.Fatalf("expected selection-size scan to fully cache listing busy=%v", app.diskUsageScanBusy())
	}

	// Drain any armed idle-sort timer as if the delay elapsed.
	ep := app.disk.idleSort[ui.PrimaryPanel].epoch
	app.applyIdleDiskSort(ui.PrimaryPanel, ep)
	app.handlePanelDirChanged(ui.PrimaryPanel)
	app.deferDiskIdleSortOnUserActivity()
	if timer := app.disk.idleSort[ui.PrimaryPanel].timer; timer != nil {
		timer.Stop()
		app.disk.idleSort[ui.PrimaryPanel].timer = nil
		app.applyIdleDiskSort(ui.PrimaryPanel, app.disk.idleSort[ui.PrimaryPanel].epoch)
	}

	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort must stay false: selection-size cache is not disk-usage analysis")
	}
	if app.model.DiskUsageShown {
		t.Fatal("DiskUsageShown must stay false after selection-size scans alone")
	}
	for i, e := range left.Entries {
		if e.Name != namesBefore[i] {
			t.Fatalf("sort order changed after invert-selection: got %q at %d, want %q", e.Name, i, namesBefore[i])
		}
	}
}

func TestNavigateOutsideDiskUsageScanScopeClearsIdleSort(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	other := filepath.Join(root, "other")
	for _, p := range []string{scanned, other} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(scanned, "in-scan.dat"))
	writeFile(t, filepath.Join(other, "out-scan.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	app.setDiskUsageScanScope(root, []string{scanned})
	app.setDiskUsageShown(true)
	app.model.DiskUsagePanelID = ui.PrimaryPanel

	left.DiskSorter = app.disk.engine.Size
	left.IdleDiskTotalsSort = true
	left.ApplySort()

	vr := app.panelViewportRows(ui.PrimaryPanel)
	if err := left.NavigateTo(other, "", vr); err != nil {
		t.Fatalf("NavigateTo other: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load lands before handlePanelDirChanged checks Path
	app.handlePanelDirChanged(ui.PrimaryPanel)

	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be false outside scan scope")
	}
	if app.listingInDiskUsageScanScope(other) {
		t.Fatal("other directory should be outside scan scope")
	}

	if err := left.NavigateTo(scanned, "", vr); err != nil {
		t.Fatalf("NavigateTo scanned: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load lands before handlePanelDirChanged checks Path
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	app.handlePanelDirChanged(ui.PrimaryPanel)
	if !left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should apply inside scan scope when fully cached")
	}
	if !app.listingInDiskUsageScanScope(scanned) {
		t.Fatal("scanned subtree should be in scope")
	}
}

func TestDiskUsageScanScopeAppliesOnEitherPanel(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	if err := os.Mkdir(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scanned, "a.dat"))
	writeFile(t, filepath.Join(scanned, "b.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	right := app.panelByID(ui.SecondaryPanel)
	right.Sort.DiskUsageIdleSizeSort = true
	vrRight := app.panelViewportRows(ui.SecondaryPanel)
	if err := right.NavigateTo(scanned, "", vrRight); err != nil {
		t.Fatalf("NavigateTo scanned on right: %v", err)
	}
	app.handlePanelDirChanged(ui.SecondaryPanel)

	if !app.listingInDiskUsageScanScope(scanned) {
		t.Fatal("scanned path should be in global scan scope")
	}
	if !right.IdleDiskTotalsSort {
		t.Fatal("right panel should idle-sort by disk totals inside scan scope")
	}
	if !app.model.DiskUsageShown ||
		!panel.ListingPathInDiskUsageScanScope(right.PathString(), app.model.DiskUsageScanOrigin, app.model.DiskUsageScanRoots) {
		t.Fatal("right panel listing should be eligible for disk usage display inside scan scope")
	}
}

func TestApplyIdleDiskSortIgnoresStaleEpoch(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }

	stale := app.disk.idleSort[ui.PrimaryPanel].epoch
	app.invalidateIdleDiskSortPanel(ui.PrimaryPanel)
	app.applyIdleDiskSort(ui.PrimaryPanel, stale)

	if left.IdleDiskTotalsSort {
		t.Fatal("stale epoch must not apply idle disk sort")
	}
}

// Regression: disk-usage sort must re-activate when navigating back to a previously-scanned
// directory after a second scan from a child replaced the global scope.
func TestDiskSortRestoredAfterNewScanReplacesScope(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpine")
	beta := filepath.Join(root, "birch")
	for _, d := range []string{alpha, beta} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(alpha, "leaf.dat"))
	writeFile(t, filepath.Join(beta, "bark.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true

	// First scan from root. Wait for it to finish and sort to activate.
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	app.handlePanelDirChanged(ui.PrimaryPanel)
	if !left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be true at root after first scan")
	}

	// Navigate into a child directory and start a second scan there, replacing the scope.
	vr := app.panelViewportRows(ui.PrimaryPanel)
	if err := left.NavigateTo(alpha, "", vr); err != nil {
		t.Fatalf("NavigateTo alpha: %v", err)
	}
	applyNextInterruptEvent(t, app, screen) // async load lands before the scan reads left.Path
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline = time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	// Scope now points at alpha, not root.
	if app.listingInDiskUsageScanScope(root) {
		t.Fatal("root should be outside the new scan scope (scope was replaced)")
	}

	// Navigate back to root. Sort must re-activate because root's entries are still
	// fully cached from the first scan, even though the scope changed.
	if err := left.Parent(vr); err != nil {
		t.Fatalf("Parent: %v", err)
	}
	app.handlePanelDirChanged(ui.PrimaryPanel)

	if !left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be true again at root: cached data is valid regardless of current scan scope")
	}
}

func TestClearAllDiskUsageData(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	if err := os.Mkdir(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scanned, "a.dat"))
	writeFile(t, filepath.Join(scanned, "b.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !app.model.DiskUsageShown {
		t.Fatal("expected disk usage to be shown after scan")
	}
	if _, ok := app.disk.engine.Size(scanned); !ok {
		t.Fatal("expected cached size for scanned directory")
	}

	left.IdleDiskTotalsSort = true
	app.clearAllDiskUsageData()

	if app.model.DiskUsageShown {
		t.Fatal("DiskUsageShown should be false after clear")
	}
	if app.model.DiskUsageScanOrigin != "" || len(app.model.DiskUsageScanRoots) > 0 {
		t.Fatal("scan scope should be cleared")
	}
	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be false after clear")
	}
	if _, ok := app.disk.engine.Size(scanned); ok {
		t.Fatal("engine cache should be empty after clear")
	}
	if app.model.Message != "Disk usage data cleared" {
		t.Fatalf("message = %q, want Disk usage data cleared", app.model.Message)
	}
}

func TestSortDialogHandleKeyAltDTogglesDirectoriesFirstWithoutStartingDiskUsageScan(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	if err := os.Mkdir(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scanned, "a.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	// ListingFullyDiskCached reflects the engine's cache map directly, so the loop above can
	// break before the trailing EventJobFinished (which drives the "scan finished" toast) has
	// been drained off the events channel. Drain it now so app.model.Message settles before
	// later assertions check it.
	app.pollDiskUsageUpdates()
	if !app.model.DiskUsageShown {
		t.Fatal("expected disk usage to be shown after scan")
	}

	left.Sort.DirectoriesFirst = false
	app.openSortDialog()
	st := &app.model.SortDialog
	if !st.Open {
		t.Fatal("sort dialog should be open")
	}
	if st.DirectoriesFirst {
		t.Fatal("sort dialog should mirror panel directories-first=false")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt)); quit {
		t.Fatal("handleKey quit on Alt+D")
	}
	if !st.DirectoriesFirst {
		t.Fatal("Alt+D should toggle directories-first in sort dialog")
	}
	if st.Focus != 6 {
		t.Fatalf("focus = %d, want 6 (directories-first row)", st.Focus)
	}
	if !app.model.DiskUsageShown {
		t.Fatal("disk usage should remain shown after Sort Alt+D")
	}
	if strings.HasPrefix(app.model.Message, "Disk usage scan started") {
		t.Fatal("Sort Alt+D must not trigger panel.disk-usage-scan")
	}
}
