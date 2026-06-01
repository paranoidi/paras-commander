package panel

import (
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
