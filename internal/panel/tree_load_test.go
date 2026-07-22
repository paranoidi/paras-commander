package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// setupTreeStateWithOneDir builds a tree-mode State rooted at a temp dir containing a single
// subdirectory ("meadow", holding one file) and one top-level file, mirroring the fixture shape
// used by tree_state_test.go.
func setupTreeStateWithOneDir(t *testing.T) (state *State, dirID string) {
	t.Helper()
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !s.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	// Directories sort first, so the cursor should already be on "meadow".
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		t.Fatalf("CurrentEntry = %+v ok=%v, want directory row", entry, ok)
	}
	return &s, entry.Path
}

// TestSetTreeNodeExpandedSyncFallbackWhenNoScheduler confirms the nil-scheduler synchronous path
// (ScheduleTreeChildLoad unset) still expands a directory in one call, exactly as Phase 1 did —
// the async dispatch added in setTreeNodeExpanded must not break existing test/unwired callers.
func TestSetTreeNodeExpandedSyncFallbackWhenNoScheduler(t *testing.T) {
	s, _ := setupTreeStateWithOneDir(t)
	if s.ScheduleTreeChildLoad != nil {
		t.Fatal("expected nil ScheduleTreeChildLoad by default")
	}
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if got := s.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after sync expand = %d, want 3 (2 top-level + 1 child)", got)
	}
}

// TestApplyTreeChildLoadSuccessExpandsDirectory drives the async path end to end: expanding
// dispatches a request (captured, not actually run on a goroutine) and leaves the directory
// collapsed-but-loading; delivering a successful ApplyTreeChildLoad then expands it and shows the
// children, mirroring what the app-layer callback does on the main thread.
func TestApplyTreeChildLoadSuccessExpandsDirectory(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	var captured *TreeChildLoadRequest
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool {
		captured = &req
		return true
	}

	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if captured == nil {
		t.Fatal("ScheduleTreeChildLoad was not invoked")
	}
	if captured.DirID != dirID {
		t.Fatalf("captured.DirID = %q, want %q", captured.DirID, dirID)
	}
	if s.TreeExpanded[dirID] {
		t.Fatal("TreeExpanded must stay false until the async load succeeds")
	}
	if node := findTreeNode(s.TreeRoots, dirID); node == nil || !node.Value.Loading {
		t.Fatal("node must be marked Loading immediately after dispatch")
	}
	if got := s.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount while loading = %d, want 2 (still collapsed)", got)
	}

	entries := []localfs.Entry{{Name: "harbor.txt", Path: filepath.Join(dirID, "harbor.txt"), Type: localfs.EntryFile}}
	if !s.ApplyTreeChildLoad(dirID, entries, nil, 10) {
		t.Fatal("ApplyTreeChildLoad returned false, want true")
	}
	if !s.TreeExpanded[dirID] {
		t.Fatal("TreeExpanded must be true after a successful async load")
	}
	if node := findTreeNode(s.TreeRoots, dirID); node == nil || node.Value.Loading {
		t.Fatal("node must no longer be Loading after apply")
	}
	if got := s.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after successful apply = %d, want 3 (2 top-level + 1 child)", got)
	}
}

// TestApplyTreeChildLoadFailureLeavesCollapsedAndRetries verifies a failed async load clears
// Loading, records LoadErr, and leaves TreeExpanded unset — and that a subsequent expand attempt
// retries by dispatching a new request, since Children is still nil. No separate retry-detection
// mechanism should be needed.
func TestApplyTreeChildLoadFailureLeavesCollapsedAndRetries(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	dispatches := 0
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool {
		dispatches++
		return true
	}

	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if dispatches != 1 {
		t.Fatalf("dispatches after first expand = %d, want 1", dispatches)
	}

	loadErr := os.ErrPermission
	if s.ApplyTreeChildLoad(dirID, nil, loadErr, 10) != true {
		t.Fatal("ApplyTreeChildLoad (failure) returned false, want true")
	}
	if s.TreeExpanded[dirID] {
		t.Fatal("TreeExpanded must stay false after a failed async load")
	}
	node := findTreeNode(s.TreeRoots, dirID)
	if node == nil || node.Value.Loading {
		t.Fatal("node must no longer be Loading after a failed apply")
	}
	if node.Value.LoadErr != loadErr {
		t.Fatalf("node.Value.LoadErr = %v, want %v", node.Value.LoadErr, loadErr)
	}
	if node.Children != nil {
		t.Fatal("Children must stay nil after a failed load, so retry is free")
	}

	// Re-expand: since Children is still nil and TreeExpanded is false, this must retry.
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow (retry): %v", err)
	}
	if dispatches != 2 {
		t.Fatalf("dispatches after retry = %d, want 2", dispatches)
	}
}

