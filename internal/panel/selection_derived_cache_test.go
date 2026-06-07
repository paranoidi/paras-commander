package panel

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestBuildSubtreeSelAncestorsNonexistentFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inner := filepath.Join(root, "sub", "inner.txt")
	ancestors := buildSubtreeSelAncestors(map[string]bool{inner: true})
	if len(ancestors) == 0 {
		t.Fatalf("expected ancestors for %q, got none", inner)
	}
	if !ancestors[cleanPathString(root)] {
		t.Fatalf("root %q missing from %v", root, ancestors)
	}
}

func makeLargeFlatSelectionState(b testing.TB, n int) *State {
	b.Helper()
	root := b.TempDir()
	selected := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		p := filepath.Join(root, fmt.Sprintf("file_%04d.txt", i))
		selected[p] = true
	}
	return &State{
		Path:          pathloc.MustParse(root),
		SelectedPaths: selected,
	}
}

func TestSelectionsStripCountCachedLargeSelection(t *testing.T) {
	t.Parallel()
	s := makeLargeFlatSelectionState(t, 1500)
	if got := s.SelectionsStripCount(); got != 0 {
		t.Fatalf("strip count in cwd = %d, want 0", got)
	}
	if got := s.SelectionsStripCount(); got != 0 {
		t.Fatalf("cached strip count = %d, want 0", got)
	}
}

func TestPrunedSelectionRootsFilesOnlyFastPath(t *testing.T) {
	t.Parallel()
	s := makeLargeFlatSelectionState(t, 1500)
	pruned := s.PrunedSelectionRoots()
	if len(pruned) != 1500 {
		t.Fatalf("pruned len = %d, want 1500", len(pruned))
	}
}

func TestHasSelectionInSubtreeLegacyPaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	inner := filepath.Join(sub, "inner.txt")
	s := &State{
		SelectedPaths: map[string]bool{
			inner: true,
		},
	}
	if !s.HasSelectionInSubtree(root) {
		t.Fatalf("root %q should have subtree selection; ancestors=%v", root, s.selDerivedCache.subtreeAncestors)
	}
	if !s.HasSelectionInSubtree(sub) {
		t.Fatal("sub should have subtree selection")
	}
}

func TestHasSelectionInSubtreeCached(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	child := filepath.Join(sub, "child.txt")
	s := &State{
		Path: pathloc.MustParse(root),
		SelectedPaths: map[string]bool{
			child: true,
		},
	}
	if !s.HasSelectionInSubtree(sub) {
		t.Fatal("expected subtree mark for sub")
	}
	if s.HasSelectionInSubtree(child) {
		t.Fatal("file itself should not be subtree ancestor")
	}
}

func benchmarkLargeFlatSelection(b *testing.B, op func(*State)) {
	s := makeLargeFlatSelectionState(b, 1500)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op(s)
	}
}

func BenchmarkSelectionsStripCount1500(b *testing.B) {
	benchmarkLargeFlatSelection(b, func(s *State) {
		_ = s.SelectionsStripCount()
	})
}

func BenchmarkPrunedSelectionRoots1500(b *testing.B) {
	benchmarkLargeFlatSelection(b, func(s *State) {
		_ = s.PrunedSelectionRoots()
	})
}

func TestBulkAddSelectionsFilesOnlyFastPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := make([]string, 1500)
	pathIsDir := make(map[string]bool, 1500)
	for i := 0; i < 1500; i++ {
		p := filepath.Join(root, fmt.Sprintf("file_%04d.txt", i))
		paths[i] = p
		pathIsDir[filepath.Clean(p)] = false
	}
	isDir := func(path string) bool {
		return pathIsDir[filepath.Clean(path)]
	}
	s := &State{Path: pathloc.MustParse(root)}
	if removed := s.BulkAddSelections(paths, isDir); removed {
		t.Fatal("unexpected conflict removals for flat files")
	}
	if got := len(s.SelectedPaths); got != 1500 {
		t.Fatalf("selected count = %d, want 1500", got)
	}
	if s.selectionHasDirs {
		t.Fatal("selectionHasDirs should be false for file-only bulk add")
	}
}

func TestBulkAddSelectionsRemovesDirAncestorForFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parent := filepath.Join(root, "dir")
	file := filepath.Join(parent, "a.txt")
	pathIsDir := map[string]bool{
		filepath.Clean(parent): true,
		filepath.Clean(file):   false,
	}
	isDir := func(path string) bool {
		return pathIsDir[filepath.Clean(path)]
	}
	s := &State{
		Path:             pathloc.MustParse(root),
		SelectedPaths:    map[string]bool{filepath.Clean(parent): true},
		selectionHasDirs: true,
	}
	if !s.BulkAddSelections([]string{file}, isDir) {
		t.Fatal("expected conflict removal when adding file under selected dir")
	}
	if s.SelectedPaths[filepath.Clean(parent)] {
		t.Fatal("parent dir should be removed")
	}
	if !s.SelectedPaths[filepath.Clean(file)] {
		t.Fatal("file should be selected")
	}
}

func BenchmarkBulkAddSelections1500Files(b *testing.B) {
	root := b.TempDir()
	paths := make([]string, 1500)
	pathIsDir := make(map[string]bool, 1500)
	for i := 0; i < 1500; i++ {
		p := filepath.Join(root, fmt.Sprintf("file_%04d.txt", i))
		paths[i] = p
		pathIsDir[filepath.Clean(p)] = false
	}
	isDir := func(path string) bool {
		return pathIsDir[filepath.Clean(path)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := &State{Path: pathloc.MustParse(root)}
		_ = s.BulkAddSelections(paths, isDir)
	}
}

func BenchmarkHasSelectionInSubtree1500(b *testing.B) {
	root := b.TempDir()
	sub := filepath.Join(root, "sub")
	selected := make(map[string]bool, 1500)
	for i := 0; i < 1500; i++ {
		selected[filepath.Join(sub, fmt.Sprintf("f_%04d.txt", i))] = true
	}
	s := &State{
		Path:          pathloc.MustParse(root),
		SelectedPaths: selected,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.HasSelectionInSubtree(sub)
	}
}
