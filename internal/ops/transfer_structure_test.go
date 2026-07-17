package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/testutil"
)

// Multi-directory selections (issued from the common root) must keep their
// directory structure under the destination instead of flattening to basenames.

func multiDirSources(t *testing.T) (root string, sources []pathloc.Path) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "maple"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "birch", "seed"), 0o755); err != nil {
		t.Fatal(err)
	}
	testutil.WriteFile(t, filepath.Join(root, "maple", "leaf.txt"))
	testutil.WriteFile(t, filepath.Join(root, "maple", "bark.txt"))
	testutil.WriteFile(t, filepath.Join(root, "birch", "seed", "pod.txt"))
	return root, pathloc.PathsForTest(
		filepath.Join(root, "maple", "leaf.txt"),
		filepath.Join(root, "maple", "bark.txt"),
		filepath.Join(root, "birch"),
	)
}

func TestExecuteCopyPreservesMultiDirStructure(t *testing.T) {
	_, sources := multiDirSources(t)
	dest := t.TempDir()

	if _, _, err := ExecuteCopy(context.Background(), sources, pathloc.FileMust(dest), Options{}, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteCopy: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("maple", "leaf.txt"),
		filepath.Join("maple", "bark.txt"),
		filepath.Join("birch", "seed", "pod.txt"),
	} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("expected %s at destination: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "leaf.txt")); !os.IsNotExist(err) {
		t.Errorf("leaf.txt flattened to destination root")
	}
}

func TestExecuteMovePreservesMultiDirStructure(t *testing.T) {
	root, sources := multiDirSources(t)
	dest := t.TempDir()

	if _, _, err := ExecuteMove(context.Background(), sources, pathloc.FileMust(dest), Options{}, ProgressEmitThrottle{}, nil, nil, nil); err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("maple", "leaf.txt"),
		filepath.Join("birch", "seed", "pod.txt"),
	} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("expected %s at destination: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "maple", "leaf.txt")); !os.IsNotExist(err) {
		t.Errorf("source maple/leaf.txt still exists after move")
	}
}

func TestSelfTargetCountMultiDirIntoCommonRoot(t *testing.T) {
	root, sources := multiDirSources(t)

	if n := SelfTargetCount(sources, pathloc.FileMust(root), false); n != len(sources) {
		t.Errorf("SelfTargetCount into common root = %d, want %d", n, len(sources))
	}
	if n := SelfTargetCount(sources, pathloc.FileMust(t.TempDir()), false); n != 0 {
		t.Errorf("SelfTargetCount into fresh dir = %d, want 0", n)
	}
}

func TestSelfTargetCountFlatDestNames(t *testing.T) {
	root, sources := multiDirSources(t)

	// With flatten, naming is basename-only: nested sources (maple/leaf.txt,
	// maple/bark.txt) no longer self-target, but "birch" (already a direct
	// child of root) still does.
	if n := SelfTargetCount(sources, pathloc.FileMust(root), true); n != 1 {
		t.Errorf("SelfTargetCount flat into common root = %d, want 1", n)
	}
	if n := SelfTargetCount(sources, pathloc.FileMust(t.TempDir()), true); n != 0 {
		t.Errorf("SelfTargetCount flat into fresh dir = %d, want 0", n)
	}
}

func TestBuildPlanSingleDirStillUsesBasenames(t *testing.T) {
	src := t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "fern.txt"))
	testutil.WriteFile(t, filepath.Join(src, "moss.txt"))
	dest := t.TempDir()

	plan, err := BuildPlan(pathloc.PathsForTest(
		filepath.Join(src, "fern.txt"),
		filepath.Join(src, "moss.txt"),
	), pathloc.FileMust(dest), true)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	for _, item := range plan {
		if item.Dst.Parent().String() != dest {
			t.Errorf("item %s resolved to %s, want direct child of %s", item.Src, item.Dst, dest)
		}
	}
}
