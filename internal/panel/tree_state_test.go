package panel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestToggleTreeExpandCollapseReusesCachedChildren(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount before expand = %d, want 2", got)
	}
	// Directories sort first, so the cursor should already be on the "meadow" row.
	entry, ok := state.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		t.Fatalf("CurrentEntry = %+v ok=%v, want directory row", entry, ok)
	}

	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand (expand): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 4 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 4 (2 top-level + 2 children)", got)
	}

	// Collapse must not drop the cached children.
	state.Cursor = 0
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand (collapse): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after collapse = %d, want 2", got)
	}

	// Remove the on-disk children so a re-fetch would be observable, then re-expand: the
	// cached children must still be shown (no re-read of the now-empty directory).
	if err := os.Remove(filepath.Join(meadow, "harbor.txt")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := os.Remove(filepath.Join(meadow, "willow.txt")); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	state.Cursor = 0
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand (re-expand): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 4 {
		t.Fatalf("VisibleEntryCount after re-expand = %d, want 4 (cached children, not re-fetched)", got)
	}
}

// TestApplyListingRebuildsTreeRowsOnNavigate is the regression test for the tree-mode navigation
// bug: after Left/Right (or Enter) navigation while in tree mode, VisibleEntry rows must reflect
// the new directory's entries, not the tree built from whichever directory tree mode was first
// toggled on in.
func TestApplyListingRebuildsTreeRowsOnNavigate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount in %s = %d, want 1", root, got)
	}

	if err := state.NavigateTo(other, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", other, err)
	}

	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after navigating to %s = %d, want 2 (stale tree rows from %s)", other, got, root)
	}
	entry, _, ok := state.VisibleEntry(0)
	if !ok {
		t.Fatal("VisibleEntry(0) ok = false, want true")
	}
	if entry.Path != filepath.Join(other, "harbor.txt") {
		t.Fatalf("VisibleEntry(0).Path = %q, want harbor.txt under %s (got stale entry from %s)", entry.Path, other, root)
	}
}

// TestApplyListingSortsTreeRootsOnNavigate is the regression test for the ApplyListing
// TreeRoots-reseeded-before-ApplySort ordering bug: depth-0 tree rows must reflect the panel's
// active sort setting after navigating while in tree mode, not raw backend/filesystem order.
// Reverse-name sort is used so a fix-independent alphabetical backend read order (harbor before
// willow) would fail before the fix and pass after.
func TestApplyListingSortsTreeRootsOnNavigate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	state.Sort = SortState{Mode: SortName, Reverse: true}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	if err := state.NavigateTo(other, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", other, err)
	}

	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after navigate = %d, want 2", got)
	}
	entry0, _, ok := state.VisibleEntry(0)
	if !ok {
		t.Fatal("VisibleEntry(0) ok = false, want true")
	}
	entry1, _, ok := state.VisibleEntry(1)
	if !ok {
		t.Fatal("VisibleEntry(1) ok = false, want true")
	}
	if entry0.Name != "willow.txt" || entry1.Name != "harbor.txt" {
		t.Fatalf("VisibleEntry order = [%s, %s], want [willow.txt, harbor.txt] (reverse-name sort applied to depth-0 tree rows)", entry0.Name, entry1.Name)
	}
}

func TestSetListLayoutRoundTripPreservesFlatEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "meadow"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	wantPaths := make([]string, len(state.Entries))
	for i, e := range state.Entries {
		wantPaths[i] = e.Path
	}

	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if !state.SetListLayout(ListLayoutFlat, 10) {
		t.Fatal("SetListLayout(Flat) = false, want true")
	}
	if state.ListLayout != ListLayoutFlat {
		t.Fatalf("ListLayout = %v, want Flat", state.ListLayout)
	}
	if len(state.Entries) != len(wantPaths) {
		t.Fatalf("len(Entries) = %d, want %d", len(state.Entries), len(wantPaths))
	}
	for i, e := range state.Entries {
		if e.Path != wantPaths[i] {
			t.Fatalf("Entries[%d].Path = %q, want %q", i, e.Path, wantPaths[i])
		}
	}
}

func TestSetListLayoutBlockedByCarouselMode(t *testing.T) {
	root := t.TempDir()
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	state.CarouselMode = true
	if state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = true while CarouselMode active, want false")
	}
	if state.ListLayout != ListLayoutFlat {
		t.Fatalf("ListLayout = %v, want Flat (blocked)", state.ListLayout)
	}
}

