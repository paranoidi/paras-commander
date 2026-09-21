package panel

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func defaultSortState() SortState {
	return SortState{Mode: SortName, Reverse: false, DirectoriesFirst: true}
}

func TestApplyPeriodicRefreshNoOpWhenEntriesEqual(t *testing.T) {
	t0 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	rows := []fsbackend.Entry{
		{Name: "a.txt", Type: fsbackend.EntryFile, Size: 1, ModifiedAt: t0},
	}
	state := State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt", Type: localfs.EntryFile, Size: 1, ModifiedAt: t0}},
		Sort:    defaultSortState(),
		Cursor:  0,
	}
	if applied, err := state.ApplyPeriodicRefresh(state.Path, rows, 5, nil); err != nil || applied {
		if err != nil {
			t.Fatalf("ApplyPeriodicRefresh: %v", err)
		}
		t.Fatal("expected no apply when listing unchanged")
	}
}

func TestApplyPeriodicRefreshKeepsSelectionByNameWhenNewFileAppears(t *testing.T) {
	t0 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	loc := pathloc.MustParse("/tmp")
	state := State{
		Path: loc,
		Entries: []localfs.Entry{
			{Name: "alpha.txt", Path: "/tmp/alpha.txt", ModifiedAt: t0},
			{Name: "beta.txt", Path: "/tmp/beta.txt", ModifiedAt: t0},
		},
		Sort:   defaultSortState(),
		Cursor: 1,
	}
	fresh := []fsbackend.Entry{
		{Name: "alpha.txt", Type: fsbackend.EntryFile, ModifiedAt: t0},
		{Name: "beta.txt", Type: fsbackend.EntryFile, ModifiedAt: t0},
		{Name: "gamma.txt", Type: fsbackend.EntryFile, ModifiedAt: t0},
	}
	applied, err := state.ApplyPeriodicRefresh(loc, fresh, 5, nil)
	if err != nil {
		t.Fatalf("ApplyPeriodicRefresh: %v", err)
	}
	if !applied {
		t.Fatal("expected apply when listing changed")
	}
	ent, ok := state.CurrentEntry()
	if !ok || ent.Name != "beta.txt" {
		name := ""
		if ok {
			name = ent.Name
		}
		t.Fatalf("highlight = %q ok=%v, want beta.txt", name, ok)
	}
}

func TestApplyPeriodicRefreshKeepsCursorRowWhenCursorIndexShifts(t *testing.T) {
	const viewportRows = 5
	t0 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	loc := pathloc.MustParse("/tmp")
	entries := make([]localfs.Entry, 12)
	fresh := make([]fsbackend.Entry, 13)
	for i := 0; i < 12; i++ {
		name := strconv.Itoa(i) + ".dat"
		entries[i] = localfs.Entry{Name: name, Path: "/tmp/" + name, ModifiedAt: t0}
		fresh[i+1] = fsbackend.Entry{Name: name, Type: fsbackend.EntryFile, ModifiedAt: t0}
	}
	// "6x.dat" sorts between "6.dat" and "7.dat" under lexicographic name order, shifting 7.dat down one row.
	fresh[0] = fsbackend.Entry{Name: "6x.dat", Type: fsbackend.EntryFile, ModifiedAt: t0}
	state := State{
		Path:         loc,
		Entries:      entries,
		Sort:         SortState{Mode: SortName, DirectoriesFirst: false},
		Cursor:       7,
		ScrollOffset: 0,
	}
	state.ApplySort()
	for i := 0; i < state.VisibleEntryCount(); i++ {
		entry, _, ok := state.VisibleEntry(i)
		if ok && entry.Name == "7.dat" {
			state.Cursor = i
			break
		}
	}
	state.ScrollOffset = state.Cursor - 3 // highlight on viewport row 3
	priorRow := state.Cursor - state.ScrollOffset
	applied, err := state.ApplyPeriodicRefresh(loc, fresh, viewportRows, nil)
	if err != nil {
		t.Fatalf("ApplyPeriodicRefresh: %v", err)
	}
	if !applied {
		t.Fatal("expected apply")
	}
	ent, ok := state.CurrentEntry()
	if !ok || ent.Name != "7.dat" {
		name := ""
		if ok {
			name = ent.Name
		}
		t.Fatalf("highlight = %q ok=%v, want 7.dat", name, ok)
	}
	if row := state.Cursor - state.ScrollOffset; row != priorRow {
		t.Fatalf("scroll=%d cursor=%d row=%d, want row %d kept after index shift", state.ScrollOffset, state.Cursor, row, priorRow)
	}
}

func TestApplyPeriodicRefreshMinimalScrollWhenHighlightUnchanged(t *testing.T) {
	const viewportRows = 5
	t0 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	loc := pathloc.MustParse("/tmp")
	entries := make([]localfs.Entry, 8)
	fresh := make([]fsbackend.Entry, 8)
	for i := 0; i < 8; i++ {
		name := strconv.Itoa(i) + ".dat"
		entries[i] = localfs.Entry{Name: name, Path: "/tmp/" + name, ModifiedAt: t0}
		fresh[i] = fsbackend.Entry{Name: name, Type: fsbackend.EntryFile, Size: int64(i), ModifiedAt: t0}
	}
	state := State{
		Path:         loc,
		Entries:      entries,
		Sort:         SortState{Mode: SortName, DirectoriesFirst: false},
		Cursor:       4,
		ScrollOffset: 0,
		ScrollMode:   ScrollModeMinimal,
	}
	state.ApplySort()
	state.Move(0, viewportRows)
	priorScroll := state.ScrollOffset
	applied, err := state.ApplyPeriodicRefresh(loc, fresh, viewportRows, nil)
	if err != nil {
		t.Fatalf("ApplyPeriodicRefresh: %v", err)
	}
	if !applied {
		t.Fatal("expected apply when size metadata changed")
	}
	ent, ok := state.CurrentEntry()
	if !ok || ent.Name != "4.dat" {
		t.Fatalf("highlight = %v, want 4.dat", ent.Name)
	}
	if state.Cursor != 4 {
		t.Fatalf("Cursor = %d, want 4", state.Cursor)
	}
	if state.ScrollOffset != priorScroll {
		t.Fatalf("ScrollOffset = %d, want %d (minimal scroll)", state.ScrollOffset, priorScroll)
	}
}

func TestFetchListingClimbToExistingAncestor(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "harbor")
	child := filepath.Join(parent, "pruned")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(child); err != nil {
		t.Fatal(err)
	}

	snap := ListingRefreshSnapshot{
		Loc:                     pathloc.MustParse(child),
		ClimbToExistingAncestor: true,
	}
	entries, loc, _, _, err := FetchListing(t.Context(), snap)
	if err != nil {
		t.Fatalf("FetchListing: %v", err)
	}
	if loc.String() != filepath.Clean(parent) {
		t.Fatalf("listing loc = %q, want ancestor %q", loc.String(), parent)
	}
	found := false
	for _, e := range entries {
		if e.Name == "beacon.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ancestor listing = %v, want beacon.txt", entries)
	}
}

func TestFirstMissingChildName(t *testing.T) {
	root := pathloc.MustParse("/harbor")
	child, err := root.Join("pruned")
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := child.Join("leaf")
	if err != nil {
		t.Fatal(err)
	}
	if got := firstMissingChildName(root, leaf); got != "pruned" {
		t.Fatalf("firstMissingChildName(harbor, leaf) = %q, want pruned", got)
	}
	if got := firstMissingChildName(child, leaf); got != "leaf" {
		t.Fatalf("firstMissingChildName(pruned, leaf) = %q, want leaf", got)
	}
}
