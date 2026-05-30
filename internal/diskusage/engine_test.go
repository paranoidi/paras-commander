package diskusage

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
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
