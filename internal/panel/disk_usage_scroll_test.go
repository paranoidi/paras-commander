package panel

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// diskSortedDirs returns 15 directories with cached totals, ordered largest first, and the
// mutable size cache behind them.
func diskSortedDirs(cursor, scroll int) (*State, map[string]int64) {
	entries := make([]localfs.Entry, 15)
	sizes := map[string]int64{}
	for i := range entries {
		name := "folder" + strconv.Itoa(i)
		p := filepath.Join("/tmp", name)
		entries[i] = localfs.Entry{Name: name, Path: p, Type: localfs.EntryDirectory}
		sizes[p] = int64(1000 - i)
	}
	state := &State{
		Path:               pathloc.MustParse("/tmp"),
		Entries:            entries,
		Sort:               SortState{Mode: SortName, DiskUsageIdleSizeSort: true},
		IdleDiskTotalsSort: true,
		ScrollMode:         ScrollModeEdge,
		ScrollEdgeMargin:   1,
		DiskSorter: func(abs string) (int64, bool) {
			v, ok := sizes[filepath.Clean(abs)]
			return v, ok
		},
	}
	state.ApplySort()
	state.Cursor, state.ScrollOffset = cursor, scroll
	return state, sizes
}

// The delete dialog drops the cursor directory's cached total and rescans it: the row must not
// wander to the uncached tail and come back centered.
func TestDiskOrderingHoldsViewWhileTotalIsRescanned(t *testing.T) {
	const viewportRows = 5
	state, sizes := diskSortedDirs(4, 2)
	key := state.Entries[4].Path
	total := sizes[key]

	delete(sizes, key)
	state.RefreshDiskUsageOrdering(viewportRows)
	if state.Cursor != 4 || state.ScrollOffset != 2 {
		t.Fatalf("while rescanning: cursor=%d scroll=%d, want 4 and 2", state.Cursor, state.ScrollOffset)
	}

	sizes[key] = total
	state.RefreshDiskUsageOrdering(viewportRows)
	if state.Cursor != 4 || state.ScrollOffset != 2 {
		t.Fatalf("after rescan: cursor=%d scroll=%d, want 4 and 2", state.Cursor, state.ScrollOffset)
	}
}

// A cursor on the last row is not "centered": a reorder moving it up must not center it.
func TestDiskOrderingDoesNotCenterCursorFromLastRow(t *testing.T) {
	const viewportRows = 5
	state, sizes := diskSortedDirs(14, 10)
	sizes[state.Entries[14].Path] = 995 // ties with folder5, sorts to index 5

	state.RefreshDiskUsageOrdering(viewportRows)
	if state.Cursor != 5 {
		t.Fatalf("cursor = %d, want 5", state.Cursor)
	}
	if row := state.Cursor - state.ScrollOffset; row == viewportRows/2 {
		t.Fatalf("cursor centered: scroll=%d row=%d", state.ScrollOffset, row)
	}
}
