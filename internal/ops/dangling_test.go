package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestDanglingDirsAfterCascadeStopsAtSurvivingContent builds:
//
//	garden/keepme.txt        (survives — garden must NOT be reported)
//	garden/orchard/meadow/acorn.txt   (removed)
//	garden/orchard/meadow/pebble.txt  (removed)
//
// After removing the two files, meadow and orchard are both fully empty, but garden
// still has keepme.txt. DanglingDirsAfter must report only the topmost emptied dir
// (orchard), not the deeper meadow (covered by orchard's recursive removal) and not
// garden (has surviving content).
func TestDanglingDirsAfterCascadeStopsAtSurvivingContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	garden := filepath.Join(root, "garden")
	orchard := filepath.Join(garden, "orchard")
	meadow := filepath.Join(orchard, "meadow")
	if err := os.MkdirAll(meadow, 0o755); err != nil {
		t.Fatal(err)
	}
	keepme := filepath.Join(garden, "keepme.txt")
	if err := os.WriteFile(keepme, []byte("stays"), 0o644); err != nil {
		t.Fatal(err)
	}
	acorn := filepath.Join(meadow, "acorn.txt")
	pebble := filepath.Join(meadow, "pebble.txt")
	if err := os.WriteFile(acorn, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pebble, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate the completed move/delete: the files are already gone by the time
	// DanglingDirsAfter runs; only their former paths (job.Sources) are passed in.
	if err := os.Remove(acorn); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(pebble); err != nil {
		t.Fatal(err)
	}

	// A source whose parent directory no longer exists at all (the operation removed
	// the whole subtree) must be skipped without failing the other chains.
	vanished := filepath.Join(root, "thicket", "bramble.txt")

	sources := []pathloc.Path{pathloc.FileMust(acorn), pathloc.FileMust(pebble), pathloc.FileMust(vanished)}
	got, err := DanglingDirsAfter(context.Background(), sources)
	if err != nil {
		t.Fatalf("DanglingDirsAfter: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d dirs, want 1: %v", len(got), got)
	}
	if got[0].String() != pathloc.FileMust(orchard).String() {
		t.Fatalf("got %v, want topmost dir %v (not the deeper meadow, not garden which still has keepme.txt)", got[0], orchard)
	}
}