// TestExpandWhileLoadingDoesNotDispatchTwice verifies a second expand press on a node that's
// already mid-load is a no-op with respect to dispatch (no duplicate concurrent fetch).
func TestExpandWhileLoadingDoesNotDispatchTwice(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	dispatches := 0
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool {
		dispatches++
		return true
	}

	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow (1st): %v", err)
	}
	if err := s.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand (2nd, while loading): %v", err)
	}
	if dispatches != 1 {
		t.Fatalf("dispatches after second expand attempt while loading = %d, want 1", dispatches)
	}
	if s.TreeExpanded[dirID] {
		t.Fatal("TreeExpanded must still be false while loading")
	}
}

// TestApplyTreeChildLoadStaleDropsSilently simulates a callback arriving after the panel already
// navigated away (ApplyListing re-roots TreeRoots per the existing bug-fix #2 behavior, wiping
// the node the stale callback refers to): ApplyTreeChildLoad must return false and not panic.
func TestApplyTreeChildLoadStaleDropsSilently(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool { return true }

	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}

	// Simulate navigating away to an unrelated directory: ApplyListing re-roots TreeRoots while
	// in tree mode (bug fix #2), which drops the loading node (and its whole former subtree)
	// entirely, since the new TreeRoots is seeded fresh from the new directory's entries.
	other := t.TempDir()
	otherLoc, err := pathloc.File(other)
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}
	if err := s.ApplyListing(otherLoc, nil, "", 10, 0, false); err != nil {
		t.Fatalf("ApplyListing: %v", err)
	}
	if findTreeNode(s.TreeRoots, dirID) != nil {
		t.Fatal("test setup: expected dirID node to be gone after navigating away")
	}

	if s.ApplyTreeChildLoad(dirID, nil, nil, 10) {
		t.Fatal("ApplyTreeChildLoad for a stale/vanished node returned true, want false")
	}
	if s.TreeExpanded[dirID] {
		t.Fatal("stale apply must not set TreeExpanded")
	}
}

// TestApplyTreeChildLoadSortsChildrenByPanelSort confirms children delivered out of on-disk
// order come back sorted per the panel's SortState (here: name, reversed) once applied — the
// Phase 3 fix for ApplyTreeChildLoad wrapping entries straight into nodes with no sort pass.
func TestApplyTreeChildLoadSortsChildrenByPanelSort(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	s.Sort = SortState{Mode: SortName, Reverse: true}
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool { return true }
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}

	// Deliver children in an order that would only look "reverse-name-sorted" after the fix.
	entries := []localfs.Entry{
		{Name: "alpha.txt", Path: filepath.Join(dirID, "alpha.txt"), Type: localfs.EntryFile},
		{Name: "zebra.txt", Path: filepath.Join(dirID, "zebra.txt"), Type: localfs.EntryFile},
		{Name: "mango.txt", Path: filepath.Join(dirID, "mango.txt"), Type: localfs.EntryFile},
	}
	if !s.ApplyTreeChildLoad(dirID, entries, nil, 10) {
		t.Fatal("ApplyTreeChildLoad returned false, want true")
	}

	node := findTreeNode(s.TreeRoots, dirID)
	if node == nil {
		t.Fatal("dirID node not found after load")
	}
	got := make([]string, len(node.Children))
	for i, c := range node.Children {
		got[i] = c.Value.Entry.Name
	}
	want := []string{"zebra.txt", "mango.txt", "alpha.txt"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Children[%d] = %q, want %q (got order %v)", i, got[i], want[i], got)
		}
	}
}

