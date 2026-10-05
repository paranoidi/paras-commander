package diskusage

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fswalk"
)

func TestPathIsOrUnder(t *testing.T) {
	t.Parallel()
	if !pathIsOrUnder("/a/b", "/a") {
		t.Fatal("descendant")
	}
	if pathIsOrUnder("/ab", "/a") {
		t.Fatal("must not match sibling path prefix")
	}
	if !pathIsOrUnder("/a", "/a") {
		t.Fatal("same path")
	}
}

func TestPendingCoversQueuedSiblingsDescendantsAndLaterJobs(t *testing.T) {
	t.Parallel()

	unblock := make(chan struct{})
	e := New()
	e.runPlannerHook = func(_ uint64, _ []string, _ ShouldIgnoreFolder, _ int) {
		<-unblock
	}

	e.StartScanFromListing([]string{"/w/a", "/w/b"}, nil, 0, ListingVolumeGate{})

	waitUntil(t, func() bool { return e.PendingForPanel("/w/a", 0) }, 2*time.Second, "first root should schedule")

	if !e.PendingForPanel("/w/b", 0) {
		t.Fatal("sibling not yet walked should still tint")
	}
	if !e.PendingForPanel("/w/a/nested", 0) {
		t.Fatal("descendant of active root should tint")
	}
	if e.PendingForPanel("/z", 0) {
		t.Fatal("unrelated path should not tint")
	}

	e.StartScanFromListing([]string{"/queued"}, nil, 0, ListingVolumeGate{})
	if !e.PendingForPanel("/queued", 0) {
		t.Fatal("queued job roots should tint")
	}
	if !e.PendingForPanel("/queued/sub", 0) {
		t.Fatal("descendant of queued root should tint")
	}

	close(unblock)
}

func TestStartScanFromListingDoesNotBlockWhenScanInProgress(t *testing.T) {
	t.Parallel()

	unblock := make(chan struct{})
	started := make(chan struct{}, 1)

	e := New()
	e.runPlannerHook = func(_ uint64, _ []string, _ ShouldIgnoreFolder, _ int) {
		started <- struct{}{}
		<-unblock
	}

	e.StartScanFromListing([]string{"/first"}, nil, 0, ListingVolumeGate{})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not start first scan")
	}

	secondReturned := make(chan struct{})
	go func() {
		e.StartScanFromListing([]string{"/second"}, nil, 0, ListingVolumeGate{})
		close(secondReturned)
	}()

	select {
	case <-secondReturned:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second StartScanFromListing blocked the caller")
	}

	close(unblock)
}

func TestDiskScanBusyReflectsWorkerAndQueue(t *testing.T) {
	t.Parallel()

	e := New()
	if e.DiskScanBusy() {
		t.Fatal("new engine should be idle")
	}
	unblock := make(chan struct{})
	e.runPlannerHook = func(_ uint64, _ []string, _ ShouldIgnoreFolder, _ int) {
		<-unblock
	}
	e.StartScanFromListing([]string{"/a"}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { return e.DiskScanBusy() }, time.Second, "want busy while worker runs")
	close(unblock)
	waitUntil(t, func() bool { return !e.DiskScanBusy() }, time.Second, "want idle after job completes")
}

func TestScanQueuePrependsNewJobs(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var runs [][]string

	unblockFirst := make(chan struct{})

	e := New()
	e.runPlannerHook = func(_ uint64, childAbs []string, _ ShouldIgnoreFolder, _ int) {
		mu.Lock()
		runs = append(runs, append([]string(nil), childAbs...))
		first := len(runs) == 1 && len(childAbs) > 0 && childAbs[0] == "/a"
		mu.Unlock()

		if first {
			<-unblockFirst
		}
	}

	e.StartScanFromListing([]string{"/a"}, nil, 0, ListingVolumeGate{})

	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(runs) >= 1
	}, 2*time.Second, "first scan did not start")

	e.StartScanFromListing([]string{"/b"}, nil, 0, ListingVolumeGate{})
	e.StartScanFromListing([]string{"/c"}, nil, 0, ListingVolumeGate{})

	close(unblockFirst)

	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(runs) >= 3
	}, 2*time.Second, "expected 3 dequeue runs")

	mu.Lock()
	defer mu.Unlock()

	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(runs))
	}
	if runs[0][0] != "/a" || runs[1][0] != "/c" || runs[2][0] != "/b" {
		t.Fatalf("run order = %v, want [/a /c /b] (newer requests before older queued)", runs)
	}
}

