package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func selectionIndexTestTree(t *testing.T) (root, meadow string) {
	t.Helper()
	root = t.TempDir()
	meadow = filepath.Join(root, "meadow")
	if err := os.MkdirAll(meadow, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(meadow, "lantern.txt"), filepath.Join(root, "beacon.txt")} {
		if err := os.WriteFile(f, []byte("xy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, meadow
}

func TestTreeChildSelectionBytesCountedAndRemovedOnLeavingTree(t *testing.T) {
	root, meadow := selectionIndexTestTree(t)
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	s.SetListLayout(ListLayoutTree, 10)
	if err := s.setTreeNodeExpanded(meadow, 0, true, false); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(meadow, "lantern.txt")
	s.TogglePathSelection(child)
	if got := s.SelectionListedBytes(); got != 2 {
		t.Fatalf("SelectionListedBytes = %d, want 2", got)
	}
	if e, ok := s.ListingEntryAt(child); !ok || e.Type == localfs.EntryDirectory {
		t.Fatalf("ListingEntryAt = %+v ok=%v", e, ok)
	}
	s.SetListLayout(ListLayoutFlat, 10)
	if _, ok := s.ListingEntryAt(child); ok {
		t.Fatal("tree index must be dropped when leaving tree mode")
	}
}
