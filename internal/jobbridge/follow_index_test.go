package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type changeLog struct {
	mu  sync.Mutex
	got []fschange.Change
}

func (c *changeLog) Apply(b []fschange.Change) {
	c.mu.Lock()
	c.got = append(c.got, b...)
	c.mu.Unlock()
}

func (c *changeLog) paths() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, ch := range c.got {
		out = append(out, ch.Path)
	}
	return out
}

// runJob runs job through TransferFunc with the given sinks.
func runJob(t *testing.T, job *jobs.Job, fsSink, indexSink fschange.Sink) {
	t.Helper()
	cfg := config.Default()
	transfer := TransferFunc(cfg.Operations, cfg.Jobs, cfg.Compare, nil, nil, fsSink, indexSink)
	if err := transfer(context.Background(), job, func(jobs.Event) {}, nil); err != nil {
		t.Fatalf("transfer: %v", err)
	}
}

func TestIndexSinkReceivesChangesOnlyForFollowingJobs(t *testing.T) {
	for _, follow := range []bool{true, false} {
		dir := t.TempDir()
		file := filepath.Join(dir, "harbor.txt")
		if err := os.WriteFile(file, []byte("anchor"), 0o644); err != nil {
			t.Fatal(err)
		}
		disk, index := &changeLog{}, &changeLog{}
		runJob(t, &jobs.Job{
			ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning, FollowIndex: follow,
			Sources: pathloc.PathsForTest(file),
		}, disk, index)
		if got := disk.paths(); len(got) != 1 || got[0] != file {
			t.Errorf("follow=%v: disk-usage sink got %v, want the deleted file", follow, got)
		}
		got := index.paths()
		if follow && (len(got) != 1 || got[0] != file) {
			t.Errorf("a following job reported %v to the index, want the deleted file", got)
		}
		if !follow && len(got) != 0 {
			t.Errorf("a job that does not follow the index reported %v to it", got)
		}
	}
}

func TestFollowingJobWorksWithoutDiskUsageSink(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "harbor.txt")
	if err := os.WriteFile(file, []byte("anchor"), 0o644); err != nil {
		t.Fatal(err)
	}
	index := &changeLog{}
	runJob(t, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning, FollowIndex: true,
		Sources: pathloc.PathsForTest(file),
	}, nil, index)
	if got := index.paths(); len(got) != 1 {
		t.Fatalf("index sink got %v, want the deleted file", got)
	}
}

func TestDeleteOfIndexPathsCleansEmptyParentsOnDiskOnly(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "orchard")
	file := filepath.Join(parent, "harbor.txt")
	keep := filepath.Join(parent, "node_modules", "pebble.txt") // not in the index
	for _, p := range []string{file, keep} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src, err := pathloc.FileIndex(file)
	if err != nil {
		t.Fatal(err)
	}
	index := &changeLog{}
	runJob(t, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning, FollowIndex: true,
		DeleteRemoveEmptyDirs: true, Sources: []pathloc.Path{src},
	}, nil, index)
	if _, err := os.Stat(file); err == nil {
		t.Fatal("the file is still on disk")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("the cleanup removed unindexed content: %v", err)
	}
	for _, p := range index.paths() {
		if p == parent {
			t.Fatalf("the non-empty parent was reported removed: %v", index.paths())
		}
	}
}
