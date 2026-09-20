package diskusage_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fswalk"
)

// TestWalkFolderUnreadableDirCachesZeroNotDot verifies that when a top-level scan root
// cannot be read (e.g. permission denied), FlattenSizes still records the entry under
// its real path (not "."), so ListingFullyDiskCached can detect full coverage.
func TestWalkFolderUnreadableDirCachesZeroNotDot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	// WalkFolder on the locked directory should return a node whose Path() == locked.
	tree := diskusage.WalkFolder(context.Background(), locked, nil, nil, nil, fswalk.Params{InitialWorkers: 1, MaxWorkers: 1, AdaptIntervalMS: 60000})
	got := map[string]int64{}
	diskusage.FlattenSizes(tree, got)

	if _, ok := got[filepath.Clean(locked)]; !ok {
		t.Fatalf("locked dir key missing; got keys: %v", got)
	}
	if got[filepath.Clean(locked)] != 0 {
		t.Fatalf("expected size 0 for unreadable dir, got %d", got[filepath.Clean(locked)])
	}
	if _, hasDot := got["."]; hasDot {
		t.Fatal(`cache must not contain "." — unreadable root was not named properly`)
	}
}

func TestWalkFolderCancelStopsFurtherReads(t *testing.T) {
	t.Parallel()

	const (
		root = "/walk-root"
		dirA = "/walk-root/dirA"
		dirB = "/walk-root/dirB"
	)
	enteredChild := make(chan struct{})
	blockChild := make(chan struct{})
	var mu sync.Mutex
	var reads []string

	readDir := func(path string) ([]fs.FileInfo, error) {
		clean := filepath.Clean(path)
		mu.Lock()
		reads = append(reads, clean)
		isFirstChild := clean != root && len(reads) == 2
		mu.Unlock()
		if isFirstChild {
			close(enteredChild)
			<-blockChild
		}
		if clean == root {
			return []fs.FileInfo{
				walkFakeInfo{name: "dirA", isDir: true},
				walkFakeInfo{name: "dirB", isDir: true},
			}, nil
		}
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = diskusage.WalkFolder(ctx, root, readDir, nil, nil, fswalk.Params{
			InitialWorkers:  1,
			MaxWorkers:      1,
			AdaptIntervalMS: 60000,
		})
	}()

	select {
	case <-enteredChild:
	case <-time.After(2 * time.Second):
		t.Fatal("did not reach blocking child ReadDir")
	}
	cancel()
	close(blockChild)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WalkFolder did not return after cancel")
	}

	mu.Lock()
	got := append([]string(nil), reads...)
	mu.Unlock()
	var childReads int
	for _, p := range got {
		if p == dirA || p == dirB {
			childReads++
		}
	}
	if childReads != 1 {
		t.Fatalf("WalkFolder kept descending after cancel: reads=%v", got)
	}
}

type walkFakeInfo struct {
	name  string
	isDir bool
}

func (f walkFakeInfo) Name() string { return f.name }
func (f walkFakeInfo) Size() int64  { return 0 }
func (f walkFakeInfo) Mode() fs.FileMode {
	if f.isDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (f walkFakeInfo) ModTime() time.Time { return time.Time{} }
func (f walkFakeInfo) IsDir() bool        { return f.isDir }
func (f walkFakeInfo) Sys() any           { return nil }

func TestWalkFolderFlatSizes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("xy"), 0o644); err != nil {
		t.Fatal(err)
	}

	tree := diskusage.WalkFolder(context.Background(), root, nil, nil, nil, fswalk.Params{InitialWorkers: 4, MaxWorkers: 4, AdaptIntervalMS: 60000})
	got := map[string]int64{}
	diskusage.FlattenSizes(tree, got)

	if _, ok := got[filepath.Clean(root)]; !ok {
		t.Fatalf("missing root key in %#v", got)
	}
	if got[filepath.Clean(root)] < 7 {
		t.Fatalf("aggregate too small %d", got[filepath.Clean(root)])
	}

	fileCounts := map[string]int64{}
	diskusage.FlattenFileCounts(tree, fileCounts)
	if fileCounts[filepath.Clean(root)] != 2 {
		t.Fatalf("root file count = %d, want 2", fileCounts[filepath.Clean(root)])
	}
	if fileCounts[filepath.Clean(sub)] != 1 {
		t.Fatalf("sub file count = %d, want 1", fileCounts[filepath.Clean(sub)])
	}
	if diskusage.CountFilesInSubtree(tree) != 2 {
		t.Fatalf("CountFilesInSubtree = %d, want 2", diskusage.CountFilesInSubtree(tree))
	}
}