func TestToggleTreeExpandNoOpOnFileRow(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lantern.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	before := state.VisibleEntryCount()
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand: %v", err)
	}
	if got := state.VisibleEntryCount(); got != before {
		t.Fatalf("VisibleEntryCount changed on file row: got %d, want %d", got, before)
	}
	if len(state.TreeExpanded) != 0 {
		t.Fatalf("TreeExpanded = %v, want empty (no-op on file row)", state.TreeExpanded)
	}
}

func TestExpandTreeCursorRow(t *testing.T) {
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
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	// Directories sort first, so the cursor sits on "meadow".
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow (dir): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 3 (meadow + harbor.txt + beacon.txt)", got)
	}

	// No-op: already expanded.
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow (already expanded): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount unchanged = %d, want 3", got)
	}

	// No-op on a file row.
	state.Cursor = 1 // harbor.txt, meadow's child
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow (file row): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount changed on file row: got %d, want 3", got)
	}
}

func TestCollapseTreeCursorRow(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Outside tree mode: no-op.
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow (outside tree mode): %v", err)
	}

	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	// No-op: row is not expanded yet.
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow (not expanded): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1 (unchanged)", got)
	}

	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 2", got)
	}

	state.Cursor = 0
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow: %v", err)
	}
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount after collapse = %d, want 1", got)
	}
}

// TestCollapseTreeCursorRowJumpsToAndCollapsesParent covers the collapse-or-jump-to-parent
// behavior: when the cursor isn't itself sitting on an expanded directory, CollapseTreeCursorRow
// walks up to the immediate parent and collapses that instead of doing nothing.
func TestCollapseTreeCursorRowJumpsToAndCollapsesParent(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	// rowOf finds the visible row index for path (child listing order is not directories-
	// first, unlike the top-level ApplySort, so tests must look rows up by path, not assume a
	// fixed index).
	rowOf := func(path string) int {
		t.Helper()
		for i := 0; i < state.VisibleEntryCount(); i++ {
			if e, _, ok := state.VisibleEntry(i); ok && e.Path == path {
				return i
			}
		}
		t.Fatalf("row for %s not found among %d visible rows", path, state.VisibleEntryCount())
		return -1
	}

	// Cursor starts on "meadow" (top-level directories sort first). Expand it.
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(meadow): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 4 {
		t.Fatalf("VisibleEntryCount after expanding meadow = %d, want 4", got)
	}

	// Cursor on harbor.txt, a file child of the expanded "meadow": collapsing should jump to
	// and collapse the parent ("meadow"), not no-op.
	state.Cursor = rowOf(filepath.Join(meadow, "harbor.txt"))
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow (file child): %v", err)
	}
	if state.TreeExpanded[meadow] {
		t.Fatal("TreeExpanded[meadow] = true, want false after collapsing via file-child cursor")
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after collapse = %d, want 2 (meadow, beacon.txt)", got)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Path != meadow {
		t.Fatalf("CurrentEntry after collapse = %+v ok=%v, want cursor on meadow", entry, ok)
	}

	// Re-expand meadow, cursor lands back on it.
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(meadow) re-expand: %v", err)
	}
	if got := state.VisibleEntryCount(); got != 4 {
		t.Fatalf("VisibleEntryCount after re-expand = %d, want 4", got)
	}

	// Cursor on "nested", a collapsed nested directory (not itself expanded): collapsing
	// should jump to and collapse its parent ("meadow"), not toggle "nested".
	state.Cursor = rowOf(nested)
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow (nested collapsed dir): %v", err)
	}
	if state.TreeExpanded[meadow] {
		t.Fatal("TreeExpanded[meadow] = true, want false after collapsing via nested-dir cursor")
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after collapse = %d, want 2 (meadow, beacon.txt)", got)
	}
	entry, ok = state.CurrentEntry()
	if !ok || entry.Path != meadow {
		t.Fatalf("CurrentEntry after collapse = %+v ok=%v, want cursor on meadow", entry, ok)
	}

	// Existing behavior preserved: cursor directly on an expanded directory collapses that
	// same directory in place, not its grandparent. Re-expand meadow, then expand nested too.
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(meadow) re-expand 2: %v", err)
	}
	state.Cursor = rowOf(nested)
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(nested): %v", err)
	}
	if got := state.VisibleEntryCount(); got != 5 {
		t.Fatalf("VisibleEntryCount after expanding nested = %d, want 5", got)
	}
	state.Cursor = rowOf(nested) // back on "nested", which is now expanded
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow (nested expanded): %v", err)
	}
	if state.TreeExpanded[nested] {
		t.Fatal("TreeExpanded[nested] = true, want false")
	}
	if !state.TreeExpanded[meadow] {
		t.Fatal("TreeExpanded[meadow] = false, want true (must not collapse the grandparent)")
	}
	entry, ok = state.CurrentEntry()
	if !ok || entry.Path != nested {
		t.Fatalf("CurrentEntry after in-place collapse = %+v ok=%v, want cursor on nested", entry, ok)
	}

	// Depth-0 no-op: cursor on a top-level file with nothing expanded above it.
	before := state.VisibleEntryCount()
	state.Cursor = rowOf(filepath.Join(root, "beacon.txt"))
	beaconIdx := state.Cursor
	wantExpanded := map[string]bool{}
	for k, v := range state.TreeExpanded {
		wantExpanded[k] = v
	}
	if err := state.CollapseTreeCursorRow(10); err != nil {
		t.Fatalf("CollapseTreeCursorRow (depth-0 file): %v", err)
	}
	if state.Cursor != beaconIdx {
		t.Fatalf("Cursor = %d, want unchanged %d (depth-0 no-op)", state.Cursor, beaconIdx)
	}
	if got := state.VisibleEntryCount(); got != before {
		t.Fatalf("VisibleEntryCount = %d, want unchanged %d (depth-0 no-op)", got, before)
	}
	for k, v := range wantExpanded {
		if state.TreeExpanded[k] != v {
			t.Fatalf("TreeExpanded[%s] changed, want unchanged (depth-0 no-op)", k)
		}
	}
}

