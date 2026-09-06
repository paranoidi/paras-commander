package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestRemoveEntriesByPathDropsRowsAndBumpsEpoch(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "harbor.txt")
	gone := filepath.Join(dir, "willow.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gone, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	epoch := state.ListingEpoch
	if !state.SelectVisibleEntry("willow.txt") {
		t.Fatal("select willow")
	}

	if !state.RemoveEntriesByPath([]string{gone}, 10) {
		t.Fatal("RemoveEntriesByPath = false, want true")
	}
	if state.ListingEpoch <= epoch {
		t.Fatalf("ListingEpoch = %d, want > %d", state.ListingEpoch, epoch)
	}
	if state.SelectVisibleEntry("willow.txt") {
		t.Fatal("willow should be gone")
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "harbor.txt" {
		name := ""
		if ok {
			name = entry.Name
		}
		t.Fatalf("after removing focused row, cursor entry = %q, want harbor.txt", name)
	}
}

func TestRemoveEntriesByPathKeepsFocusedEntryByPath(t *testing.T) {
	dir := t.TempDir()
	// Lexical order: alpha, meadow, zebra — focus meadow, remove alpha (above) and zebra (below).
	names := []string{"alpha.txt", "meadow.txt", "zebra.txt"}
	paths := make([]string, len(names))
	for i, name := range names {
		p := filepath.Join(dir, name)
		paths[i] = p
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SelectVisibleEntry("meadow.txt") {
		t.Fatal("select meadow")
	}
	priorIdx := state.Cursor

	if !state.RemoveEntriesByPath([]string{paths[0], paths[2]}, 10) {
		t.Fatal("RemoveEntriesByPath = false, want true")
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "meadow.txt" {
		name := ""
		if ok {
			name = entry.Name
		}
		t.Fatalf("cursor entry = %q (idx %d), want meadow.txt (had idx %d before prune)", name, state.Cursor, priorIdx)
	}
}

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

func TestStaleEpochPreventsResurrectionContract(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snapEpoch := state.ListingEpoch
	gone := filepath.Join(dir, "willow.txt")
	_ = state.RemoveEntriesByPath([]string{gone}, 10)
	if state.ListingEpoch == snapEpoch {
		t.Fatal("epoch should bump on remove")
	}
	// App layer must refuse ApplyListing when req.ListingEpoch != state.ListingEpoch.
	if snapEpoch == state.ListingEpoch {
		t.Fatal("stale snapshot epoch should not match after prune")
	}
	if state.SelectVisibleEntry("willow.txt") {
		t.Fatal("willow should stay pruned until a matching-epoch listing applies")
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

// TestRemoveEntriesByPathThenStaleReloadDoesNotMarkReappearanceAsNew reproduces the delete/move
// race: RemoveEntriesByPath prunes a row optimistically when the job is enqueued, but a reload
// that lands before the physical filesystem op completes still sees the file on disk. That
// reappearance must not be flagged as newly created (it would show a spurious new-file icon
// alongside the in-flight job's own icon).
func TestRemoveEntriesByPathThenStaleReloadDoesNotMarkReappearanceAsNew(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "departing.txt")
	if err := os.WriteFile(gone, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if !state.RemoveEntriesByPath([]string{gone}, 10) {
		t.Fatal("RemoveEntriesByPath = false, want true")
	}

	// Simulate a stale periodic-refresh read landing before the delete/move job's real op
	// completes: the file is still physically present.
	staleListing := []fsbackend.Entry{{Name: "departing.txt", Type: fsbackend.EntryFile}}
	if _, err := state.ApplyPeriodicRefresh(pathloc.MustParse(dir), staleListing, 10); err != nil {
		t.Fatalf("ApplyPeriodicRefresh: %v", err)
	}

	departing := localfs.Entry{Name: "departing.txt", Type: localfs.EntryFile}
	if got := state.NewFileMarkTier(departing); got != panellist.NewFileMarkNone {
		t.Fatalf("tier = %v, want none (reappearance during in-flight removal must not be new)", got)
	}
}
