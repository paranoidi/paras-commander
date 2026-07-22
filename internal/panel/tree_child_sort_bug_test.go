package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// TestTreeModeSortRefreshesExpandedChildOrder reproduces a follow-up to the resyncTreeOrder bug:
// resyncTreeOrder (see tree_sort_bug_test.go) only reorders TreeRoots' depth-0 nodes to match a
// sort change. Any directory that was already expanded keeps its Children in whatever order they
// were sorted in at load time (ApplyTreeChildLoad / the synchronous fallback in
// setTreeNodeExpanded), even after ApplySort runs under a new sort mode. This checks that
// resyncTreeChildOrder walks already-loaded Children (recursively, so nested expansions are
// covered too) and re-sorts them to match the panel's current Sort.
func TestTreeModeSortRefreshesExpandedChildOrder(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "dir1")
	dir2 := filepath.Join(dir1, "dir2")
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Byte sizes chosen so name order and size order disagree, same trick the sibling bug test
	// uses for disk-usage totals. Files named so os.ReadDir's (alphabetical) load-time order and
	// ascending-size order are reversed for the plain files.
	writeSized(t, filepath.Join(dir1, "a_big.txt"), 300)
	writeSized(t, filepath.Join(dir1, "b_mid.txt"), 200)
	writeSized(t, filepath.Join(dir1, "c_small.txt"), 100)
	writeSized(t, filepath.Join(dir2, "x_big.txt"), 300)
	writeSized(t, filepath.Join(dir2, "y_mid.txt"), 200)
	writeSized(t, filepath.Join(dir2, "z_small.txt"), 100)

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 20) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	// Expand dir1 (cursor starts on it: the only depth-0 entry), then find and expand the nested
	// dir2 so both levels have loaded Children under the initial (name) sort.
	state.Cursor = 0
	if err := state.ExpandTreeCursorRow(20); err != nil {
		t.Fatalf("ExpandTreeCursorRow(dir1): %v", err)
	}
	state.Cursor = treeRowIndex(t, &state, dir2)
	if err := state.ExpandTreeCursorRow(20); err != nil {
		t.Fatalf("ExpandTreeCursorRow(dir2): %v", err)
	}

	// Switch to size-ascending sort and resync. Without resyncTreeChildOrder, dir1/dir2's already
	// loaded Children stay in their load-time (name) order.
	state.Sort = SortState{Mode: SortSize, DirectoriesFirst: true}
	state.ApplySort()

	wantDir1 := []string{"dir2", "c_small.txt", "b_mid.txt", "a_big.txt"}
	if got := childNames(t, state.TreeRoots, dir1); !equalStrings(got, wantDir1) {
		t.Fatalf("dir1 children after sort change = %v, want %v (stale load-time order)", got, wantDir1)
	}

	wantDir2 := []string{"z_small.txt", "y_mid.txt", "x_big.txt"}
	if got := childNames(t, state.TreeRoots, dir2); !equalStrings(got, wantDir2) {
		t.Fatalf("dir2 (nested) children after sort change = %v, want %v (recursion didn't reach depth 2)", got, wantDir2)
	}
}

func writeSized(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// treeRowIndex returns the index of the tree row whose ID equals path, failing the test if absent.
func treeRowIndex(t *testing.T, s *State, path string) int {
	t.Helper()
	for i, r := range s.treeRows {
		if r.ID == path {
			return i
		}
	}
	t.Fatalf("no tree row with ID %q", path)
	return -1
}

// childNames returns the Children names (in order) of the node identified by path.
func childNames(t *testing.T, roots []treeflat.Node[TreeEntry], path string) []string {
	t.Helper()
	node := findTreeNode(roots, path)
	if node == nil {
		t.Fatalf("node not found: %s", path)
	}
	names := make([]string, len(node.Children))
	for i, c := range node.Children {
		names[i] = c.Value.Entry.Name
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
