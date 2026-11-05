package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// watchApp is an app over dir with the real directory watcher on and the periodic ticker off, so
// only the watcher can refresh a panel.
func watchApp(t *testing.T, dir string, tweak func(*Options)) (*App, tcell.SimulationScreen) {
	t.Helper()
	screen := newScreen(t, 100, 30)
	opts := testOptions(dir)
	opts.Config.Panels.WatchDirectories = true
	opts.Config.Panels.RefreshIntervalMS = 0
	if tweak != nil {
		tweak(&opts)
	}
	app := newTestApp(t, screen, opts)
	app.config.Shell.Persistent = false
	app.reconcileAfterEvent() // what the run loop does after every event: arms the watches
	return app, screen
}

// pumpWakes applies queued interrupts like the run loop, counting directory-watch wakes, until
// cond holds.
func pumpWakes(t *testing.T, app *App, screen tcell.SimulationScreen, wakes *int, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		for screen.HasPendingEvent() {
			if ie, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				if _, isWake := ie.Data().(dirWatchWakePayload); isWake {
					*wakes++
				}
				app.handleInterruptPayload(ie.Data())
				app.reconcileAfterEvent()
			}
		}
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for the directory watcher")
		}
		time.Sleep(time.Millisecond)
	}
}

// quiesce pumps until no wake has arrived for longer than a debounce window plus margin.
func quiesce(t *testing.T, app *App, screen tcell.SimulationScreen, wakes *int) {
	t.Helper()
	last, seen := time.Now(), *wakes
	pumpWakes(t, app, screen, wakes, func() bool {
		if *wakes != seen {
			seen, last = *wakes, time.Now()
		}
		return time.Since(last) > 500*time.Millisecond
	})
}

// armDirWatch waits until the (asynchronously added) watch on dir reports: it pokes the
// directory's own attributes, which changes no listing, until a wake arrives.
func armDirWatch(t *testing.T, app *App, screen tcell.SimulationScreen, wakes *int, dir string) {
	t.Helper()
	var last time.Time
	pumpWakes(t, app, screen, wakes, func() bool {
		if *wakes > 0 {
			return true
		}
		if time.Since(last) > 20*time.Millisecond {
			last = time.Now()
			_ = os.Chmod(dir, 0o755)
		}
		return false
	})
	quiesce(t, app, screen, wakes)
}

func realDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPanelWatchDirs(t *testing.T) {
	root := pathloc.MustParse("/srv/quiet-harbor")
	got := panelWatchDirs(root, []string{"/srv/quiet-harbor/alder", "/srv/quiet-harbor/birch"}, 10)
	want := []string{"/srv/quiet-harbor", "/srv/quiet-harbor/alder", "/srv/quiet-harbor/birch"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// The cap keeps the root and drops the rest.
	if got := panelWatchDirs(root, []string{"/srv/quiet-harbor/alder", "/srv/quiet-harbor/birch"}, 2); !slices.Equal(got, want[:2]) {
		t.Fatalf("capped: got %v, want %v", got, want[:2])
	}
	if got := panelWatchDirs(root, []string{"/srv/quiet-harbor/alder"}, 1); !slices.Equal(got, want[:1]) {
		t.Fatalf("cap 1: got %v, want only the root", got)
	}
	// Remote and direct fileindex:// panels are never watched; neither are non-local tree ids.
	if got := panelWatchDirs(pathloc.MustParse("sftp://user@example.invalid/srv"), nil, 10); got != nil {
		t.Fatalf("sftp root watched: %v", got)
	}
	idx, err := pathloc.FileIndex("/srv/quiet-harbor")
	if err != nil {
		t.Fatal(err)
	}
	if got := panelWatchDirs(idx, nil, 10); got != nil {
		t.Fatalf("fileindex:// root watched: %v", got)
	}
	if got := panelWatchDirs(root, []string{"sftp://user@example.invalid/srv/alder"}, 10); !slices.Equal(got, want[:1]) {
		t.Fatalf("non-local tree id watched: %v", got)
	}
}

func TestMatchChangedDirs(t *testing.T) {
	root := pathloc.MustParse("/srv/quiet-harbor")
	tree := []string{"/srv/quiet-harbor/alder", "/srv/quiet-harbor/alder/cedar"}
	ids, matched := matchChangedDirs(root, tree, []string{"/srv/quiet-harbor/alder/cedar", "/srv/elsewhere", "/srv/quiet-harbor", "/srv/quiet-harbor/alder/cedar"})
	if !slices.Equal(ids, []string{"/srv/quiet-harbor/alder/cedar"}) {
		t.Fatalf("ids = %v", ids)
	}
	if !slices.Equal(matched, []string{"/srv/quiet-harbor/alder/cedar", "/srv/quiet-harbor"}) {
		t.Fatalf("matched = %v", matched)
	}
	if ids, matched := matchChangedDirs(pathloc.MustParse("sftp://user@example.invalid/srv"), nil, []string{"/srv"}); ids != nil || matched != nil {
		t.Fatalf("remote panel matched %v %v", ids, matched)
	}
}

func TestScheduleReportsTransientSkipsAsRetry(t *testing.T) {
	app := testAppMinimal(t)
	p := app.panelByID(ui.PrimaryPanel)

	p.ListingPending = true
	if !app.schedulePanelListingRefresh(ui.PrimaryPanel) {
		t.Error("a pending load is transient")
	}
	p.ListingPending = false

	app.panelRefreshNotBefore[ui.PrimaryPanel].Store(time.Now().Add(time.Hour).UnixNano())
	if !app.schedulePanelListingRefresh(ui.PrimaryPanel) {
		t.Error("the slow-refresh deadline is transient")
	}
	app.panelRefreshNotBefore[ui.PrimaryPanel].Store(0)

	app.panelRefreshInFlight[ui.PrimaryPanel].Store(true)
	if !app.schedulePanelListingRefresh(ui.PrimaryPanel) {
		t.Error("a read in flight is transient")
	}
	app.panelRefreshInFlight[ui.PrimaryPanel].Store(false)

	if app.schedulePanelListingRefresh(ui.PrimaryPanel) {
		t.Error("a started refresh needs no retry")
	}
	pollUntilPanelRefreshIdle(t, app, ui.PrimaryPanel, 2*time.Second)

	app.panelRefreshNotBefore[ui.PrimaryPanel].Store(0)
	addPausedJobOn(t, app, p.PathString())
	if app.schedulePanelListingRefresh(ui.PrimaryPanel) {
		t.Error("a contended volume is not retried")
	}
}

func TestDirWatchUpdatesRowsOnExternalChanges(t *testing.T) {
	dir := realDir(t)
	writeFile(t, filepath.Join(dir, "amber.txt"))
	writeFile(t, filepath.Join(dir, "birch.txt"))
	app, screen := watchApp(t, dir, nil)
	p := app.activePanel()
	wakes := 0
	armDirWatch(t, app, screen, &wakes, dir)

	if err := os.Remove(filepath.Join(dir, "amber.txt")); err != nil {
		t.Fatal(err)
	}
	pumpWakes(t, app, screen, &wakes, func() bool { return !panelListsRow(p, "amber.txt") })

	writeFile(t, filepath.Join(dir, "cedar.txt"))
	pumpWakes(t, app, screen, &wakes, func() bool { return panelListsRow(p, "cedar.txt") })
	if !panelListsRow(p, "birch.txt") {
		t.Fatal("untouched row vanished")
	}
}

func TestDirWatchRetriesTransientSkip(t *testing.T) {
	dir := realDir(t)
	app, screen := watchApp(t, dir, nil)
	p := app.panelByID(ui.PrimaryPanel)
	wakes := 0
	armDirWatch(t, app, screen, &wakes, dir)

	before := wakes
	app.panelRefreshNotBefore[ui.PrimaryPanel].Store(time.Now().Add(time.Second).UnixNano())
	writeFile(t, filepath.Join(dir, "meadow.txt"))
	pumpWakes(t, app, screen, &wakes, func() bool { return panelListsRow(p, "meadow.txt") })
	if wakes-before < 2 {
		t.Fatalf("%d wakes, want the first skipped by the deadline and a retry", wakes-before)
	}
}

func TestDirWatchContendedVolumeStartsNoRefresh(t *testing.T) {
	dir := realDir(t)
	app, screen := watchApp(t, dir, nil)
	addPausedJobOn(t, app, dir)
	t.Cleanup(func() { app.jobState.CancelJob("contend") })
	fetches := 0
	swapFetchListingForPanelRefresh(t, func(ctx context.Context, snap panel.ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
		fetches++
		return panel.FetchListing(ctx, snap)
	})
	wakes := 0
	armDirWatch(t, app, screen, &wakes, dir)

	before := wakes
	writeFile(t, filepath.Join(dir, "pebble.txt"))
	pumpWakes(t, app, screen, &wakes, func() bool { return wakes > before })
	// A contended volume is dropped, not retried: no second wake follows.
	quiesce(t, app, screen, &wakes)
	if wakes != before+1 || fetches != 0 {
		t.Fatalf("wakes %d (want %d), fetches %d (want 0)", wakes-before, 1, fetches)
	}
	if panelListsRow(app.panelByID(ui.PrimaryPanel), "pebble.txt") {
		t.Fatal("a contended panel must wait for the job's own reload or the next tick")
	}
}

func treeShows(p *panel.State, path string) bool {
	for i := range p.VisibleEntryCount() {
		if e, _, ok := p.VisibleEntry(i); ok && e.Path == path {
			return true
		}
	}
	return false
}

func expandedCount(p *panel.State) int {
	n := 0
	for _, on := range p.TreeExpanded {
		if on {
			n++
		}
	}
	return n
}

// treeFixture builds n directories, each holding a seed file, and shows them all expanded.
func treeFixture(t *testing.T, app *App, screen tcell.SimulationScreen, n int) {
	t.Helper()
	p := app.panelByID(ui.PrimaryPanel)
	rows := app.panelViewportRows(ui.PrimaryPanel)
	if !p.SetListLayout(panel.ListLayoutTree, rows) {
		t.Fatal("tree layout refused")
	}
	if err := p.ExpandAllTreeShallow(rows); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return expandedCount(p) == n })
}

