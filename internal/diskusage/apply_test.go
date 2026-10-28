package diskusage

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fschange"
)

// applyWait applies each batch in order and waits until the engine has applied them all.
func applyWait(t *testing.T, e *Engine, batches ...[]fschange.Change) {
	t.Helper()
	for _, b := range batches {
		e.Apply(b)
	}
	waitUntil(t, e.applyIdle, 5*time.Second, "apply did not settle")
}

func removed(p string, isDir bool) []fschange.Change {
	return []fschange.Change{{Kind: fschange.Removed, Path: p, IsDir: isDir}}
}

func TestApplyRemovedFileShrinksAncestors(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 1000, "/srv/lantern": 600, "/srv/lantern/kettle.txt": 200, "/srv/meadow": 400})
	applyWait(t, e, removed("/srv/lantern/kettle.txt", false))
	wantSizes(t, e, map[string]int64{"/srv": 800, "/srv/lantern": 400, "/srv/lantern/kettle.txt": -1, "/srv/meadow": 400})
}

func renamed(from, to string, isDir bool) fschange.Change {
	return fschange.Change{Kind: fschange.Renamed, From: from, Path: to, IsDir: isDir}
}

func TestApplyRemovedDirDropsSubtreeOnce(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 1000, "/srv/lantern": 600, "/srv/lantern/orchard": 200, "/srv/meadow": 400, "/srv/lanterns": 0})
	// The nested root is inside the outer one's size: subtracted once.
	applyWait(t, e, []fschange.Change{{Kind: fschange.Removed, Path: "/srv/lantern", IsDir: true}, {Kind: fschange.Removed, Path: "/srv/lantern/orchard", IsDir: true}})
	wantSizes(t, e, map[string]int64{"/srv": 400, "/srv/meadow": 400, "/srv/lantern": -1, "/srv/lantern/orchard": -1, "/srv/lanterns": 0})
}

func TestApplyRemovedStopsAtExcludedAndClamps(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 50, "/srv/mnt": 0, "/srv/mnt/lantern": 300, "/srv/mnt/lantern/orchard": 100, "/srv/meadow": 300})
	e.MarkExcluded("/srv/mnt")
	applyWait(t, e, removed("/srv/mnt/lantern/orchard", true), removed("/srv/meadow", true))
	wantSizes(t, e, map[string]int64{"/srv/mnt/lantern": 200, "/srv/mnt": 0, "/srv": 0})
}

func TestApplyRenamedMovesAncestorTotalsAndReplacesTarget(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{
		"/srv": 1000, "/srv/lantern": 600, "/srv/lantern/orchard": 600, "/srv/lantern/orchard/pear": 600,
		"/srv/harbor": 300, "/srv/harbor/orchard": 100, "/srv/harbor/orchard/fig": 100,
	})
	// orchard replaces harbor/orchard: harbor gains 600, loses the replaced 100; /srv nets -100.
	applyWait(t, e, []fschange.Change{renamed("/srv/lantern/orchard", "/srv/harbor/orchard", true)})
	wantSizes(t, e, map[string]int64{
		"/srv": 900, "/srv/lantern": 0, "/srv/harbor": 800, "/srv/harbor/orchard": 600,
		"/srv/harbor/orchard/pear": 600, "/srv/harbor/orchard/fig": -1, "/srv/lantern/orchard": -1,
	})
}

func TestApplyRenameSwapBatchIsSimultaneous(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv/a": 100, "/srv/a/lantern": 100, "/srv/b": 300, "/srv/b/meadow": 300})
	applyWait(t, e, []fschange.Change{renamed("/srv/a", "/srv/b", true), renamed("/srv/b", "/srv/a", true)})
	wantSizes(t, e, map[string]int64{"/srv/a": 300, "/srv/a/meadow": 300, "/srv/b": 100, "/srv/b/lantern": 100})
}

