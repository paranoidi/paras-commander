package panel

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTreeModeSortRefreshesVisibleOrderAfterDiskUsageSortDisabled reproduces the reported bug:
// "when disk usage data is available it seems to be always used to sort the items, even when
// user explicitly disables the '[x] Disk usage' from 'Sort order'".
//
// Root cause: in tree mode, VisibleEntry/CurrentEntry read from the cached treeRows (built from
// TreeRoots), not from State.Entries directly (see VisibleEntry in state.go). TreeRoots is only a
// snapshot of Entries taken when tree mode is entered (SetListLayout) or a directory is (re)loaded
// (ApplyListing). Sort-changing operations that flow through ApplySort — ApplySortFromDialog,
// SetSortMode, RefreshDiskUsageOrdering — correctly reorder Entries but, before this fix, never
// resynced TreeRoots/treeRows, so the tree view kept showing whatever order was snapshotted
// earlier (e.g. disk-usage order) even after the user unchecked the box.
func TestTreeModeSortRefreshesVisibleOrderAfterDiskUsageSortDisabled(t *testing.T) {
	root := t.TempDir()
	names := []string{"alpha.txt", "middle.txt", "zulu.txt"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", n, err)
		}
	}

	state, err := New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Fake disk-usage totals that invert name order: zulu has the largest total, alpha the
	// smallest, so a disk-primary sort visibly disagrees with name order.
	totals := map[string]int64{
		filepath.Clean(filepath.Join(root, "alpha.txt")):  1,
		filepath.Clean(filepath.Join(root, "middle.txt")): 100,
		filepath.Clean(filepath.Join(root, "zulu.txt")):   1000,
	}
	state.DiskSorter = func(path string) (int64, bool) {
		v, ok := totals[filepath.Clean(path)]
		return v, ok
	}

	// Turn on disk-usage idle sort and let it actually activate (mirrors what the idle timer
	// does at fire time), then enter tree mode: TreeRoots snapshots the disk-sorted order.
	state.Sort.DiskUsageIdleSizeSort = true
	state.IdleDiskTotalsSort = true
	state.ApplySort()
	if !state.SetListLayout(ListLayoutTree, 10) {
		t.Fatal("SetListLayout(Tree) = false, want true")
	}

	entry0, _, ok := state.VisibleEntry(0)
	if !ok || entry0.Name != "zulu.txt" {
		t.Fatalf("sanity check failed: tree row 0 = %q ok=%v, want zulu.txt (largest disk total first)", entry0.Name, ok)
	}

	// User unchecks "Disk usage" in the Sort dialog: ApplySortFromDialog must both reorder
	// Entries AND make the tree view reflect the new (name) order.
	state.ApplySortFromDialog(SortState{Mode: SortName, DiskUsageIdleSizeSort: false}, 10)

	if state.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort still true after disabling disk-usage sort")
	}

	wantOrder := []string{"alpha.txt", "middle.txt", "zulu.txt"}
	for i, want := range wantOrder {
		e, _, ok := state.VisibleEntry(i)
		if !ok {
			t.Fatalf("VisibleEntry(%d) ok=false", i)
		}
		if e.Name != want {
			t.Fatalf("tree row %d = %q, want %q (visible order stuck in stale disk-usage order)", i, e.Name, want)
		}
	}
}
