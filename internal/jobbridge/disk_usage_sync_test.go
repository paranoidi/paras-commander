package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// runDiskUsageJob runs job through TransferFunc with du as its fschange sink and returns the undo
// journal the job emitted.
func runDiskUsageJob(t *testing.T, du *diskusage.Engine, job *jobs.Job) []ops.UndoStep {
	t.Helper()
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, cfg.Compare, nil, nil, du)
	var journal []ops.UndoStep
	emit := func(ev jobs.Event) {
		if ev.Type == jobs.EventUndoJournal {
			journal = ev.Undo
		}
	}
	answer := func(jobs.BlockerRequest) jobs.BlockerAnswer {
		return jobs.BlockerAnswer{Decision: jobs.DecisionOverwrite}
	}
	if err := transfer(context.Background(), job, emit, answer); err != nil {
		t.Fatalf("transfer: %v", err)
	}
	return journal
}

// scanned returns an engine that has walked root.
func scanned(t *testing.T, root string) *diskusage.Engine {
	t.Helper()
	du := diskusage.New()
	du.StartScanFromListing([]string{root}, nil, 0, diskusage.ListingVolumeGate{})
	waitSize(t, du, root, -1)
	return du
}

// waitSize waits until p's cached size is want (any cached size when want is -1).
func waitSize(t *testing.T, du *diskusage.Engine, p string, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, ok := du.Size(p)
		if ok && (want < 0 || n == want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Size(%s) = %d, %v; want %d", p, n, ok, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func waitGone(t *testing.T, du *diskusage.Engine, p string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, ok := du.Size(p); !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s still cached", p)
		}
		time.Sleep(time.Millisecond)
	}
}

func put(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopyJobGrowsScannedDestinationAndUndoShrinksIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put(t, filepath.Join(dir, "src/basket/fig.txt"), 300)
	put(t, filepath.Join(dir, "src/basket/pear.txt"), 200)
	put(t, filepath.Join(dir, "dst/acorn.txt"), 100)
	dst := filepath.Join(dir, "dst")
	du := scanned(t, dst)

	journal := runDiskUsageJob(t, du, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeCopy, Status: jobs.StatusRunning,
		Sources: pathloc.PathsForTest(filepath.Join(dir, "src/basket")), Destination: pathloc.FileMust(dst),
	})
	waitSize(t, du, dst, 600)
	if n, _ := du.FileCount(dst); n != 3 {
		t.Fatalf("dst file count = %d, want 3", n)
	}

	runDiskUsageJob(t, du, &jobs.Job{ID: jobs.NewJobID(), Type: jobs.TypeUndo, Status: jobs.StatusRunning, Undo: journal})
	waitSize(t, du, dst, 100)
	waitGone(t, du, filepath.Join(dst, "basket"))
}

func TestCopyJobOverwriteNetsTheDifference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put(t, filepath.Join(dir, "src/acorn.txt"), 40)
	put(t, filepath.Join(dir, "dst/acorn.txt"), 100)
	put(t, filepath.Join(dir, "dst/birch.txt"), 50)
	dst := filepath.Join(dir, "dst")
	du := scanned(t, dst)

	runDiskUsageJob(t, du, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeCopy, Status: jobs.StatusRunning,
		Sources: pathloc.PathsForTest(filepath.Join(dir, "src/acorn.txt")), Destination: pathloc.FileMust(dst),
	})
	waitSize(t, du, dst, 90)
	if n, _ := du.FileCount(dst); n != 2 {
		t.Fatalf("dst file count = %d, want 2", n)
	}
}

func TestDeleteJobShrinksAncestorsAndRemovesEmptiedParents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put(t, filepath.Join(dir, "attic/trunk/quilt.txt"), 500)
	put(t, filepath.Join(dir, "meadow.txt"), 70)
	du := scanned(t, dir)
	waitSize(t, du, dir, 570)

	runDiskUsageJob(t, du, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning,
		Sources: pathloc.PathsForTest(filepath.Join(dir, "attic/trunk/quilt.txt")), DeleteRemoveEmptyDirs: true,
	})
	waitSize(t, du, dir, 70)
	waitGone(t, du, filepath.Join(dir, "attic/trunk"))
	if n, _ := du.FileCount(dir); n != 1 {
		t.Fatalf("parent file count = %d, want 1", n)
	}
}

func TestMoveJobCarriesSizesToDestination(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	put(t, filepath.Join(dir, "lantern/orchard/pebble.txt"), 4096)
	if err := os.Mkdir(filepath.Join(dir, "harbor"), 0o755); err != nil {
		t.Fatal(err)
	}
	du := scanned(t, dir)
	waitSize(t, du, dir, 4096)

	runDiskUsageJob(t, du, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeMove, Status: jobs.StatusRunning,
		Sources:     pathloc.PathsForTest(filepath.Join(dir, "lantern")),
		Destination: pathloc.FileMust(filepath.Join(dir, "harbor")),
	})
	waitSize(t, du, filepath.Join(dir, "harbor/lantern/orchard"), 4096)
	waitSize(t, du, filepath.Join(dir, "harbor"), 4096)
	waitGone(t, du, filepath.Join(dir, "lantern"))
	if n, _ := du.Size(dir); n != 4096 {
		t.Fatalf("shared ancestor total = %d, want unchanged 4096", n)
	}
}