func TestApplyKeepsOrderOfOverlappingBatches(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 900, "/srv/a": 100, "/srv/a/fig": 100, "/srv/b": 300, "/srv/b/pear": 300, "/srv/c": 500})
	// Remove a, then rename b into a's old name, then c into b's: applied as one pass, each
	// change must still see the state the previous one left.
	applyWait(t, e, removed("/srv/a", true), []fschange.Change{renamed("/srv/b", "/srv/a", true)}, []fschange.Change{renamed("/srv/c", "/srv/b", false)})
	wantSizes(t, e, map[string]int64{"/srv": 800, "/srv/a": 300, "/srv/a/pear": 300, "/srv/a/fig": -1, "/srv/b": 500, "/srv/c": -1})
}

func TestApplyCoalescesDisjointDirRenamesIntoOneSweep(t *testing.T) {
	t.Parallel()
	e := New()
	names := []string{"amber", "basil", "cobalt", "dune", "ember", "fennel"}
	sizes := map[string]int64{"/srv/src": 600, "/srv/dst": 0}
	for _, n := range names {
		sizes["/srv/src/"+n] = 100
		sizes["/srv/src/"+n+"/leaf"] = 100
	}
	seedSizes(e, sizes)
	var batches [][]fschange.Change
	for _, n := range names {
		batches = append(batches, []fschange.Change{renamed("/srv/src/"+n, "/srv/dst/"+n, true)})
	}
	e.mu.Lock() // hold the pass back so every batch lands in one drain
	for _, b := range batches {
		e.Apply(b)
	}
	e.mu.Unlock()
	waitUntil(t, e.applyIdle, 5*time.Second, "apply did not settle")
	if n := e.sweeps.Load(); n > 2 {
		t.Fatalf("sweeps = %d, want at most 2 (the first batch may drain alone)", n)
	}
	want := map[string]int64{"/srv/src": 0, "/srv/dst": 600}
	for _, n := range names {
		want["/srv/dst/"+n+"/leaf"] = 100
		want["/srv/src/"+n] = -1
	}
	wantSizes(t, e, want)
}

func TestApplyFileOnlyChangesDoNotSweep(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 300, "/srv/kettle.txt": 100, "/srv/teapot.txt": 200})
	applyWait(t, e, []fschange.Change{renamed("/srv/kettle.txt", "/srv/teapot.txt", false)})
	if n := e.sweeps.Load(); n != 0 {
		t.Fatalf("sweeps = %d, want 0", n)
	}
	wantSizes(t, e, map[string]int64{"/srv": 100, "/srv/teapot.txt": 100, "/srv/kettle.txt": -1})
}

func created(p string, bytes int64, isDir bool) []fschange.Change {
	return []fschange.Change{{Kind: fschange.Created, Path: p, Bytes: bytes, IsDir: isDir}}
}

func TestApplyCreatedFileGrowsAncestorsAndIsRemovable(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 100, "/srv/lantern": 100, "/srv/lantern/fig": 100})
	applyWait(t, e, created("/srv/lantern/kettle.txt", 50, false))
	for p, want := range map[string]int64{"/srv": 150, "/srv/lantern": 150, "/srv/lantern/kettle.txt": 50} {
		if n, ok := e.Size(p); !ok || n != want {
			t.Errorf("%s = %d, %v; want %d", p, n, ok, want)
		}
	}
	if fc, _ := e.FileCount("/srv"); fc != 11 {
		t.Fatalf("/srv file count = %d, want 11", fc)
	}
	// An undo of the copy removes it again.
	applyWait(t, e, removed("/srv/lantern/kettle.txt", false))
	wantSizes(t, e, map[string]int64{"/srv": 100, "/srv/lantern": 100})
}

func TestApplyCreatedOverwriteNetsTheDifferenceAndRepeatsIdempotently(t *testing.T) {
	t.Parallel()
	e := New()
	e.SetSize("/srv", 2, 300)
	e.SetSize("/srv/kettle.txt", 1, 100)
	e.SetSize("/srv/teapot.txt", 1, 200)
	// A walk that already saw the new file and then the event: still counted once.
	applyWait(t, e, created("/srv/kettle.txt", 40, false), created("/srv/kettle.txt", 40, false))
	if n, _ := e.Size("/srv"); n != 240 {
		t.Fatalf("/srv = %d, want 240", n)
	}
	if fc, _ := e.FileCount("/srv"); fc != 2 {
		t.Fatalf("/srv file count = %d, want 2", fc)
	}
}