func TestDirWatchNestedTreeDirChangeIsFetched(t *testing.T) {
	const dirs = 30
	dir := realDir(t)
	for i := range dirs {
		sub := filepath.Join(dir, fmt.Sprintf("grove-%02d", i+1))
		mkdirs(t, sub)
		writeFile(t, filepath.Join(sub, "seed.txt"))
	}
	app, screen := watchApp(t, dir, nil)
	treeFixture(t, app, screen, dirs)
	p := app.panelByID(ui.PrimaryPanel)

	target := filepath.Join(dir, fmt.Sprintf("grove-%02d", dirs))
	if got := app.desiredWatchDirs(); got[0] != dir || !slices.Contains(got, target) || len(got) != dirs+1 {
		t.Fatalf("watch set = %v", got)
	}
	// The last directory is off-screen and not this tick's round-robin pick, so only the
	// watcher's naming it can get it re-listed.
	for _, r := range p.TreeRefreshPlan(app.panelViewportRows(ui.PrimaryPanel), 1) {
		if r.ID == target {
			t.Fatalf("%s already in the periodic plan", target)
		}
	}

	// Pulse files into the nested directory until one shows: the watch is added asynchronously,
	// and nothing but that watch can refresh this panel.
	wakes := 0
	var pulses []string
	var last time.Time
	pumpWakes(t, app, screen, &wakes, func() bool {
		if slices.ContainsFunc(pulses, func(path string) bool { return treeShows(p, path) }) {
			return true
		}
		if time.Since(last) > 40*time.Millisecond {
			last = time.Now()
			pulses = append(pulses, filepath.Join(target, fmt.Sprintf("pulse-%d.txt", len(pulses))))
			writeFile(t, pulses[len(pulses)-1])
		}
		return false
	})
	// The periodic plan alone would reach the directory only after a round of ticks (one
	// off-screen directory each), i.e. a dozen wakes here; the named directory shows at once.
	if wakes > 6 {
		t.Fatalf("nested change took %d wakes to show: the watcher's directory was not re-listed directly", wakes)
	}
}

