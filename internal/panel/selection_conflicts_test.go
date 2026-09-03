package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func testSelectionConflictDirs(t *testing.T) (root, parent, child, file string) {
	t.Helper()
	root = t.TempDir()
	parent = filepath.Join(root, "parent")
	child = filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	file = filepath.Join(child, "leaf.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, parent, child, file
}

func testExistingIsDir(t *testing.T, dirs map[string]bool) func(string) bool {
	t.Helper()
	return func(p string) bool {
		return dirs[filepath.Clean(p)]
	}
}

func TestClearSelectionConflictsChildDirThenParent(t *testing.T) {
	_, parent, child, _ := testSelectionConflictDirs(t)
	selected := map[string]bool{
		filepath.Clean(child): true,
	}
	isDir := testExistingIsDir(t, map[string]bool{child: true})
	if !ClearSelectionConflicts(selected, parent, true, isDir) {
		t.Fatal("expected conflicts removed")
	}
	if selected[filepath.Clean(child)] {
		t.Fatal("child should be removed when parent dir is added")
	}
}

func TestClearSelectionConflictsParentDirThenChild(t *testing.T) {
	_, parent, child, _ := testSelectionConflictDirs(t)
	selected := map[string]bool{
		filepath.Clean(parent): true,
	}
	isDir := testExistingIsDir(t, map[string]bool{parent: true})
	if !ClearSelectionConflicts(selected, child, true, isDir) {
		t.Fatal("expected conflicts removed")
	}
	if selected[filepath.Clean(parent)] {
		t.Fatal("parent should be removed when child dir is added")
	}
}

func TestClearSelectionConflictsFileRemovesAncestorDir(t *testing.T) {
	_, parent, _, file := testSelectionConflictDirs(t)
	selected := map[string]bool{
		filepath.Clean(parent): true,
	}
	isDir := testExistingIsDir(t, map[string]bool{parent: true})
	if !ClearSelectionConflicts(selected, file, false, isDir) {
		t.Fatal("expected conflicts removed")
	}
	if selected[filepath.Clean(parent)] {
		t.Fatal("parent dir should be removed when descendant file is added")
	}
}

func TestClearSelectionConflictsFileUsesAncestorWalkOnly(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	siblingDir := filepath.Join(root, "other")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(parent, "leaf.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{
		filepath.Clean(parent):     true,
		filepath.Clean(siblingDir): true,
	}
	isDir := testExistingIsDir(t, map[string]bool{
		filepath.Clean(parent):     true,
		filepath.Clean(siblingDir): true,
	})
	if !ClearSelectionConflicts(selected, file, false, isDir) {
		t.Fatal("expected parent dir removed")
	}
	if selected[filepath.Clean(parent)] {
		t.Fatal("parent dir should be removed")
	}
	if !selected[filepath.Clean(siblingDir)] {
		t.Fatal("unrelated sibling dir should remain selected")
	}
}

func TestBulkApplySelectionAddsWalkOrderMatchesSequential(t *testing.T) {
	root, parent, child, file := testSelectionConflictDirs(t)
	isDir := testExistingIsDir(t, map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(child):  true,
	})
	// WalkDir pre-order: parent, child, file
	walkOrder := []string{parent, child, file}
	want := make(map[string]bool)
	ApplySelectionAdds(want, walkOrder, isDir)
	got := make(map[string]bool)
	BulkApplySelectionAddsWalkOrder(got, walkOrder, isDir)
	for path, on := range want {
		if got[path] != on {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	for path := range got {
		if !want[path] {
			t.Fatalf("unexpected %q in %v", path, got)
		}
	}
	_ = root
}

func TestToggleSelectionRemovesConflictingNestedDirs(t *testing.T) {
	root, parent, child, _ := testSelectionConflictDirs(t)
	state := State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: filepath.Base(parent), Path: parent, Type: localfs.EntryDirectory},
		},
		SelectedPaths: map[string]bool{filepath.Clean(child): true},
	}
	selected, conflicts := state.ToggleSelection()
	if !selected || !conflicts {
		t.Fatalf("toggle parent: selected=%v conflicts=%v", selected, conflicts)
	}
	if state.SelectedPaths[filepath.Clean(child)] {
		t.Fatal("child selection should be removed when parent is selected")
	}
	if !state.SelectedPaths[filepath.Clean(parent)] {
		t.Fatal("parent should be selected")
	}
}

