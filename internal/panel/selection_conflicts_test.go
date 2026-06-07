package panel

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

func TestBulkApplySelectionAddsMatchesApplySelectionAdds(t *testing.T) {
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