// TestJumpTreeSiblingDir covers next/prev sibling-directory jumps from a nested file, no-wrap
// at the ends, and no-ops outside tree mode / on a depth-0 file.
func TestJumpTreeSiblingDir(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	bravo := filepath.Join(root, "bravo")
	charlie := filepath.Join(root, "charlie")
	for _, dir := range []string{alpha, bravo, charlie} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir(%s): %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(bravo, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	state.JumpTreeSiblingDir(1, 10) // outside tree mode: no-op

	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	rowOf := func(path string) int {
		t.Helper()
		for i := 0; i < state.VisibleEntryCount(); i++ {
			if e, _, ok := state.VisibleEntry(i); ok && e.Path == path {
				return i
			}
		}
		t.Fatalf("row for %s not found among %d visible rows", path, state.VisibleEntryCount())
		return -1
	}

	state.Cursor = rowOf(bravo)
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(bravo): %v", err)
	}

	// Nested file under bravo → next sibling dir is charlie; prev is alpha.
	state.Cursor = rowOf(filepath.Join(bravo, "harbor.txt"))
	state.JumpTreeSiblingDir(1, 10)
	entry, ok := state.CurrentEntry()
	if !ok || entry.Path != charlie {
		t.Fatalf("after next from nested file: CurrentEntry = %+v ok=%v, want %s", entry, ok, charlie)
	}
	state.Cursor = rowOf(filepath.Join(bravo, "harbor.txt"))
	state.JumpTreeSiblingDir(-1, 10)
	entry, ok = state.CurrentEntry()
	if !ok || entry.Path != alpha {
		t.Fatalf("after prev from nested file: CurrentEntry = %+v ok=%v, want %s", entry, ok, alpha)
	}

	// No wrap past the last / first sibling directory.
	state.Cursor = rowOf(charlie)
	state.JumpTreeSiblingDir(1, 10)
	entry, ok = state.CurrentEntry()
	if !ok || entry.Path != charlie {
		t.Fatalf("next at last sibling: CurrentEntry = %+v ok=%v, want unchanged %s", entry, ok, charlie)
	}
	state.Cursor = rowOf(alpha)
	state.JumpTreeSiblingDir(-1, 10)
	entry, ok = state.CurrentEntry()
	if !ok || entry.Path != alpha {
		t.Fatalf("prev at first sibling: CurrentEntry = %+v ok=%v, want unchanged %s", entry, ok, alpha)
	}

	// Depth-0 file: no parent directory → no-op.
	state.Cursor = rowOf(filepath.Join(root, "beacon.txt"))
	beaconIdx := state.Cursor
	state.JumpTreeSiblingDir(1, 10)
	if state.Cursor != beaconIdx {
		t.Fatalf("Cursor after jump on depth-0 file = %d, want unchanged %d", state.Cursor, beaconIdx)
	}
}

