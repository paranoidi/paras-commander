package panel

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/search"
)

func TestStripFilterMatchesBasenameAndCycles(t *testing.T) {
	state := State{
		Path: pathloc.MustParse("/tmp/harbor"),
		SelectedPaths: map[string]bool{
			"/tmp/meadow/bravo.txt":   true,
			"/tmp/meadow/crystal.txt": true,
			"/tmp/orchard/delta.txt":  true,
		},
		SelectionsStripOrder: []string{
			"/tmp/meadow/bravo.txt",
			"/tmp/meadow/crystal.txt",
			"/tmp/orchard/delta.txt",
		},
		StripFilter: FilterState{CaseInsensitive: true},
	}
	if n := state.SelectionsStripCount(); n != 3 {
		t.Fatalf("strip count = %d, want 3 (paths=%v)", n, state.SelectionsStripPaths())
	}

	state.AppendStripFilterRune('c', 10)
	if !state.StripFilter.Active || !state.StripFilterHasMatches() {
		t.Fatalf("want active strip filter with matches, got %+v results=%d", state.StripFilter, len(state.StripFilter.results))
	}
	path, ok := state.SelectedPathAtStripIndex(state.SelectionsStripCursor)
	if !ok || filepath.Base(path) != "crystal.txt" {
		t.Fatalf("cursor path = %q ok=%v, want crystal.txt", path, ok)
	}

	state.AppendStripFilterRune('r', 10) // "cr" still crystal
	path, ok = state.SelectedPathAtStripIndex(state.SelectionsStripCursor)
	if !ok || filepath.Base(path) != "crystal.txt" {
		t.Fatalf("after 'cr' path = %q, want crystal.txt", path)
	}

	state.CancelStripFilter(10)
	if state.StripFilter.Active || state.StripFilter.Editing || state.StripFilter.Query != "" {
		t.Fatalf("after cancel StripFilter = %+v", state.StripFilter)
	}
}

func TestMapStripBasenameRangesToDisplay(t *testing.T) {
	display := "meadow/crystal.txt"
	abs := "/tmp/meadow/crystal.txt"
	mapped := MapStripBasenameRangesToDisplay(display, abs, []search.Range{{Start: 0, End: 3}})
	if len(mapped) != 1 {
		t.Fatalf("mapped = %v, want one range", mapped)
	}
	if mapped[0].Start != len([]rune("meadow/")) || mapped[0].End != len([]rune("meadow/"))+3 {
		t.Fatalf("mapped[0] = %+v, want offset onto basename", mapped[0])
	}
}