func TestApplySelectionAddsMatchesSequentialClear(t *testing.T) {
	_, parent, child, file := testSelectionConflictDirs(t)
	isDir := testExistingIsDir(t, map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(child):  true,
	})
	orders := [][]string{
		{file, parent},
		{parent, file},
		{child, parent},
		{parent, child, file},
	}
	for _, order := range orders {
		want := make(map[string]bool)
		for _, path := range order {
			path = filepath.Clean(path)
			ClearSelectionConflicts(want, path, isDir(path), isDir)
			want[path] = true
		}
		got := make(map[string]bool)
		ApplySelectionAdds(got, order, isDir)
		for path, on := range want {
			if got[path] != on {
				t.Fatalf("order %v: got %v want %v", order, got, want)
			}
		}
		for path := range got {
			if !want[path] {
				t.Fatalf("order %v: unexpected %q in %v", order, path, got)
			}
		}
	}
}

// BulkApplySelectionAdds resolves conflicts by path depth (shallowest directory processed
// first), not by the sequential "last add wins" rule ApplySelectionAdds uses — the two only
// agree when the batch happens to already be walk-ordered (every directory before its own
// descendants). This is why Bulk no longer matches Apply for every input order: it trades
// order-dependence for O(n log n) batch performance, which is safe because a bulk call
// represents one atomic "mark all these paths" action, not a sequence of distinct user actions
// (interactive single-path toggling keeps its own "last action wins" semantics unchanged, via
// State.ToggleSelection / resolveSelectionConflicts).
func TestBulkApplySelectionAddsOrderIndependent(t *testing.T) {
	_, parent, child, file := testSelectionConflictDirs(t)
	isDir := testExistingIsDir(t, map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(child):  true,
	})
	// For each batch, only the deepest path in the ancestor/descendant chain should survive,
	// regardless of the order the batch is given in: shallower dirs are always processed
	// first (depth sort) and then evicted by clearSelectionDirAncestors once their
	// descendant is added.
	cases := []struct {
		order []string
		want  string
	}{
		{[]string{file, parent}, file},
		{[]string{parent, file}, file},
		{[]string{child, parent}, child},
		{[]string{parent, child, file}, file},
	}
	for _, tc := range cases {
		want := map[string]bool{filepath.Clean(tc.want): true}
		got := make(map[string]bool)
		BulkApplySelectionAdds(got, tc.order, isDir)
		if len(got) != len(want) || !got[filepath.Clean(tc.want)] {
			t.Fatalf("order %v: got %v want %v", tc.order, got, want)
		}
	}
}

func TestBulkApplySelectionAddsWalkOrderedMatchesApplySelectionAdds(t *testing.T) {
	_, parent, child, file := testSelectionConflictDirs(t)
	isDir := testExistingIsDir(t, map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(child):  true,
	})
	// Walk-ordered batches (parent before descendant, matching how SelectGroup feeds it from
	// a listing/tree traversal) are exactly where Bulk and Apply are guaranteed to agree.
	orders := [][]string{
		{parent, file},
		{parent, child, file},
	}
	for _, order := range orders {
		want := make(map[string]bool)
		ApplySelectionAdds(want, order, isDir)
		got := make(map[string]bool)
		BulkApplySelectionAdds(got, order, isDir)
		for path, on := range want {
			if got[path] != on {
				t.Fatalf("order %v: got %v want %v", order, got, want)
			}
		}
		for path := range got {
			if !want[path] {
				t.Fatalf("order %v: unexpected %q in %v", order, path, got)
			}
		}
	}
}