func TestCollapseAllTree(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Outside tree mode: no-op, must not panic or flip layout.
	state.CollapseAllTree(10)

	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 2", got)
	}

	// Manual expand leaves treeExpandAllDepth at 0 → CollapseAllTree clears remaining expansions.
	state.CollapseAllTree(10)
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount after CollapseAllTree = %d, want 1", got)
	}
	if len(state.TreeExpanded) != 0 {
		t.Fatalf("TreeExpanded = %v, want empty after CollapseAllTree", state.TreeExpanded)
	}
}

func TestCollapseAllTreeOneLevelUndoesExpandAll(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	deeper := filepath.Join(nested, "deeper")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deeper, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 1: %v", err)
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 2: %v", err)
	}
	if !state.TreeExpanded[meadow] || !state.TreeExpanded[nested] {
		t.Fatalf("TreeExpanded meadow/nested = %v/%v, want both true", state.TreeExpanded[meadow], state.TreeExpanded[nested])
	}
	if state.TreeExpanded[deeper] {
		t.Fatal("TreeExpanded[deeper] = true, want false before collapse")
	}

	state.CollapseAllTree(10)
	if !state.TreeExpanded[meadow] {
		t.Fatal("TreeExpanded[meadow] = false after one collapse, want true (depth-0 stays)")
	}
	if state.TreeExpanded[nested] {
		t.Fatal("TreeExpanded[nested] = true after one collapse, want false (depth-1 collapsed)")
	}
	if state.treeExpandAllDepth != 1 {
		t.Fatalf("treeExpandAllDepth = %d, want 1", state.treeExpandAllDepth)
	}
}

// TestCollapseAllTreeFullyClearsEverything resets expand-all depth and all expansions in one press.
func TestCollapseAllTreeFullyClearsEverything(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 1: %v", err)
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 2: %v", err)
	}
	state.CollapseAllTreeFully(10)
	if len(state.TreeExpanded) != 0 {
		t.Fatalf("TreeExpanded = %v, want empty", state.TreeExpanded)
	}
	if state.treeExpandAllDepth != 0 {
		t.Fatalf("treeExpandAllDepth = %d, want 0", state.treeExpandAllDepth)
	}
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1", got)
	}
}

func TestCollapseAllTreeCursorLandsOnRootAncestorNotOldIndex(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	rowOf := func(path string) int {
		t.Helper()
		for i := 0; i < state.VisibleEntryCount(); i++ {
			if e, _, ok := state.VisibleEntry(i); ok && e.Path == path {
				return i
			}
		}
		t.Fatalf("row for %s not found among %d visible rows", path, state.VisibleEntryCount())
		return -1
	}

	// Expand meadow, then nested, and put the cursor on the deeply nested file.
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(meadow): %v", err)
	}
	state.Cursor = rowOf(nested)
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow(nested): %v", err)
	}
	state.Cursor = rowOf(filepath.Join(nested, "deep.txt"))

	state.CollapseAllTree(10)

	// Manual expands (treeExpandAllDepth 0) still full-clear: cursor follows "meadow" (the
	// depth-0 ancestor) rather than landing on whatever entry now occupies the old numeric index.
	entry, ok := state.CurrentEntry()
	if !ok || entry.Path != meadow {
		t.Fatalf("CurrentEntry after CollapseAllTree = %+v ok=%v, want cursor on meadow", entry, ok)
	}
}

func TestCollapseAllTreeCursorMovesToCollapsedDepthAncestor(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	deeper := filepath.Join(nested, "deeper")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deeper, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 1: %v", err)
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 2: %v", err)
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 3: %v", err)
	}
	deepFile := filepath.Join(deeper, "deep.txt")
	for i := 0; i < state.VisibleEntryCount(); i++ {
		if e, _, ok := state.VisibleEntry(i); ok && e.Path == deepFile {
			state.Cursor = i
			break
		}
	}
	state.CollapseAllTree(10)
	entry, ok := state.CurrentEntry()
	if !ok || entry.Path != deeper {
		t.Fatalf("CurrentEntry after one-level collapse = %+v ok=%v, want deeper (collapsed-depth ancestor)", entry, ok)
	}
}