func TestTreeExpandGenSeesCollapse(t *testing.T) {
	dir := realDir(t)
	for _, name := range []string{"alder", "birch"} {
		mkdirs(t, filepath.Join(dir, name))
		writeFile(t, filepath.Join(dir, name, "seed.txt"))
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	treeFixture(t, app, screen, 2)
	p := app.panelByID(ui.PrimaryPanel)
	gen, n := p.TreeExpandGen, len(p.TreeExpanded)
	p.Cursor = 0
	if err := p.ToggleTreeExpand(app.panelViewportRows(ui.PrimaryPanel)); err != nil {
		t.Fatal(err)
	}
	if p.TreeExpandGen == gen {
		t.Fatal("collapsing did not change the expand generation")
	}
	if len(p.TreeExpanded) != n {
		t.Fatalf("len(TreeExpanded) %d -> %d: the case the generation exists for did not occur", n, len(p.TreeExpanded))
	}
}

// indexedFixture is an app browsing a directory the index knows: harbor.txt at the top, and the
// subdirectories orchard and thicket with one indexed file each. Drift passes are off, so the
// tests set the directory states a pass would.
func indexedFixture(t *testing.T) (*App, tcell.SimulationScreen, string, time.Time) {
	t.Helper()
	dir := realDir(t)
	mkdirs(t, filepath.Join(dir, "orchard"), filepath.Join(dir, "thicket"))
	var batch []db.ScannedFile
	var harborMtime time.Time
	for i, rel := range []string{"harbor.txt", "orchard/pear.txt", "thicket/plum.txt"} {
		path := filepath.Join(dir, rel)
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
		// Whole-second mtimes survive the database round trip exactly, so a ghost row compares
		// equal to the file it replaces in everything but its overlay state.
		whole := time.Unix(1_700_000_000+int64(i)*60, 0)
		if err := os.Chtimes(path, whole, whole); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			harborMtime = fi.ModTime()
		}
		sum := string(rune('a'+i)) + "0000000000000000000000000000000"
		batch = append(batch, db.ScannedFile{Dir: filepath.Dir(path), Name: filepath.Base(path), Size: fi.Size(), Mtime: fi.ModTime(), MD5: sum, SHA256: sum + sum})
	}
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CommitScanBatch(batch); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	app, screen := watchApp(t, dir, func(o *Options) { o.Config.FileIndex.Database = dbPath })
	t.Cleanup(func() { _ = app.fileIndex.Close() })
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		app.panelByID(id).ScheduleIndexDrift = func(panel.IndexDriftRequest) bool { return false }
	}
	app.dispatch(keymap.ActionFileIndexBrowseWith)
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool {
		p := app.activePanel()
		st, _ := entryStatus(app, "orchard")
		return p.WithIndex && !p.ListingPending && st == localfs.IndexStatusIndexed
	})
	return app, screen, dir, harborMtime
}

func TestDesiredWatchDirsWatchBrowseWithIndexButNotFileIndexOrSFTP(t *testing.T) {
	app, _, dir, _ := indexedFixture(t)
	if !app.activePanel().WithIndex {
		t.Fatal("fixture is not browsing with the index")
	}
	other := realDir(t)
	sec := app.inactivePanel()
	sec.Path = pathloc.MustParse(other)
	if got := app.desiredWatchDirs(); !slices.Equal(got, []string{dir, other}) {
		t.Fatalf("local panels: %v", got)
	}
	sec.Path = pathloc.MustParse("sftp://user@example.invalid/srv")
	if got := app.desiredWatchDirs(); !slices.Equal(got, []string{dir}) {
		t.Fatalf("with an sftp panel: %v", got)
	}
	idx, err := pathloc.FileIndex(other)
	if err != nil {
		t.Fatal(err)
	}
	sec.Path = idx
	if got := app.desiredWatchDirs(); !slices.Equal(got, []string{dir}) {
		t.Fatalf("with a fileindex:// panel: %v", got)
	}
}