func TestAbortClearsQueuedJobs(t *testing.T) {
	t.Parallel()

	unblock := make(chan struct{})
	var mu sync.Mutex
	var runs [][]string

	e := New()
	e.runPlannerHook = func(_ uint64, childAbs []string, _ ShouldIgnoreFolder, _ int) {
		mu.Lock()
		runs = append(runs, append([]string(nil), childAbs...))
		mu.Unlock()
		<-unblock
	}

	e.StartScanFromListing([]string{"/a"}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(runs) >= 1
	}, 2*time.Second, "first scan did not start")

	e.StartScanFromListing([]string{"/queued"}, nil, 0, ListingVolumeGate{})
	e.Abort()
	close(unblock)

	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(runs) >= 1
	}, 2*time.Second, "first scan should complete")

	mu.Lock()
	defer mu.Unlock()
	if len(runs) != 1 {
		t.Fatalf("after abort, runs = %d (%v), want only the in-flight /a", len(runs), runs)
	}
	if runs[0][0] != "/a" {
		t.Fatalf("got %v, want [/a]", runs[0])
	}
}

func TestInvalidateSubtreeRemovesRootAndDescendants(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	nested := filepath.Join(sub, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		filepath.Join(sub, "a.dat"),
		filepath.Join(nested, "b.dat"),
	} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(root, "other.dat")
	if err := os.WriteFile(other, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New()
	e.StartScanFromListing([]string{sub, other}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool {
		_, okSub := e.Size(sub)
		_, okOther := e.Size(other)
		return okSub && okOther
	}, 5*time.Second, "sizes not cached")

	e.InvalidateSubtree(sub)
	if _, ok := e.Size(sub); ok {
		t.Fatal("sub size should be gone after InvalidateSubtree")
	}
	if _, ok := e.Size(nested); ok {
		t.Fatal("nested size should be gone after InvalidateSubtree")
	}
	if _, ok := e.Size(other); !ok {
		t.Fatal("unrelated path should remain cached")
	}
}

func TestClearCacheRemovesSizes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := filepath.Join(root, "file.dat")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New()
	e.StartScanFromListing([]string{f}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool {
		_, ok := e.Size(f)
		return ok
	}, 5*time.Second, "file size not cached")

	e.ClearCache()
	if _, ok := e.Size(f); ok {
		t.Fatal("size should be gone after ClearCache")
	}
	if e.DiskScanBusy() {
		t.Fatal("ClearCache should abort busy scans")
	}
}

func TestCacheVersionIncrementsOnMutationOnly(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := filepath.Join(root, "file.dat")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New()
	v0 := e.CacheVersion()

	e.StartScanFromListing([]string{f}, nil, 0, ListingVolumeGate{})
	waitUntil(t, func() bool {
		_, ok := e.Size(f)
		return ok
	}, 5*time.Second, "file size not cached")

	v1 := e.CacheVersion()
	if v1 == v0 {
		t.Fatal("CacheVersion should increment after a cache mutation")
	}
	if got := e.CacheVersion(); got != v1 {
		t.Fatalf("CacheVersion changed without a mutation: got %d, want %d", got, v1)
	}

	e.ClearCache()
	if v2 := e.CacheVersion(); v2 == v1 {
		t.Fatal("CacheVersion should increment after ClearCache")
	}
}

func TestPendingForPanelOtherPanelNotTinted(t *testing.T) {
	t.Parallel()

	unblock := make(chan struct{})
	e := New()
	e.runPlannerHook = func(_ uint64, _ []string, _ ShouldIgnoreFolder, _ int) {
		<-unblock
	}

	e.StartScanFromListing([]string{"/w/a"}, nil, 0, ListingVolumeGate{})

	waitUntil(t, func() bool { return e.PendingForPanel("/w/a", 0) }, 2*time.Second, "source panel should tint")
	if e.PendingForPanel("/w/a", 1) {
		t.Fatal("other panel should not show disk-scan tint for sibling panel's scan job")
	}
	close(unblock)
}

func TestScanEmitsSubtreeIndexedPerRootAndJobFinished(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	d1 := filepath.Join(root, "d1")
	d2 := filepath.Join(root, "d2")
	if err := os.Mkdir(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d2, 0o755); err != nil {
		t.Fatal(err)
	}

	e := New()
	e.StartScanFromListing([]string{d1, d2}, nil, 0, ListingVolumeGate{})

	subtrees := 0
	finished := false
	timeout := time.After(15 * time.Second)
	for !finished || subtrees != 2 {
		select {
		case ev := <-e.Events():
			switch ev.Kind {
			case EventSubtreeIndexed:
				subtrees++
			case EventJobFinished:
				finished = true
			}
		case <-timeout:
			t.Fatalf("timeout subtrees=%d finished=%v busy=%v", subtrees, finished, e.DiskScanBusy())
		}
	}
}