func TestApplyCreatedFillsMissingIntermediateDirs(t *testing.T) {
	t.Parallel()
	e := New()
	e.SetSize("/srv", 0, 0)
	// mkdir -p made a/b without events; the file's arrival materialises them.
	applyWait(t, e, created("/srv/a/b/kettle.txt", 70, false), created("/srv/a/b/teapot.txt", 30, false))
	for p, want := range map[string]int64{"/srv": 100, "/srv/a": 100, "/srv/a/b": 100, "/srv/a/b/kettle.txt": 70} {
		if n, ok := e.Size(p); !ok || n != want {
			t.Errorf("%s = %d, %v; want %d", p, n, ok, want)
		}
	}
	if fc, _ := e.FileCount("/srv/a"); fc != 2 {
		t.Fatalf("/srv/a file count = %d, want 2", fc)
	}
	// Removing the materialised dir subtracts it all.
	applyWait(t, e, removed("/srv/a", true))
	if n, _ := e.Size("/srv"); n != 0 {
		t.Fatalf("/srv = %d after removing a, want 0", n)
	}
}

func TestApplyCreatedDirAndUnscannedOrExcludedParents(t *testing.T) {
	t.Parallel()
	e := New()
	e.SetSize("/srv", 0, 0)
	e.SetSize("/srv/mnt", 0, 0)
	e.MarkExcluded("/srv/mnt")
	applyWait(t, e,
		created("/srv/lantern", 0, true),
		created("/srv/mnt/x/kettle.txt", 10, false), // under an excluded dir: never counted
		created("/elsewhere/kettle.txt", 10, false), // nothing cached above
	)
	if n, ok := e.Size("/srv/lantern"); !ok || n != 0 {
		t.Fatalf("created dir = %d, %v; want cached 0", n, ok)
	}
	for _, p := range []string{"/srv/mnt/x", "/srv/mnt/x/kettle.txt", "/elsewhere/kettle.txt", "/elsewhere"} {
		if _, ok := e.Size(p); ok {
			t.Errorf("%s cached", p)
		}
	}
	if n, _ := e.Size("/srv"); n != 0 {
		t.Fatalf("/srv = %d, want 0", n)
	}
}

func TestApplyRenamedFromUnscannedCountsTheArrival(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 300, "/srv/kettle.txt": 100, "/srv/basket": 200, "/srv/basket/fig": 200})
	applyWait(t, e,
		// A file moved in from an unscanned tree, overwriting kettle.txt: its size is known.
		[]fschange.Change{{Kind: fschange.Renamed, From: "/home/kettle.txt", Path: "/srv/kettle.txt", Bytes: 40}},
		// A directory moved in over basket: its size is unknown, so basket's is dropped.
		[]fschange.Change{renamed("/home/basket", "/srv/basket", true)},
	)
	if n, _ := e.Size("/srv/kettle.txt"); n != 40 {
		t.Fatalf("kettle.txt = %d, want 40", n)
	}
	if n, _ := e.Size("/srv"); n != 40 {
		t.Fatalf("/srv = %d, want 40 (kettle replaced, basket gone until rescanned)", n)
	}
	for _, p := range []string{"/srv/basket", "/srv/basket/fig"} {
		if _, ok := e.Size(p); ok {
			t.Errorf("%s still cached", p)
		}
	}
}

func writeSized(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scannedEngine(t *testing.T, root string) *Engine {
	t.Helper()
	e := New()
	e.StartScanFromListing([]string{root}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { _, ok := e.Size(root); return ok && !e.DiskScanBusy() }, 5*time.Second, "scan did not finish")
	return e
}

func TestApplyDirtyRescansPathAndMovesAncestors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "dest/basket/fig.txt"), 100)
	writeSized(t, filepath.Join(root, "meadow.txt"), 10)
	e := scannedEngine(t, root)
	// An archive tool adds a file and grows another; twice reported dirty, counted once.
	writeSized(t, filepath.Join(root, "dest/basket/pear.txt"), 50)
	writeSized(t, filepath.Join(root, "dest/basket/fig.txt"), 300)
	dirty := []fschange.Change{{Kind: fschange.Dirty, Path: filepath.Join(root, "dest/basket"), IsDir: true}}
	e.Apply(dirty)
	e.Apply(dirty)
	waitUntil(t, func() bool {
		n, _ := e.Size(root)
		return n == 360 && e.applyIdle() && !e.DiskScanBusy()
	}, 5*time.Second, "root total did not reach 360")
	if n, _ := e.Size(filepath.Join(root, "dest")); n != 350 {
		t.Fatalf("dest = %d, want 350", n)
	}
	if fc, _ := e.FileCount(root); fc != 3 {
		t.Fatalf("root file count = %d, want 3", fc)
	}
}