func TestExpandAllTreeShallow(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	orchard := filepath.Join(root, "orchard")
	if err := os.Mkdir(orchard, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orchard, "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	// Depth-0: meadow, orchard (2 dirs). Both should expand by one level.
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount before expand-all = %d, want 2", got)
	}

	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow: %v", err)
	}
	// meadow -> nested (1 child), orchard -> ember.txt (1 child): 2 roots + 2 children = 4.
	if got := state.VisibleEntryCount(); got != 4 {
		t.Fatalf("VisibleEntryCount after ExpandAllTreeShallow = %d, want 4 (depth-0 dirs expanded by one level only)", got)
	}
	if !state.TreeExpanded[meadow] {
		t.Fatalf("TreeExpanded[%s] = false, want true", meadow)
	}
	if !state.TreeExpanded[orchard] {
		t.Fatalf("TreeExpanded[%s] = false, want true", orchard)
	}
	// "nested" is a depth-1 directory with its own child ("deep.txt"); it must NOT be
	// auto-expanded by the shallow expand-all.
	if state.TreeExpanded[nested] {
		t.Fatalf("TreeExpanded[%s] = true, want false (shallow expand must not recurse)", nested)
	}
}

// TestExpandAllTreeShallowSecondPressDeepensWholeTree covers Ctrl+Alt+Right pressed twice:
// first expands depth-0 roots; second expands every depth-1 directory across the tree (not
// only under the cursor), without recursing to depth 2 in one press.
func TestExpandAllTreeShallowSecondPressDeepensWholeTree(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	deeper := filepath.Join(nested, "deeper")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deeper, "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	orchard := filepath.Join(root, "orchard")
	grove := filepath.Join(orchard, "grove")
	if err := os.MkdirAll(grove, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(grove, "ember.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow (roots): %v", err)
	}
	// Cursor on expanded meadow — second press must still deepen the whole tree.
	for i := 0; i < state.VisibleEntryCount(); i++ {
		e, _, ok := state.VisibleEntry(i)
		if ok && e.Path == meadow {
			state.Cursor = i
			break
		}
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow (depth 1): %v", err)
	}
	if !state.TreeExpanded[nested] {
		t.Fatalf("TreeExpanded[%s] = false, want true (depth-1 under meadow)", nested)
	}
	if !state.TreeExpanded[grove] {
		t.Fatalf("TreeExpanded[%s] = false, want true (depth-1 under orchard, not only cursor branch)", grove)
	}
	if state.TreeExpanded[deeper] {
		t.Fatalf("TreeExpanded[%s] = true, want false (must not recurse past one level)", deeper)
	}
	cur, _, ok := state.VisibleEntry(state.Cursor)
	if !ok || cur.Path != meadow {
		t.Fatalf("cursor after deepen = %q ok=%v, want meadow", cur.Path, ok)
	}
}

// TestExpandAllTreeShallowDepthLimit caps successive deepen presses at 5, then returns
// ErrExpandAllDepthLimit; CollapseAllTree resets the counter.
func TestExpandAllTreeShallowDepthLimit(t *testing.T) {
	root := t.TempDir()
	// Chain of 6 nested dirs so press 5 can expand depth-4 without running out of levels.
	path := root
	for i := 0; i < 6; i++ {
		path = filepath.Join(path, "layer")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("Mkdir layer %d: %v", i, err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "leaf.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	for i := 0; i < maxExpandAllShallowDepth; i++ {
		if err := state.ExpandAllTreeShallow(10); err != nil {
			t.Fatalf("ExpandAllTreeShallow press %d: %v", i+1, err)
		}
	}
	if err := state.ExpandAllTreeShallow(10); !errors.Is(err, ErrExpandAllDepthLimit) {
		t.Fatalf("press %d err = %v, want ErrExpandAllDepthLimit", maxExpandAllShallowDepth+1, err)
	}
	state.CollapseAllTreeFully(10)
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow after CollapseAllTreeFully: %v", err)
	}
}

