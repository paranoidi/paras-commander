package panel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// setupPatternFilterState creates a directory with three files (one matching a pattern by
// basename, one only via a meta column value, one matching neither) and returns the loaded state
// plus the meta-matching file's absolute path.
func setupPatternFilterState(t *testing.T) (State, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"thicket.txt", "lantern.txt", "sesame.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return state, filepath.Join(dir, "lantern.txt")
}

func TestPatternFilterIncludeMetaMatchesByNameOrValue(t *testing.T) {
	state, lanternPath := setupPatternFilterState(t)
	meta := func() GroupSelectMeta {
		return GroupSelectMeta{Cols: []map[string]string{{lanternPath: "beacon tag"}}}
	}
	filter, err := PatternFilter("thicket", GroupPatternSimple, false, false, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	if filter.ID != PatternMetaFilterID {
		t.Fatalf("ID = %q, want %q", filter.ID, PatternMetaFilterID)
	}
	state.SetEntryFilter(filter)
	// thicket.txt matches by basename; lantern.txt only via its meta value — but that value
	// doesn't contain "thicket", so only thicket.txt should be visible.
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}

	// Now match a pattern that only the meta value satisfies: with Include on, basename matching
	// still runs (finds nothing) and the meta value match adds lantern.txt.
	filter, err = PatternFilter("beacon", GroupPatternSimple, false, false, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok || entry.Name != "lantern.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want lantern.txt", entry, ok)
	}
}

func TestPatternFilterOnlyMetaSkipsBasenameMatch(t *testing.T) {
	state, lanternPath := setupPatternFilterState(t)
	meta := func() GroupSelectMeta {
		return GroupSelectMeta{Cols: []map[string]string{{lanternPath: "beacon tag"}}, OnlyMeta: true}
	}
	// "thicket" matches thicket.txt's basename, but OnlyMeta suppresses basename matching
	// entirely, so only meta-value matches count — here, none.
	filter, err := PatternFilter("thicket", GroupPatternSimple, false, false, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 0 {
		t.Fatalf("VisibleEntryCount = %d, want 0", got)
	}

	// "beacon" matches only lantern.txt's meta value.
	filter, err = PatternFilter("beacon", GroupPatternSimple, false, false, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok || entry.Name != "lantern.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want lantern.txt", entry, ok)
	}
}

// TestPatternFilterRefreshPicksUpLiveProviderData simulates meta results arriving
// asynchronously: the meta closure re-reads a variable that gets swapped (not mutated in place,
// mirroring MetaResults being replaced wholesale) after the filter is already installed, and
// RefreshEntryFilter must pick up the change without reinstalling the filter.
func TestPatternFilterRefreshPicksUpLiveProviderData(t *testing.T) {
	state, lanternPath := setupPatternFilterState(t)

	var live map[string]string // nil until the "provider" resolves
	meta := func() GroupSelectMeta {
		return GroupSelectMeta{Cols: []map[string]string{live}}
	}
	filter, err := PatternFilter("beacon", GroupPatternSimple, false, false, meta)
	if err != nil {
		t.Fatalf("PatternFilter() error = %v", err)
	}
	state.SetEntryFilter(filter)
	if got := state.VisibleEntryCount(); got != 0 {
		t.Fatalf("VisibleEntryCount before data arrives = %d, want 0", got)
	}

	live = map[string]string{lanternPath: "beacon tag"}
	state.RefreshEntryFilter()
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount after RefreshEntryFilter = %d, want 1", got)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok || entry.Name != "lantern.txt" {
		t.Fatalf("VisibleEntry(0) = %+v, ok=%v, want lantern.txt", entry, ok)
	}
}

func TestEntryFilterKeepsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "harbor"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"match.go", "other.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pattern := func(pat string, dirsOnly bool) *EntryFilter {
		f, err := PatternFilter(pat, GroupPatternShell, false, dirsOnly, nil)
		if err != nil {
			t.Fatalf("PatternFilter() error = %v", err)
		}
		return f
	}
	tests := []struct {
		name   string
		filter *EntryFilter
		want   []string
	}{
		{"pattern keeps dir", pattern("*.go", false), []string{"harbor", "match.go"}},
		{"dirs only filters dirs", pattern("harb*", true), []string{"harbor"}},
		{"reject-all keeps dir", &EntryFilter{ID: "none", Match: func(localfs.Entry, *State) bool { return false }}, []string{"harbor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := New(dir)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			state.SetEntryFilter(tt.filter)
			var got []string
			for i := 0; i < state.VisibleEntryCount(); i++ {
				if e, _, ok := state.VisibleEntry(i); ok && e.Name != ".." {
					got = append(got, e.Name)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("visible = %v, want %v", got, tt.want)
			}
		})
	}
}
