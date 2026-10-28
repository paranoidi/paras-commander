package jobbridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func runDiskUsageSyncJob(t *testing.T, du *diskusage.Engine, job *jobs.Job) {
	t.Helper()
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, cfg.Compare, nil, nil, du)
	if err := transfer(context.Background(), job, func(jobs.Event) {}, func(jobs.BlockerRequest) jobs.BlockerAnswer { return jobs.BlockerAnswer{} }); err != nil {
		t.Fatalf("transfer: %v", err)
	}
}

func mkLantern(t *testing.T, dir string) (src, nested string) {
	t.Helper()
	src = filepath.Join(dir, "lantern")
	nested = filepath.Join(src, "orchard")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "pebble.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	return src, nested
}

func TestDeleteJobDropsDiskUsageOfSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, nested := mkLantern(t, dir)
	other := filepath.Join(dir, "meadow")
	du := diskusage.New()
	du.SetSize(dir, 4, 13096)
	du.SetSize(src, 1, 4096)
	du.SetSize(nested, 1, 4096)
	du.SetSize(other, 3, 9000)

	runDiskUsageSyncJob(t, du, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning,
		Sources: []pathloc.Path{pathloc.FileMust(src)},
	})
	for _, p := range []string{src, nested} {
		if _, ok := du.Size(p); ok {
			t.Fatalf("size of deleted %q still cached", p)
		}
	}
	if _, ok := du.Size(other); !ok {
		t.Fatal("unrelated size dropped")
	}
	if n, _ := du.Size(dir); n != 9000 {
		t.Fatalf("parent total = %d, want 9000 after removing 4096", n)
	}
	if n, _ := du.FileCount(dir); n != 3 {
		t.Fatalf("parent file count = %d, want 3", n)
	}
}

func TestMoveJobRebasesRenamedDiskUsage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, nested := mkLantern(t, dir)
	dstDir := filepath.Join(dir, "harbor")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	du := diskusage.New()
	du.SetSize(dir, 1, 4096)
	du.SetSize(src, 1, 4096)
	du.SetSize(nested, 1, 4096)
	du.SetSize(dstDir, 0, 0)

	runDiskUsageSyncJob(t, du, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeMove, Status: jobs.StatusRunning,
		Sources:     []pathloc.Path{pathloc.FileMust(src)},
		Destination: pathloc.FileMust(dstDir),
	})
	if _, ok := du.Size(src); ok {
		t.Fatal("size still cached at the moved-away source")
	}
	if n, ok := du.Size(filepath.Join(dstDir, "lantern", "orchard")); !ok || n != 4096 {
		t.Fatalf("rebased nested size = %d, %v; want 4096, true", n, ok)
	}
	if n, _ := du.Size(dstDir); n != 4096 {
		t.Fatalf("destination parent total = %d, want 4096", n)
	}
	if n, _ := du.Size(dir); n != 4096 {
		t.Fatalf("shared ancestor total = %d, want unchanged 4096", n)
	}
}

// fakeDiskUsageCache records calls in order.
type fakeDiskUsageCache struct{ calls []string }

func (f *fakeDiskUsageCache) RenamePaths([][2]string) { f.calls = append(f.calls, "rename") }
func (f *fakeDiskUsageCache) DropSubtrees(gone, present []string) {
	f.calls = append(f.calls, fmt.Sprintf("drop%v%v", gone, present))
}

func TestSyncDiskUsageCacheRenamesBeforeDropping(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	renamed, gone, kept := filepath.Join(dir, "willow"), filepath.Join(dir, "lantern"), filepath.Join(dir, "meadow")
	if err := os.Mkdir(kept, 0o755); err != nil {
		t.Fatal(err)
	}
	c := &fakeDiskUsageCache{}
	job := &jobs.Job{Type: jobs.TypeMove, Sources: pathloc.PathsForTest(renamed, gone, kept)}
	syncDiskUsageCache(c, job, [][2]string{{renamed, filepath.Join(dir, "harbor", "willow")}})
	want := []string{"rename", fmt.Sprintf("drop[%s][%s]", gone, kept)}
	if fmt.Sprint(c.calls) != fmt.Sprint(want) {
		t.Fatalf("calls = %v, want %v", c.calls, want)
	}
	c.calls = nil
	syncDiskUsageCache(c, &jobs.Job{Type: jobs.TypeCopy, Sources: job.Sources}, nil)
	if len(c.calls) != 0 {
		t.Fatalf("copy job calls = %v, want none", c.calls)
	}
}