func TestBulkApplySelectionAddsClearsPreexistingDescendants(t *testing.T) {
	_, parent, child, file := testSelectionConflictDirs(t)
	isDir := testExistingIsDir(t, map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(child):  true,
	})
	// A pre-existing selection contains a deep descendant; a bulk batch then adds a shallower
	// covering directory. The descendant must be evicted even though it was never part of the
	// batch itself (Phase 1 of applySelectionAddsBulk).
	selected := map[string]bool{filepath.Clean(file): true}
	if !BulkApplySelectionAdds(selected, []string{parent}, isDir) {
		t.Fatal("expected conflicts removed")
	}
	if selected[filepath.Clean(file)] {
		t.Fatal("preexisting descendant file should be removed when covering parent dir is bulk-added")
	}
	if !selected[filepath.Clean(parent)] {
		t.Fatal("parent dir should be selected")
	}
}

func TestBulkApplySelectionAddsLargeFlatListingIsFast(t *testing.T) {
	root := t.TempDir()
	const n = 8000
	paths := make([]string, n)
	pathIsDir := make(map[string]bool, n)
	for i := range n {
		p := filepath.Join(root, fmt.Sprintf("dir_%05d", i))
		paths[i] = p
		pathIsDir[filepath.Clean(p)] = true
	}
	isDir := func(path string) bool { return pathIsDir[filepath.Clean(path)] }
	selected := make(map[string]bool, n)
	done := make(chan struct{})
	go func() {
		BulkApplySelectionAdds(selected, paths, isDir)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("BulkApplySelectionAdds took too long on a large flat listing (likely quadratic)")
	}
	if len(selected) != n {
		t.Fatalf("got %d selected, want %d", len(selected), n)
	}
}

func TestPruneSelectionConflictsRemovesNestedPaths(t *testing.T) {
	_, parent, _, file := testSelectionConflictDirs(t)
	selected := map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(file):   true,
	}
	isDir := testExistingIsDir(t, map[string]bool{filepath.Clean(parent): true})
	if !PruneSelectionConflicts(selected, isDir) {
		t.Fatal("expected nested conflicts to be pruned")
	}
	if !selected[filepath.Clean(parent)] {
		t.Fatal("parent dir should remain after depth-descending prune")
	}
	if selected[filepath.Clean(file)] {
		t.Fatal("descendant file should be removed when parent dir remains")
	}
}

func TestClearSelectionConflictsParentDirRemovesDescendantFile(t *testing.T) {
	_, parent, _, file := testSelectionConflictDirs(t)
	selected := map[string]bool{
		filepath.Clean(file): true,
	}
	isDir := testExistingIsDir(t, map[string]bool{})
	if !ClearSelectionConflicts(selected, parent, true, isDir) {
		t.Fatal("expected conflicts removed")
	}
	if selected[filepath.Clean(file)] {
		t.Fatal("descendant file should be removed when parent dir is added")
	}
}

func BenchmarkBulkApplySelectionAdds10000Files(b *testing.B) {
	root := b.TempDir()
	const n = 10000
	paths := make([]string, n)
	pathIsDir := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		p := filepath.Join(root, fmt.Sprintf("file_%05d.txt", i))
		paths[i] = p
		pathIsDir[filepath.Clean(p)] = false
	}
	isDir := func(path string) bool {
		return pathIsDir[filepath.Clean(path)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selected := make(map[string]bool, n)
		_ = BulkApplySelectionAdds(selected, paths, isDir)
	}
}

func BenchmarkBulkApplySelectionAddsMixed10000(b *testing.B) {
	root := b.TempDir()
	const dirs = 100
	const filesPerDir = 100
	pathIsDir := make(map[string]bool, dirs+dirs*filesPerDir)
	paths := make([]string, 0, dirs+dirs*filesPerDir)
	for d := range dirs {
		dir := filepath.Join(root, fmt.Sprintf("dir_%03d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		dir = filepath.Clean(dir)
		pathIsDir[dir] = true
		paths = append(paths, dir)
		for f := range filesPerDir {
			p := filepath.Join(dir, fmt.Sprintf("file_%03d.txt", f))
			paths = append(paths, p)
			pathIsDir[p] = false
		}
	}
	isDir := func(path string) bool {
		return pathIsDir[filepath.Clean(path)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selected := make(map[string]bool, len(paths))
		_ = BulkApplySelectionAddsWalkOrder(selected, paths, isDir)
	}
}