// TestExpandAllTreeShallowAsyncCoalescesRedrawAndKeepsCursor covers the async expand-all path:
// N quiet ScheduleTreeChildLoad dispatches must not rebuild/steal cursor on intermediate
// ApplyTreeChildLoad calls — only the last apply rebuilds, and the cursor stays on the anchor.
func TestExpandAllTreeShallowAsyncCoalescesRedrawAndKeepsCursor(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	alpha := filepath.Join(meadow, "alpha")
	bravo := filepath.Join(meadow, "bravo")
	for _, dir := range []string{alpha, bravo} {
		if err := os.MkdirAll(filepath.Join(dir, "child"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	// Sync expand roots so meadow's children are visible.
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow (roots): %v", err)
	}
	for i := 0; i < state.VisibleEntryCount(); i++ {
		e, _, ok := state.VisibleEntry(i)
		if ok && e.Path == meadow {
			state.Cursor = i
			break
		}
	}

	var scheduled []string
	state.ScheduleTreeChildLoad = func(req TreeChildLoadRequest) bool {
		scheduled = append(scheduled, req.DirID)
		return true
	}
	beforeCount := state.VisibleEntryCount()
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow (under meadow): %v", err)
	}
	if len(scheduled) != 2 {
		t.Fatalf("scheduled = %v, want 2 child dirs", scheduled)
	}
	if state.treeExpandQuiet != 2 {
		t.Fatalf("treeExpandQuiet = %d, want 2", state.treeExpandQuiet)
	}
	if got := state.VisibleEntryCount(); got != beforeCount {
		t.Fatalf("VisibleEntryCount while quiet = %d, want %d (no intermediate rebuild)", got, beforeCount)
	}

	alphaEntries := []localfs.Entry{{Name: "child", Path: filepath.Join(alpha, "child"), Type: localfs.EntryDirectory}}
	if state.ApplyTreeChildLoad(alpha, alphaEntries, nil, 10) {
		t.Fatal("first ApplyTreeChildLoad returned true, want false (quiet coalesce)")
	}
	cur, _, ok := state.VisibleEntry(state.Cursor)
	if !ok || cur.Path != meadow {
		t.Fatalf("cursor after first apply = %q ok=%v, want meadow", cur.Path, ok)
	}
	if state.TreeExpanded[bravo] {
		t.Fatal("bravo must still be collapsed until its apply")
	}

	bravoEntries := []localfs.Entry{{Name: "child", Path: filepath.Join(bravo, "child"), Type: localfs.EntryDirectory}}
	if !state.ApplyTreeChildLoad(bravo, bravoEntries, nil, 10) {
		t.Fatal("last ApplyTreeChildLoad returned false, want true (final rebuild)")
	}
	if state.treeExpandQuiet != 0 {
		t.Fatalf("treeExpandQuiet after final = %d, want 0", state.treeExpandQuiet)
	}
	if !state.TreeExpanded[alpha] || !state.TreeExpanded[bravo] {
		t.Fatalf("TreeExpanded alpha/bravo = %v/%v, want both true", state.TreeExpanded[alpha], state.TreeExpanded[bravo])
	}
	cur, _, ok = state.VisibleEntry(state.Cursor)
	if !ok || cur.Path != meadow {
		t.Fatalf("cursor after final apply = %q ok=%v, want meadow", cur.Path, ok)
	}
}

// TestQuickFilterMatchesExpandedTreeChild is the regression test for the quick-filter-does-not-
// search-expanded-leafs bug: rebuildFilter must build its corpus from VisibleEntry/
// VisibleEntryCount (which include flattened tree rows), not from the flat, depth-0-only
// s.Entries slice.
func TestQuickFilterMatchesExpandedTreeChild(t *testing.T) {
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

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	// Cursor starts on "meadow" (directories sort first).
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand: %v", err)
	}

	state.OpenFilter(10)
	for _, r := range "harbor" {
		state.AppendFilterRune(r, 10)
	}

	if !state.FilterHasMatches() {
		t.Fatal("FilterHasMatches() = false, want true for expanded tree child harbor.txt")
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "harbor.txt" {
		t.Fatalf("CurrentEntry() = %+v ok=%v, want harbor.txt (expanded tree child)", entry, ok)
	}
}

// TestSelectGroupMatchesExpandedTreeChild is the regression test for the sibling bug to the
// quick-filter one above: SelectGroup/UnselectGroup/InvertSelection iterated s.Entries directly
// (the flat, depth-0-only list), so a group-select pattern could never match a file only visible
// inside an expanded tree directory. Switching to VisibleEntry/VisibleEntryCount fixes it.
func TestSelectGroupMatchesExpandedTreeChild(t *testing.T) {
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

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	// Cursor starts on "meadow" (directories sort first).
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand: %v", err)
	}

	if _, err := state.SelectGroup("harbor.txt", false, false, true, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatalf("SelectGroup: %v", err)
	}
	harborPath := filepath.Join(meadow, "harbor.txt")
	if !state.SelectedPaths[harborPath] {
		t.Fatalf("SelectedPaths[%q] = false, want true (expanded tree child must be selectable)", harborPath)
	}

	if _, err := state.UnselectGroup("harbor.txt", false, false, true, GroupPatternShell, GroupSelectMeta{}); err != nil {
		t.Fatalf("UnselectGroup: %v", err)
	}
	if state.SelectedPaths[harborPath] {
		t.Fatal("SelectedPaths still true after UnselectGroup on expanded tree child")
	}
}