// A file leaving the disk keeps its indexed size and mtime as a ghost row, so only the overlay
// state tells the listings apart; the same going back.
func TestDirWatchTurnsIndexedFileIntoGhostAndBack(t *testing.T) {
	app, screen, dir, mtime := indexedFixture(t)
	if st, _ := entryStatus(app, "harbor.txt"); st != localfs.IndexStatusIndexed {
		t.Fatalf("harbor.txt = %q, want indexed", st)
	}
	wakes := 0
	armDirWatch(t, app, screen, &wakes, dir)

	path := filepath.Join(dir, "harbor.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	pumpWakes(t, app, screen, &wakes, func() bool { st, _ := entryStatus(app, "harbor.txt"); return st == localfs.IndexStatusMissing })

	if err := os.WriteFile(path, []byte("harbor.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	pumpWakes(t, app, screen, &wakes, func() bool { st, _ := entryStatus(app, "harbor.txt"); return st == localfs.IndexStatusIndexed })
}

// The drift pass rewrites indexed directories' states after a listing lands; an idle panel must
// not see those as a change against a fresh overlay and re-apply (re-running the pass) forever.
func TestIdleIndexPanelDriftStatesDoNotReapply(t *testing.T) {
	app, screen, _, _ := indexedFixture(t)
	p := app.panelByID(ui.PrimaryPanel)
	states := map[string]string{"orchard": localfs.IndexStatusDiffers, "thicket": localfs.IndexStatusUnchecked}
	for i, e := range p.Entries {
		if st, ok := states[e.Name]; ok {
			p.Entries[i].IndexStatus = st
		}
	}
	epoch := p.ListingEpoch

	snap := p.ListingRefreshSnapshot(p.Path, 5*time.Second)
	entries, loc, gitignoreActive, dotfilesHidden, err := panel.FetchListing(context.Background(), snap)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name == "orchard" && e.IndexStatus != localfs.IndexStatusIndexed {
			t.Fatalf("fresh overlay gave orchard %q; the check below would prove nothing", e.IndexStatus)
		}
	}
	payload := panelRefreshApplyPayload{PanelID: ui.PrimaryPanel, Path: loc, Entries: entries, GitignoreActive: gitignoreActive, DotfilesHiddenActive: dotfilesHidden, ListingEpoch: epoch}
	if app.applyPanelListingRefresh(payload) {
		t.Fatal("an unchanged listing was applied")
	}

	for range 3 {
		app.panelRefreshNotBefore[ui.PrimaryPanel].Store(0)
		app.schedulePanelListingRefresh(ui.PrimaryPanel)
		pollUntilPanelRefreshIdle(t, app, ui.PrimaryPanel, 5*time.Second)
	}
	drainInterruptEventsUntil(t, app, screen, 5*time.Second, func() bool { return true })
	if p.ListingEpoch != epoch {
		t.Fatalf("listing epoch %d -> %d: an idle panel was re-applied", epoch, p.ListingEpoch)
	}
	if st, _ := entryStatus(app, "orchard"); st != localfs.IndexStatusDiffers {
		t.Fatalf("orchard = %q, want its drift state kept", st)
	}
}

// A directory replaced under its path (rm -rf x; mkdir x) loses its inotify watch; the listing can
// look the same, so nothing but the wake itself can re-arm it.
func TestDirWatchRearmsRecreatedDirectory(t *testing.T) {
	dir := filepath.Join(realDir(t), "harbor")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	app, screen := watchApp(t, dir, nil)
	p := app.activePanel()
	wakes := 0
	armDirWatch(t, app, screen, &wakes, dir)

	before := wakes
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pumpWakes(t, app, screen, &wakes, func() bool { return wakes > before })
	// Let that wake's own listing finish first, or it could pick the row up by itself.
	quiesce(t, app, screen, &wakes)
	pollUntilPanelRefreshIdle(t, app, ui.PrimaryPanel, 5*time.Second)

	// The worker re-adds the watch asynchronously, so keep touching until a change gets through.
	// With no ticker running, only a re-armed watch can show this row.
	marker := filepath.Join(dir, "lantern.txt")
	pumpWakes(t, app, screen, &wakes, func() bool {
		writeFile(t, marker)
		return panelListsRow(p, "lantern.txt")
	})
}
