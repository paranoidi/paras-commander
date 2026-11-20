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
		SelectedStripOrder: []string{
			"/tmp/meadow/bravo.txt",
			"/tmp/meadow/crystal.txt",
			"/tmp/orchard/delta.txt",
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
