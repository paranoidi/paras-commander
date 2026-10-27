package panel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStashSaveFromSelectionAndRestore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	for _, p := range []string{aPath, bPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &State{
		SelectedPaths: map[string]bool{aPath: true, bPath: true},
	}
	if !s.StashSaveFromSelection() {
		t.Fatal("StashSaveFromSelection() = false, want true")
	}
	if s.StashPathCount() != 2 {
		t.Fatalf("StashPathCount() = %d, want 2", s.StashPathCount())
	}
	if s.SelectedPathCount() != 0 {
		t.Fatalf("live selection should be cleared, got %d", s.SelectedPathCount())
	}
	s.ApplySelectionSnapshot(s.SelectionStashPaths, s.SelectionStashStripOrder, "")
	s.StashClear()
	if s.SelectedPathCount() != 2 {
		t.Fatalf("restored count = %d, want 2", s.SelectedPathCount())
	}
	if !s.SelectedPaths[aPath] || !s.SelectedPaths[bPath] {
		t.Fatalf("restored paths = %v", s.SelectedPaths)
	}
}

func TestApplySelectionSnapshotSkipsMissingPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(dir, "gone.txt")
	s := &State{}
	s.ApplySelectionSnapshot([]string{keep, gone}, []string{gone, keep}, "")
	if s.SelectedPathCount() != 1 {
		t.Fatalf("count = %d, want 1", s.SelectedPathCount())
	}
	if !s.SelectedPaths[keep] {
		t.Fatalf("paths = %v", s.SelectedPaths)
	}
	if len(s.SelectedStripOrder) != 0 {
		t.Fatalf("strip order should omit missing paths, got %v", s.SelectedStripOrder)
	}
}

func TestStashSaveFromSelectionEmpty(t *testing.T) {
	t.Parallel()
	s := &State{}
	if s.StashSaveFromSelection() {
		t.Fatal("StashSaveFromSelection() on empty selection should be false")
	}
}

func TestMergeSelectionSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	newPath := filepath.Join(dir, "new.txt")
	for _, p := range []string{keep, newPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &State{SelectedPaths: map[string]bool{keep: true}}
	s.MergeSelectionSnapshot([]string{newPath, keep}, nil, "")
	if s.SelectedPathCount() != 2 {
		t.Fatalf("count = %d, want 2", s.SelectedPathCount())
	}
	if !s.SelectedPaths[newPath] || !s.SelectedPaths[keep] {
		t.Fatalf("paths = %v", s.SelectedPaths)
	}
}

func TestMergeSelectionSnapshotSkipsMissingPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &State{SelectedPaths: map[string]bool{keep: true}}
	s.MergeSelectionSnapshot([]string{keep, filepath.Join(dir, "missing.txt")}, nil, "")
	if s.SelectedPathCount() != 1 {
		t.Fatalf("count = %d, want 1", s.SelectedPathCount())
	}
}

func TestStashClear(t *testing.T) {
	t.Parallel()
	s := &State{SelectionStashPaths: []string{"/x"}}
	s.StashClear()
	if !s.StashEmpty() {
		t.Fatal("stash should be empty after StashClear")
	}
}

func TestStashSaveFromSelectionMergesIntoStash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	aPath := filepath.Join(dir, "orchard.txt")
	bPath := filepath.Join(dir, "pebble.txt")
	s := &State{SelectedPaths: map[string]bool{aPath: true}, SelectedStripOrder: []string{aPath}}
	s.StashSaveFromSelection()
	s.SelectedPaths = map[string]bool{aPath: true, bPath: true}
	s.SelectedStripOrder = []string{bPath, aPath}
	if !s.StashSaveFromSelection() {
		t.Fatal("StashSaveFromSelection() = false, want true")
	}
	if got := s.SelectionStashPaths; len(got) != 2 || got[0] != aPath || got[1] != bPath {
		t.Fatalf("stash paths = %v, want [%s %s]", got, aPath, bPath)
	}
	if got := s.SelectionStashStripOrder; len(got) != 2 || got[0] != aPath || got[1] != bPath {
		t.Fatalf("stash strip = %v, want stash order first", got)
	}
	if s.SelectedPathCount() != 0 {
		t.Fatalf("live selection = %d, want 0", s.SelectedPathCount())
	}
}
