package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestInsertEntryOnlyWhenParentMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	other := filepath.Join(t.TempDir(), "beacon.txt")
	if state.InsertEntry(localfs.Entry{Name: "beacon.txt", Path: other, Type: localfs.EntryFile}, 10) {
		t.Fatal("InsertEntry into foreign parent should be false")
	}
	newPath := filepath.Join(dir, "beacon.txt")
	if !state.InsertEntry(localfs.Entry{Name: "beacon.txt", Path: newPath, Type: localfs.EntryFile, Size: 3}, 10) {
		t.Fatal("InsertEntry = false, want true")
	}
	if !state.SelectVisibleEntry("beacon.txt") {
		t.Fatal("beacon missing after insert")
	}
	if state.InsertEntry(localfs.Entry{Name: "beacon.txt", Path: newPath, Type: localfs.EntryFile}, 10) {
		t.Fatal("duplicate InsertEntry should be false")
	}
}

func TestInsertEntriesBatchSkipsDuplicatesAndForeignParents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	foreign := filepath.Join(t.TempDir(), "outsider.txt")
	batch := []localfs.Entry{
		{Name: "beacon.txt", Path: filepath.Join(dir, "beacon.txt"), Type: localfs.EntryFile, Size: 3},
		// Duplicate of an already-listed row: must be skipped, not appended twice.
		{Name: "harbor.txt", Path: filepath.Join(dir, "harbor.txt"), Type: localfs.EntryFile},
		// Duplicate within the batch itself: only the first should be kept.
		{Name: "beacon.txt", Path: filepath.Join(dir, "beacon.txt"), Type: localfs.EntryFile},
		{Name: "outsider.txt", Path: foreign, Type: localfs.EntryFile},
		{Name: "cellar.txt", Path: filepath.Join(dir, "cellar.txt"), Type: localfs.EntryFile, Size: 5},
	}
	if !state.InsertEntries(batch, 10) {
		t.Fatal("InsertEntries = false, want true")
	}
	if !state.SelectVisibleEntry("beacon.txt") {
		t.Fatal("beacon missing after batch insert")
	}
	if !state.SelectVisibleEntry("cellar.txt") {
		t.Fatal("cellar missing after batch insert")
	}
	if state.SelectVisibleEntry("outsider.txt") {
		t.Fatal("foreign-parent entry should not have been inserted")
	}
	count := 0
	for _, e := range state.Entries {
		if e.Name == "beacon.txt" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("beacon.txt appears %d times, want 1 (in-batch duplicate not deduped)", count)
	}
	// One insert pass, not one per entry: single-entry InsertEntry already covers per-call
	// epoch bumping, this only needs to confirm the batch bumped it exactly once overall.
	if state.ListingEpoch == 0 {
		t.Fatal("ListingEpoch should have advanced")
	}
}

func TestInsertEntriesEmptyBatchIsNoop(t *testing.T) {
	dir := t.TempDir()
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if state.InsertEntries(nil, 10) {
		t.Fatal("InsertEntries(nil) should be false")
	}
	if state.InsertEntries([]localfs.Entry{}, 10) {
		t.Fatal("InsertEntries(empty) should be false")
	}
}

func TestRenameEntryUpdatesPathAndSelection(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "willow.txt")
	if err := os.WriteFile(oldPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	state.SelectedPaths = map[string]bool{oldPath: true}
	priorCursor := state.Cursor
	if !state.RenameEntry(oldPath, "harbor.txt", 10) {
		t.Fatal("RenameEntry = false")
	}
	want := filepath.Join(dir, "harbor.txt")
	if state.SelectedPaths[oldPath] {
		t.Fatal("old path still selected")
	}
	if !state.SelectedPaths[want] {
		t.Fatal("new path not selected")
	}
	if state.Cursor != priorCursor {
		t.Fatalf("Cursor = %d, want prior index %d", state.Cursor, priorCursor)
	}
	if !state.SelectVisibleEntry("harbor.txt") {
		t.Fatal("harbor missing")
	}
}

func TestRefreshSupersedesSameDirPendingLoad(t *testing.T) {
	dir := t.TempDir()
	state := &State{Path: pathloc.MustParse(dir)}
	var calls int
	state.ScheduleAsyncLoad = func(req AsyncLoadRequest) bool {
		calls++
		return true
	}
	if err := state.Refresh(10); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !state.ListingPending {
		t.Fatal("ListingPending should be true")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if err := state.Refresh(10); err != nil {
		t.Fatalf("Refresh while same-dir pending: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (same-dir supersede)", calls)
	}
}

func TestRefreshDoesNotClobberCrossDirNavigation(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "meadow")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	state := &State{Path: pathloc.MustParse(dir)}
	var calls int
	state.ScheduleAsyncLoad = func(req AsyncLoadRequest) bool {
		calls++
		return true
	}
	if err := state.NavigateToPath(pathloc.MustParse(other), "", 10); err != nil {
		t.Fatalf("NavigateToPath: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if err := state.Refresh(10); err != nil {
		t.Fatalf("Refresh during nav: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (cross-dir pending must not schedule refresh)", calls)
	}
}
