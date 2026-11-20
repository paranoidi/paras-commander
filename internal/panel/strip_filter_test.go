package panel

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestStripFilterMatchesBasenameAndCycles(t *testing.T) {
	state := State{
		Path: pathloc.MustParse("/tmp/harbor"),
		SelectedPaths: map[string]bool{
			"/tmp/meadow/bravo.txt":   true,
			"/tmp/meadow/crystal.txt": true,
			"/tmp/orchard/delta.txt":  true,
		},
		FilterOptions: ciFilterOptions,
	}
	if n := state.SelectedStripCount(); n != 3 {
		t.Fatalf("strip count = %d, want 3 (paths=%v)", n, state.SelectedStripPaths())
	}

	state.AppendStripFilterRune('c', 10)
	if !state.Strip.Filter.Active || !state.Strip.Filter.HasMatches() {
		t.Fatalf("want active strip filter with matches, got %+v results=%d", state.Strip.Filter, state.Strip.Filter.MatchCount())
	}
	path, ok := state.SelectedPathAtStripIndex(state.Strip.Cursor)
	if !ok || filepath.Base(path) != "crystal.txt" {
		t.Fatalf("cursor path = %q ok=%v, want crystal.txt", path, ok)
	}

	state.AppendStripFilterRune('r', 10) // "cr" still crystal
	path, ok = state.SelectedPathAtStripIndex(state.Strip.Cursor)
	if !ok || filepath.Base(path) != "crystal.txt" {
		t.Fatalf("after 'cr' path = %q, want crystal.txt", path)
	}

	state.CancelStripFilter(10)
	if state.Strip.Filter.Active || state.Strip.Filter.Editing || state.Strip.Filter.Query != "" {
		t.Fatalf("after cancel StripFilter = %+v", state.Strip.Filter)
	}
}

func TestStripCursorRememberRestore(t *testing.T) {
	paths := []string{"/tmp/meadow/bravo.txt", "/tmp/meadow/crystal.txt", "/tmp/orchard/delta.txt"}
	mk := func(ps []string) State {
		st := State{Path: pathloc.MustParse("/tmp/harbor"), SelectedPaths: map[string]bool{}}
		for _, p := range ps {
			st.AddSelection(p)
		}
		return st
	}
	st := mk(paths)
	st.Strip.Cursor = 2
	want, _ := st.SelectedPathAtStripIndex(2)
	st.RememberStripCursor()
	st.ClearSelection()
	if st.Strip.Cursor != 0 || st.Strip.CursorPath != "" {
		t.Fatalf("after clear: cursor %d path %q", st.Strip.Cursor, st.Strip.CursorPath)
	}
	for _, p := range paths {
		st.AddSelection(p)
	}
	st.Strip.CursorPath = want
	st.RestoreStripCursor()
	if got, _ := st.SelectedPathAtStripIndex(st.Strip.Cursor); got != want {
		t.Fatalf("restored %q, want %q", got, want)
	}

	st.Strip.CursorPath = "/tmp/gone/missing.txt"
	st.Strip.Cursor = 1
	st.RestoreStripCursor()
	if st.Strip.Cursor != 1 {
		t.Fatalf("cursor moved to %d for missing path", st.Strip.Cursor)
	}
}

func TestStripTreeFolderRowUnselectsSubtree(t *testing.T) {
	st := State{Path: pathloc.MustParse("/tmp/harbor"), SelectedPaths: map[string]bool{}}
	keep := "/tmp/orchard/delta.txt"
	for _, p := range []string{"/tmp/meadow/quartz/bravo.txt", "/tmp/meadow/quartz/crystal.txt", "/tmp/meadow/willow/ember.txt", keep} {
		st.AddSelection(p)
	}
	st.Strip.Tree = true
	rows := st.SelectedStripRows()
	var meadow = -1
	for i, r := range rows {
		if r.Group && r.Path == "/tmp/meadow" {
			meadow = i
		}
	}
	if meadow < 0 || rows[meadow].Display != "meadow" {
		t.Fatalf("no meadow folder row in %+v", rows)
	}
	if n := st.SelectedStripCount(); n != len(rows) || n <= len(st.SelectedPaths) {
		t.Fatalf("tree strip count = %d, rows %d, selections %d: folders must add rows", n, len(rows), len(st.SelectedPaths))
	}
	st.Strip.Cursor = meadow
	if !st.ToggleOrRemoveStripSelection() {
		t.Fatal("ToggleOrRemoveStripSelection on a folder row = false")
	}
	if len(st.SelectedPaths) != 1 || !st.SelectedPaths[keep] {
		t.Fatalf("selections after folder unselect = %v, want only %s", st.SelectedPaths, keep)
	}
	for _, r := range st.SelectedStripRows() {
		if r.Path != keep && !r.Group {
			t.Fatalf("stale row %+v after folder unselect", r)
		}
	}
}