func TestApplyDirtyUnderNothingScannedQueuesNothing(t *testing.T) {
	t.Parallel()
	e := New()
	applyWait(t, e, []fschange.Change{{Kind: fschange.Dirty, Path: "/nowhere/basket", IsDir: true}})
	if e.DiskScanBusy() {
		t.Fatal("a refresh scan was queued for an unscanned path")
	}
}

func TestApplyDuringWalkRewalksTheRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "lantern/fig.txt"), 100)
	e := New()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	e.walkReadDir = func(p string) ([]os.FileInfo, error) {
		if p == filepath.Join(root, "lantern") {
			once.Do(func() {
				close(entered)
				<-release
			})
		}
		return ReadDirInfos(p)
	}
	e.StartScanFromListing([]string{root}, nil, 0, ListingVolumeGate{})
	<-entered
	// The walk has read root but not lantern yet... then lantern is renamed away and reported.
	if err := os.Rename(filepath.Join(root, "lantern"), filepath.Join(root, "harbor")); err != nil {
		t.Fatal(err)
	}
	e.Apply([]fschange.Change{renamed(filepath.Join(root, "lantern"), filepath.Join(root, "harbor"), true)})
	waitUntil(t, e.applyIdle, 5*time.Second, "apply did not settle")
	close(release)
	waitUntil(t, func() bool { _, ok := e.Size(root); return ok && !e.DiskScanBusy() }, 5*time.Second, "scan did not finish")
	if _, ok := e.Size(filepath.Join(root, "lantern")); ok {
		t.Fatal("stale walk merged lantern back")
	}
	if n, _ := e.Size(filepath.Join(root, "harbor/fig.txt")); n != 100 {
		t.Fatalf("harbor/fig.txt = %d, want 100", n)
	}
}

func TestApplyDirtySettledByWhicheverWalkReachesItFirst(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSized(t, filepath.Join(root, "dest/basket/fig.txt"), 100)
	e := scannedEngine(t, root)
	writeSized(t, filepath.Join(root, "dest/basket/pear.txt"), 50)
	basket := filepath.Join(root, "dest/basket")
	applyWait(t, e, []fschange.Change{{Kind: fschange.Dirty, Path: basket, IsDir: true}})
	// A listing scan of the dirty path, queued after the refresh, runs before it.
	e.StartScanFromListing([]string{basket}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { return !e.DiskScanBusy() }, 5*time.Second, "scans did not finish")
	for p, want := range map[string]int64{root: 150, filepath.Join(root, "dest"): 150, basket: 150} {
		if n, _ := e.Size(p); n != want {
			t.Errorf("%s = %d, want %d", p, n, want)
		}
	}
}

func TestApplySwapBatchWithAnArrivalUsesPreBatchSizes(t *testing.T) {
	t.Parallel()
	e := New()
	seedSizes(e, map[string]int64{"/srv": 300, "/srv/b.txt": 300})
	// b.txt moves to a.txt while a file from an unscanned place takes b.txt's name.
	applyWait(t, e, []fschange.Change{
		{Kind: fschange.Renamed, From: "/home/a.txt", Path: "/srv/b.txt", Bytes: 70},
		renamed("/srv/b.txt", "/srv/a.txt", false),
	})
	for p, want := range map[string]int64{"/srv": 370, "/srv/a.txt": 300, "/srv/b.txt": 70} {
		if n, _ := e.Size(p); n != want {
			t.Errorf("%s = %d, want %d", p, n, want)
		}
	}
}
