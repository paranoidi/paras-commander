package compare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestWalkRootSkipsSymlinks(t *testing.T) {
	rootDir := t.TempDir()
	scan := filepath.Join(rootDir, "scan")
	hidden := filepath.Join(rootDir, "hidden", "other")
	if err := os.MkdirAll(scan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, scan, "real1.txt", "duplicate!")
	writeFile(t, scan, "real2.txt", "duplicate!")
	writeFile(t, hidden, "hidden-dup.txt", "duplicate!")
	if err := os.Symlink("real1.txt", filepath.Join(scan, "link1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real2.txt", filepath.Join(scan, "link2")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "hidden", "other"), filepath.Join(scan, "other-link")); err != nil {
		t.Fatal(err)
	}

	root, err := pathloc.File(scan)
	if err != nil {
		t.Fatal(err)
	}

	withSymlinks, err := WalkRoot(context.Background(), root, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(withSymlinks) < 3 {
		t.Fatalf("default walk files = %d, want at least symlink entries indexed", len(withSymlinks))
	}

	skipped, err := WalkRoot(context.Background(), root, WalkOptions{SkipSymlinks: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 2 {
		t.Fatalf("skip-symlinks walk files = %d, want 2 regular files only", len(skipped))
	}
	for _, f := range skipped {
		if f.Rel != "real1.txt" && f.Rel != "real2.txt" {
			t.Fatalf("unexpected file %q in skip-symlinks walk", f.Rel)
		}
	}
}