func TestFileScanRootEmitsSubtreeIndexedAndCachesSize(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := filepath.Join(root, "x.dat")
	if err := os.WriteFile(f, []byte("abcd"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New()
	e.StartScanFromListing([]string{f}, nil, 0, ListingVolumeGate{})

	subtrees := 0
	finished := false
	timeout := time.After(15 * time.Second)
	for !finished || subtrees < 1 {
		select {
		case ev := <-e.Events():
			switch ev.Kind {
			case EventSubtreeIndexed:
				subtrees++
			case EventJobFinished:
				finished = true
			}
		case <-timeout:
			t.Fatalf("timeout subtrees=%d finished=%v", subtrees, finished)
		}
	}

	sz, ok := e.Size(f)
	if !ok || sz != 4 {
		t.Fatalf("Size(f) = %d ok=%v want 4 true", sz, ok)
	}
}

func waitUntil(t *testing.T, cond func() bool, d time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !cond() {
		t.Fatal(msg)
	}
}

type fakeDirInfo struct {
	name  string
	isDir bool
	size  int64
}

func (f fakeDirInfo) Name() string { return f.name }
func (f fakeDirInfo) Size() int64  { return f.size }
func (f fakeDirInfo) Mode() fs.FileMode {
	if f.isDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (f fakeDirInfo) ModTime() time.Time { return time.Time{} }
func (f fakeDirInfo) IsDir() bool        { return f.isDir }
func (f fakeDirInfo) Sys() any           { return nil }

// TestAbortCancelsActiveWalkAndNextRequestStarts proves Abort cancels WalkFolder instead of
// waiting for the stale subtree: a blocked child ReadDir is released, siblings are not
// descended into, post-abort results stay uncached, and a replacement scan starts.
func TestAbortCancelsActiveWalkAndNextRequestStarts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dirA := filepath.Join(root, "dirA")
	dirB := filepath.Join(root, "dirB")
	dirC := filepath.Join(root, "dirC")
	next := t.TempDir()
	children := map[string][]fs.FileInfo{
		filepath.Clean(root): {
			fakeDirInfo{name: "dirA", isDir: true},
			fakeDirInfo{name: "dirB", isDir: true},
			fakeDirInfo{name: "dirC", isDir: true},
		},
		filepath.Clean(next): {
			fakeDirInfo{name: "leaf.dat", isDir: false, size: 4},
		},
	}

	var mu sync.Mutex
	var reads []string
	enteredChild := make(chan struct{})
	blockChild := make(chan struct{})
	nextStarted := make(chan struct{})

	e := New()
	e.fsWalk = fswalk.Params{InitialWorkers: 1, MaxWorkers: 1, AdaptIntervalMS: 60000}
	e.walkReadDir = func(path string) ([]fs.FileInfo, error) {
		clean := filepath.Clean(path)
		mu.Lock()
		reads = append(reads, clean)
		isFirstChild := clean != filepath.Clean(root) && clean != filepath.Clean(next) && len(reads) == 2
		mu.Unlock()

		switch clean {
		case filepath.Clean(next):
			close(nextStarted)
		default:
			if isFirstChild {
				close(enteredChild)
				<-blockChild
			}
		}

		if entries, ok := children[clean]; ok {
			return entries, nil
		}
		return nil, nil
	}

	e.StartScanFromListing([]string{root}, nil, 0, ListingVolumeGate{})

	select {
	case <-enteredChild:
	case <-time.After(2 * time.Second):
		t.Fatal("walk did not reach blocking child ReadDir")
	}

	e.Abort()
	e.StartScanFromListing([]string{next}, nil, 0, ListingVolumeGate{})
	close(blockChild)

	select {
	case <-nextStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("next request did not start after abort released the stale walk")
	}

	waitUntil(t, func() bool { return !e.DiskScanBusy() }, 2*time.Second, "want idle after replacement scan")

	mu.Lock()
	got := append([]string(nil), reads...)
	mu.Unlock()
	firstChild := firstReadUnder(got, root)
	for _, p := range got {
		if p == dirA || p == dirB || p == dirC {
			if p != firstChild {
				t.Fatalf("stale walk kept descending after abort: reads=%v", got)
			}
		}
	}
	if _, ok := e.Size(root); ok {
		t.Fatal("aborted walk result must be rejected by generation check")
	}
}

func TestStartScanClassifiesIgnoredListingChildren(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skip := filepath.Join(root, "node_modules")
	keep := filepath.Join(root, "src")
	nestedSkip := filepath.Join(keep, "node_modules")
	for _, dir := range []string{skip, keep, nestedSkip} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(keep, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ignore := func(abs string) bool {
		return filepath.Base(abs) == "node_modules"
	}
	e := New()
	e.StartScanFromListing([]string{skip, keep}, ignore, 0, ListingVolumeGate{})
	waitUntil(t, func() bool { return !e.DiskScanBusy() }, 5*time.Second, "scan should finish")

	if !e.IsKnownExcluded(skip) {
		t.Fatal("listing child matching ignore must be classified excluded")
	}
	if !e.IsKnownExcluded(nestedSkip) {
		t.Fatal("nested ignore match must be classified during the walk")
	}
	if e.IsKnownExcluded(keep) {
		t.Fatal("unignored listing child must not be classified excluded")
	}
}

func firstReadUnder(reads []string, root string) string {
	root = filepath.Clean(root)
	for _, p := range reads {
		if p != root {
			return p
		}
	}
	return ""
}

// A FromRoot gate compares against the scan root's own device, so an explicit scan of a
// directory on another volume than the listing (e.g. a mount point) still descends into it.
func TestFromRootVolumeGateUsesScanRootDevice(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	child := filepath.Join(root, "lantern")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	got := make(chan bool, 1)
	e := New()
	e.runPlannerHook = func(_ uint64, _ []string, ignore ShouldIgnoreFolder, _ int) {
		got <- ignore(child)
	}

	// RefDev 0 never matches a real device: without FromRoot every child would be skipped.
	e.StartScanFromListing([]string{root}, nil, 0, ListingVolumeGate{Enabled: true, Valid: true, FromRoot: true})
	select {
	case ignored := <-got:
		if ignored {
			t.Fatal("child on the scan root's own volume must not be ignored")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("planner not invoked")
	}
}

// TestPriorityScanPreemptsAndRequeuesRest proves StartPriorityScan runs ahead of a walk in
// progress, then re-walks the preempted root in full and still walks its unstarted siblings.
func TestPriorityScanPreemptsAndRequeuesRest(t *testing.T) {
	t.Parallel()

	slow, sibling, prio := t.TempDir(), t.TempDir(), t.TempDir()
	sub := filepath.Join(slow, "sub")
	children := map[string][]fs.FileInfo{
		slow:    {fakeDirInfo{name: "sub", isDir: true}},
		sub:     {fakeDirInfo{name: "leaf.dat", size: 5}},
		sibling: {fakeDirInfo{name: "leaf.dat", size: 3}},
		prio:    {fakeDirInfo{name: "leaf.dat", size: 7}},
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var mu sync.Mutex
	var reads []string
	e := New()
	e.fsWalk = fswalk.Params{InitialWorkers: 1, MaxWorkers: 1, AdaptIntervalMS: 60000}
	e.walkReadDir = func(path string) ([]fs.FileInfo, error) {
		mu.Lock()
		reads = append(reads, filepath.Clean(path))
		mu.Unlock()
		if path == sub {
			once.Do(func() {
				close(entered)
				<-release
			})
		}
		return children[filepath.Clean(path)], nil
	}

	e.StartScanFromListing([]string{slow, sibling}, nil, 0, ListingVolumeGate{})
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("walk did not reach blocking ReadDir")
	}

	e.StartPriorityScan([]string{prio}, nil, 0, ListingVolumeGate{})
	close(release)

	waitUntil(t, func() bool { return !e.DiskScanBusy() }, 2*time.Second, "want idle")
	for path, want := range map[string]int64{prio: 7, slow: 5, sibling: 3} {
		if got, ok := e.Size(path); !ok || got != want {
			t.Fatalf("Size(%s) = %d ok=%v want %d", path, got, ok, want)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if slices.Index(reads, prio) > slices.Index(reads, sibling) {
		t.Fatalf("priority root walked after queued sibling: %v", reads)
	}

	var finished []bool
	for len(e.events) > 0 {
		if ev := <-e.events; ev.Kind == EventJobFinished {
			finished = append(finished, ev.QueueEmpty)
		}
	}
	if !slices.Equal(finished, []bool{false, true}) {
		t.Fatalf("JobFinished QueueEmpty = %v, want [false true] (priority job, then requeued rest)", finished)
	}
}
