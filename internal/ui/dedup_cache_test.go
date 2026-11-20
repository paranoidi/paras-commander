package ui

import (
	"slices"
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// dedupRowIDs lists the node IDs of rows.
func dedupRowIDs(rows []DedupRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

// TestDedupCacheInvalidates drives one shared cache through every key and checks
// each answer against an uncached view.
func TestDedupCacheInvalidates(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "walnut/ember/quilt", "pepper/quilt"),
			dedupTestGroup(2, 0, "walnut/ember/lantern", "pepper/lantern"),
			dedupTestGroup(3, 30, "walnut/meadow/anchor", "harbor/anchor"),
		},
	}
	view := DedupViewState{TreeDirs: true, IgnoreEmpty: true, Cache: &DedupCache{}}
	check := func(step string, snap comparepkg.DedupSnapshot, view DedupViewState) {
		t.Helper()
		bare := view
		bare.Cache = nil
		got, gotEmpty := DedupRowsFromSnapshot(snap, view)
		want, wantEmpty := DedupRowsFromSnapshot(snap, bare)
		if !slices.Equal(dedupRowIDs(got), dedupRowIDs(want)) || gotEmpty != wantEmpty {
			t.Fatalf("%s: cached rows %v (%d empty) != direct %v (%d empty)",
				step, dedupRowIDs(got), gotEmpty, dedupRowIDs(want), wantEmpty)
		}
		if g, w := DedupExpandableIDs(snap, view), DedupExpandableIDs(snap, bare); !slices.Equal(g, w) {
			t.Fatalf("%s: cached expandable IDs %v != direct %v", step, g, w)
		}
	}

	check("first build", snap, view)
	rows, _ := DedupRowsFromSnapshot(snap, view)

	// Collapsing reuses the tree and still flattens correctly.
	view.Main.SetCollapsed("d:walnut", true)
	collapsed, _ := DedupRowsFromSnapshot(snap, view)
	if len(collapsed) >= len(rows) {
		t.Fatalf("collapsed rows %d, want fewer than %d", len(collapsed), len(rows))
	}
	check("collapsed", snap, view)
	view.Main.SetCollapsed("d:walnut", false)

	view.IgnoreEmpty = false
	check("ignore-empty off", snap, view)
	view.IgnoreEmpty = true
	check("ignore-empty on", snap, view)

	view.TreeDirs = false
	check("groups mode", snap, view)
	view.SortByWasted = true
	check("groups sorted by wasted", snap, view)
	view.TreeDirs = true
	check("dirs mode again", snap, view)

	view.GroupFilter = func(g comparepkg.DedupGroup) bool { return g.Size == 30 }
	view.GroupFilterRev++
	check("group filter", snap, view)
	if n := len(DedupExpandableIDs(snap, view)); n == 0 {
		t.Fatal("filtered tree has no expandable IDs")
	}
	view.GroupFilter = nil
	view.GroupFilterRev++
	check("filter cleared", snap, view)

	// A replaced snapshot (fewer groups) must not serve the old tree.
	snap2 := snap.WithoutPaths(map[string]bool{"/root/harbor/anchor": true})
	check("snapshot replaced", snap2, view)
	if got, _ := DedupRowsFromSnapshot(snap2, view); len(got) >= len(rows) {
		t.Fatalf("rows after WithoutPaths = %d, want fewer than %d", len(got), len(rows))
	}

	// Mark sets follow MarksRev.
	if got := dedupMarkSetsFor(snap, view).marked; got != nil {
		t.Fatalf("no marks: dirs = %v, want nil", got)
	}
	view.Marked = map[string]bool{"/root/walnut/ember/quilt": true}
	view.MarksRev++
	if got := dedupMarkSetsFor(snap, view).marked; !got["walnut"] || !got["walnut/ember"] || len(got) != 2 {
		t.Fatalf("after mark: dirs = %v, want walnut + walnut/ember", got)
	}
	view.Kept = map[string]bool{"/root/harbor/anchor": true}
	view.MarksRev++
	if got := dedupMarkSetsFor(snap, view).kept; !got["harbor"] {
		t.Fatalf("after keep: kept dirs = %v, want harbor", got)
	}
	view.Marked, view.Kept = nil, nil
	view.MarksRev++
	if got := dedupMarkSetsFor(snap, view).marked; got != nil {
		t.Fatalf("after clear: dirs = %v, want nil", got)
	}

	// Column widths are cached per row slice and follow a replaced slice.
	sizeW, countW := view.Cache.columnWidths(rows)
	if wantS, wantC := dedupListColumnWidths(rows); sizeW != wantS || countW != wantC {
		t.Fatalf("widths = %d,%d want %d,%d", sizeW, countW, wantS, wantC)
	}
	if s, c := view.Cache.columnWidths(rows[:1]); s == 0 || c == 0 {
		t.Fatalf("widths for a shorter slice = %d,%d", s, c)
	}
}

// TestDedupMainWidthsSurviveCollapse checks the main-pane widths belong to the
// cached tree: a collapse toggle re-flattens but reuses the same computed widths.
func TestDedupMainWidthsSurviveCollapse(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			dedupTestGroup(1, 10, "walnut/ember/quilt", "pepper/quilt"),
		},
	}
	view := DedupViewState{TreeDirs: true, Cache: &DedupCache{}}
	tree := dedupTreeFor(snap, view)
	s, c := tree.columnWidths()
	view.Main.SetCollapsed(tree.expandableIDs()[0], true)
	if dedupTreeFor(snap, view) != tree {
		t.Fatal("collapse toggle rebuilt the tree")
	}
	collapsed, _ := DedupRowsFromSnapshot(snap, view)
	if s2, c2 := tree.columnWidths(); s2 != s || c2 != c || !tree.widthsDone {
		t.Fatalf("widths changed: %d,%d -> %d,%d", s, c, s2, c2)
	}
	all, _ := DedupRowsFromSnapshot(snap, DedupViewState{TreeDirs: true})
	if wantS, wantC := dedupListColumnWidths(all); s != wantS || c != wantC || len(collapsed) >= len(all) {
		t.Fatalf("widths %d,%d want %d,%d over the fully expanded rows", s, c, wantS, wantC)
	}
}
