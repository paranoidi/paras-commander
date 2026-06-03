package panel

import (
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
	if applied, err := state.ApplyPeriodicRefresh(state.Path, rows, 5); err != nil || applied {
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
	applied, err := state.ApplyPeriodicRefresh(loc, fresh, 5)
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

func TestApplyPeriodicRefreshCentersWhenCursorIndexShifts(t *testing.T) {
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
	fresh[0] = fsbackend.Entry{Name: "new.dat", Type: fsbackend.EntryFile, ModifiedAt: t0}
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
	applied, err := state.ApplyPeriodicRefresh(loc, fresh, viewportRows)
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
	if !state.cursorAppearsCentered(viewportRows) {
		t.Fatalf("scroll=%d cursor=%d, want centered after index shift", state.ScrollOffset, state.Cursor)
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
		Path:            loc,
		Entries:         entries,
		Sort:            SortState{Mode: SortName, DirectoriesFirst: false},
		Cursor:          4,
		ScrollOffset:    0,
		CenterScrolling: false,
	}
	state.ApplySort()
	state.Move(0, viewportRows)
	priorScroll := state.ScrollOffset
	applied, err := state.ApplyPeriodicRefresh(loc, fresh, viewportRows)
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