// TestApplyTreeChildLoadIgnoresDiskUsagePrimarySort confirms tree-mode children are never sorted
// by cached disk totals even when the panel's flat-mode idle disk-usage sort is fully armed
// (Sort.DiskUsageIdleSizeSort + IdleDiskTotalsSort both true) — useDiskPrimary is forced false
// for ApplyTreeChildLoad's SortEntries call regardless of the panel's own sort state.
func TestApplyTreeChildLoadIgnoresDiskUsagePrimarySort(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	s.Sort = SortState{Mode: SortName, DiskUsageIdleSizeSort: true}
	s.IdleDiskTotalsSort = true
	s.DiskSorter = func(absPath string) (int64, bool) {
		// Disk totals disagree with name order: zebra.txt "largest" so would sort first
		// if disk-primary sort were (wrongly) applied to tree children.
		if filepath.Base(absPath) == "zebra.txt" {
			return 9999, true
		}
		return 1, true
	}
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool { return true }
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}

	entries := []localfs.Entry{
		{Name: "zebra.txt", Path: filepath.Join(dirID, "zebra.txt"), Type: localfs.EntryFile},
		{Name: "alpha.txt", Path: filepath.Join(dirID, "alpha.txt"), Type: localfs.EntryFile},
	}
	if !s.ApplyTreeChildLoad(dirID, entries, nil, 10) {
		t.Fatal("ApplyTreeChildLoad returned false, want true")
	}

	node := findTreeNode(s.TreeRoots, dirID)
	if node == nil {
		t.Fatal("dirID node not found after load")
	}
	got := make([]string, len(node.Children))
	for i, c := range node.Children {
		got[i] = c.Value.Entry.Name
	}
	want := []string{"alpha.txt", "zebra.txt"} // name order, disk totals ignored
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Children[%d] = %q, want %q (disk-primary sort must not apply to tree children)", i, got[i], want[i])
		}
	}
}

// TestApplyTreeChildLoadSchedulesGitStatusForChildren confirms a successful child load triggers
// ScheduleGitStatus for the newly-loaded children (Phase 3 item 6: git status never populated
// for tree children before this), scoped to just that directory (not a full-tree union), and
// reuses the work-tree root prepareGitColumn cached at cwd-listing time.
func TestApplyTreeChildLoadSchedulesGitStatusForChildren(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	s.GitColumnActive = true
	s.gitWorkRoot = "/fake/repo"
	var captured *GitStatusRequest
	s.ScheduleGitStatus = func(req GitStatusRequest) bool {
		captured = &req
		return true
	}
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool { return true }

	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	entries := []localfs.Entry{{Name: "harbor.txt", Path: filepath.Join(dirID, "harbor.txt"), Type: localfs.EntryFile}}
	if !s.ApplyTreeChildLoad(dirID, entries, nil, 10) {
		t.Fatal("ApplyTreeChildLoad returned false, want true")
	}

	if captured == nil {
		t.Fatal("ScheduleGitStatus was not invoked for newly-loaded tree children")
	}
	if captured.WorkRoot != "/fake/repo" {
		t.Fatalf("captured.WorkRoot = %q, want %q (reused cwd-level work root)", captured.WorkRoot, "/fake/repo")
	}
	if captured.ListDir != dirID {
		t.Fatalf("captured.ListDir = %q, want %q", captured.ListDir, dirID)
	}
	if len(captured.Paths) != 1 || captured.Paths[0].AbsPath != entries[0].Path {
		t.Fatalf("captured.Paths = %+v, want one entry for %q", captured.Paths, entries[0].Path)
	}
}

// TestApplyTreeChildLoadSkipsGitStatusWhenColumnInactive confirms the git-status fetch is
// skipped when GitColumnActive is false, mirroring prepareGitColumn's own guard.
func TestApplyTreeChildLoadSkipsGitStatusWhenColumnInactive(t *testing.T) {
	s, dirID := setupTreeStateWithOneDir(t)
	s.GitColumnActive = false
	called := false
	s.ScheduleGitStatus = func(req GitStatusRequest) bool {
		called = true
		return true
	}
	s.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool { return true }

	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	entries := []localfs.Entry{{Name: "harbor.txt", Path: filepath.Join(dirID, "harbor.txt"), Type: localfs.EntryFile}}
	if !s.ApplyTreeChildLoad(dirID, entries, nil, 10) {
		t.Fatal("ApplyTreeChildLoad returned false, want true")
	}
	if called {
		t.Fatal("ScheduleGitStatus was invoked while GitColumnActive is false")
	}
}