// TestInvertSelectionCoversExpandedTreeChild confirms InvertSelection toggles selection for
// rows only visible inside an expanded tree directory, not just the depth-0 listing.
func TestInvertSelectionCoversExpandedTreeChild(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand: %v", err)
	}

	state.InvertSelection()

	harborPath := filepath.Join(meadow, "harbor.txt")
	if !state.SelectedPaths[harborPath] {
		t.Fatalf("SelectedPaths[%q] = false after InvertSelection, want true (expanded tree child included)", harborPath)
	}
}

// TestFilterResultsStayValidAfterTreeExpandChangesRowLayout proves rebuildTreeRows keeps an
// active filter's results in sync: expanding "meadow" inserts a new row above "aardvark.txt",
// shifting its row index. Without a rebuildFilter call in rebuildTreeRows, the filter's stale
// Index would keep pointing at the old row position (now occupied by "harbor.txt").
func TestFilterResultsStayValidAfterTreeExpandChangesRowLayout(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "harbor.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "aardvark.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount before expand = %d, want 2 (meadow, aardvark.txt)", got)
	}

	state.OpenFilter(10)
	for _, r := range "aardvark" {
		state.AppendFilterRune(r, 10)
	}
	if !state.FilterHasMatches() {
		t.Fatal("FilterHasMatches() = false before expand, want true")
	}

	// The filter match moved the cursor onto aardvark.txt (a file); move it back to meadow
	// (row 0) so ToggleTreeExpand acts on the directory, not the no-op file row.
	state.Cursor = 0

	// Expanding meadow inserts harbor.txt above aardvark.txt: rows become
	// meadow(0), harbor.txt(1), aardvark.txt(2).
	if err := state.ToggleTreeExpand(10); err != nil {
		t.Fatalf("ToggleTreeExpand: %v", err)
	}
	if got := state.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 3", got)
	}

	if !state.FilterHasMatches() {
		t.Fatal("FilterHasMatches() = false after expand, want true (filter must stay in sync)")
	}
	if ranges := state.MatchRanges(2); len(ranges) == 0 {
		t.Fatal("MatchRanges(2) empty, want match on aardvark.txt's new row after tree rebuild")
	}
	if ranges := state.MatchRanges(1); len(ranges) != 0 {
		t.Fatalf("MatchRanges(1) = %v, want no match on harbor.txt's row (stale filter index)", ranges)
	}
}

// TestNavigateBackRestoresTreeExpansionAndCursor covers per-directory tree expansion + highlight
// recall: expanding a subdirectory and moving the cursor onto a nested file, navigating away, then
// back must restore both the expanded set and the exact highlighted row — not just the row index,
// since tree-mode Cursor indexes treeRows, not Entries.
func TestNavigateBackRestoresTreeExpansionAndCursor(t *testing.T) {
	root := t.TempDir()
	harbor := filepath.Join(root, "harbor")
	if err := os.Mkdir(harbor, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harbor, "willow.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "beacon.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "cinder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	// Directories sort first, so the cursor sits on "harbor".
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if !state.SelectVisibleEntry("willow.txt") {
		t.Fatal("SelectVisibleEntry(willow.txt) = false, want true")
	}

	if err := state.NavigateTo(other, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", other, err)
	}
	if err := state.NavigateTo(root, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", root, err)
	}

	if !state.TreeExpanded[harbor] {
		t.Fatalf("TreeExpanded[%s] = false after navigating back, want true", harbor)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "willow.txt" {
		t.Fatalf("CurrentEntry = %+v ok=%v, want willow.txt", entry, ok)
	}
}

// TestNavigateBackRestoresTreeExpandAllDepth covers persisting the expand-all deepen counter
// per directory: deepen twice, leave, return — next expand-all continues at depth 3, not 1.
func TestNavigateBackRestoresTreeExpandAllDepth(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	nested := filepath.Join(meadow, "nested")
	deeper := filepath.Join(nested, "deeper")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deeper, "leaf.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "cinder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 1: %v", err)
	}
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 2: %v", err)
	}
	if state.treeExpandAllDepth != 2 {
		t.Fatalf("treeExpandAllDepth before leave = %d, want 2", state.treeExpandAllDepth)
	}

	if err := state.NavigateTo(other, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", other, err)
	}
	if err := state.NavigateTo(root, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", root, err)
	}
	if state.treeExpandAllDepth != 2 {
		t.Fatalf("treeExpandAllDepth after return = %d, want 2", state.treeExpandAllDepth)
	}
	if !state.TreeExpanded[meadow] || !state.TreeExpanded[nested] {
		t.Fatalf("TreeExpanded meadow/nested = %v/%v after return, want both true", state.TreeExpanded[meadow], state.TreeExpanded[nested])
	}
	// Third deepen should open deeper, not re-expand roots.
	if err := state.ExpandAllTreeShallow(10); err != nil {
		t.Fatalf("ExpandAllTreeShallow 3 after return: %v", err)
	}
	if !state.TreeExpanded[deeper] {
		t.Fatalf("TreeExpanded[%s] = false after third deepen, want true", deeper)
	}
	if state.treeExpandAllDepth != 3 {
		t.Fatalf("treeExpandAllDepth after third deepen = %d, want 3", state.treeExpandAllDepth)
	}
}

