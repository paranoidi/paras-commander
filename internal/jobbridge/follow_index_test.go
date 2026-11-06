package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fileindex"
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

// indexedFile writes harbor.txt under dir/orchard (next to an unindexed node_modules/pebble.txt),
// records it in a fresh index registered behind fileindex://, and returns its disk path, its
// fileindex:// path and the index.
func indexedFile(t *testing.T) (file, parent string, src pathloc.Path, idx *fileindex.Index) {
	t.Helper()
	dir := t.TempDir()
	parent = filepath.Join(dir, "orchard")
	file = filepath.Join(parent, "harbor.txt")
	for _, p := range []string{file, filepath.Join(parent, "node_modules", "pebble.txt")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Init(); err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("a", 32)
	if _, err := d.CommitScanBatch([]db.ScannedFile{{Dir: parent, Name: "harbor.txt", Size: 1, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	idx = fileindex.New(config.FileIndexConfig{Database: dbPath})
	t.Cleanup(func() { _ = idx.Close() })
	if err := fileindex.Register(idx); err != nil {
		t.Fatal(err)
	}
	if src, err = pathloc.FileIndex(file); err != nil {
		t.Fatal(err)
	}
	return file, parent, src, idx
}

func TestDeleteOfIndexPathsOnDiskCleansEmptyParentsOnDiskOnly(t *testing.T) {
	file, parent, src, _ := indexedFile(t)
	emptyKid := filepath.Join(parent, "empty")
	if err := os.Mkdir(emptyKid, 0o755); err != nil {
		t.Fatal(err)
	}
	index := &changeLog{}
	runJob(t, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning, FollowIndex: true, DeleteOnDisk: true,
		DeleteRemoveEmptyDirs: true, Sources: []pathloc.Path{src},
	}, nil, index)
	if _, err := os.Stat(file); err == nil {
		t.Fatal("the file is still on disk")
	}
	if _, err := os.Stat(filepath.Join(parent, "node_modules", "pebble.txt")); err != nil {
		t.Fatalf("the cleanup removed unindexed content: %v", err)
	}
	if _, err := os.Stat(emptyKid); err == nil {
		t.Fatal("the cleanup left an empty directory beside the deleted file")
	}
	if got := index.paths(); !slices.Contains(got, file) {
		t.Fatalf("index sink got %v, want the deleted file's disk path", got)
	}
}

func TestDeleteOfIndexPathsWithoutDeleteOnDiskLeavesDiskAlone(t *testing.T) {
	file, parent, src, idx := indexedFile(t)
	index := &changeLog{}
	runJob(t, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning, FollowIndex: true,
		DeleteRemoveEmptyDirs: true, Sources: []pathloc.Path{src},
	}, nil, index)
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the file left the disk: %v", err)
	}
	if got := index.paths(); len(got) != 0 {
		t.Fatalf("index sink got %v; the backend changes the index itself", got)
	}
	d, err := idx.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := d.FileInfo(file); found {
		t.Fatal("the record is still in the index")
	}
	if _, err := os.Stat(parent); err != nil {
		t.Fatalf("cleanup removed a directory: %v", err)
	}
}

func TestDeleteOnDiskOfFileGoneFromDiskRemovesRecordOnly(t *testing.T) {
	file, _, src, idx := indexedFile(t)
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	runJob(t, &jobs.Job{
		ID: jobs.NewJobID(), Type: jobs.TypeDelete, Status: jobs.StatusRunning, FollowIndex: true, DeleteOnDisk: true,
		DeleteRemoveEmptyDirs: true, Sources: []pathloc.Path{src},
	}, nil, &changeLog{})
	d, err := idx.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, _ := d.FileInfo(file); found {
		t.Fatal("the record of a file gone from disk is still in the index")
	}
}
