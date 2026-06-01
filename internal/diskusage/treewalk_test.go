package diskusage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/diskusage"
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
	tree := diskusage.WalkFolder(locked, nil, nil, nil, 1)
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

	tree := diskusage.WalkFolder(root, nil, nil, nil, 4)
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