// TestSameDirRefreshPreservesExpansionAndRefetchesChildren covers the same-directory-reload path
// (periodic refresh, post-file-op reload): an expanded directory must stay expanded across a
// refresh, and its children must be re-fetched from disk rather than served from the stale cache
// — a file deleted inside the expanded directory must disappear from the tree.
func TestSameDirRefreshPreservesExpansionAndRefetchesChildren(t *testing.T) {
	root := t.TempDir()
	meadow := filepath.Join(root, "meadow")
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	doomed := filepath.Join(meadow, "quartz.txt")
	if err := os.WriteFile(doomed, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meadow, "salted.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if got := state.VisibleEntryCount(); got != 3 {
		t.Fatalf("VisibleEntryCount after expand = %d, want 3 (meadow + 2 children)", got)
	}

	if err := os.Remove(doomed); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := state.Refresh(10); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if !state.TreeExpanded[meadow] {
		t.Fatal("TreeExpanded[meadow] = false after refresh, want true (still expanded)")
	}
	if got := state.VisibleEntryCount(); got != 2 {
		t.Fatalf("VisibleEntryCount after refresh = %d, want 2 (meadow + salted.txt; quartz.txt re-fetched away)", got)
	}
	if state.SelectVisibleEntry("quartz.txt") {
		t.Fatal("quartz.txt still visible after refresh, want re-fetch to drop it (not served from cache)")
	}
}

// TestReturnToDrasticallyChangedDirectory covers the "drastic content changes" degrade-gracefully
// requirement: returning to a directory whose expanded subdirectory and highlighted file both
// vanished (replaced with unrelated content) must show no stale rows, disregard the vanished
// expansion, and fall back the cursor to the nearest remaining row rather than crashing or
// resurrecting deleted entries.
func TestReturnToDrasticallyChangedDirectory(t *testing.T) {
	root := t.TempDir()
	lagoon := filepath.Join(root, "lagoon")
	if err := os.Mkdir(lagoon, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	nested := filepath.Join(lagoon, "ember.txt")
	if err := os.WriteFile(nested, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "cinder.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}
	if err := state.ExpandTreeCursorRow(10); err != nil {
		t.Fatalf("ExpandTreeCursorRow: %v", err)
	}
	if !state.SelectVisibleEntry("ember.txt") {
		t.Fatal("SelectVisibleEntry(ember.txt) = false, want true")
	}

	if err := state.NavigateTo(other, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", other, err)
	}

	if err := os.RemoveAll(lagoon); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "granite.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := state.NavigateTo(root, "", 10); err != nil {
		t.Fatalf("NavigateTo(%s): %v", root, err)
	}

	// A stale TreeExpanded[lagoon] entry (if any survives) is inert: treeflat.Flatten only
	// consults flags for nodes that exist in the fresh TreeRoots, and lagoon has none.
	if state.SelectVisibleEntry("ember.txt") {
		t.Fatal("ember.txt still selectable after deletion, want it gone from the tree")
	}
	if got := state.VisibleEntryCount(); got != 1 {
		t.Fatalf("VisibleEntryCount = %d, want 1 (granite.txt only, lagoon removed)", got)
	}
	entry, ok := state.CurrentEntry()
	if !ok || entry.Name != "granite.txt" {
		t.Fatalf("CurrentEntry = %+v ok=%v, want granite.txt (nearest-remaining fallback)", entry, ok)
	}
}
