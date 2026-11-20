package ui

import (
	"fmt"
	"math/rand"
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
	if got := dedupMarkDirs(dedupMarkSetsFor(snap, view), "marked"); got != nil {
		t.Fatalf("no marks: dirs = %v, want nil", got)
	}
	view.Marked = map[string]bool{"/root/walnut/ember/quilt": true}
	view.MarksRev++
	if got := dedupMarkDirs(dedupMarkSetsFor(snap, view), "marked"); !got["walnut"] || !got["walnut/ember"] || len(got) != 2 {
		t.Fatalf("after mark: dirs = %v, want walnut + walnut/ember", got)
	}
	view.Kept = map[string]bool{"/root/harbor/anchor": true}
	view.MarksRev++
	if got := dedupMarkDirs(dedupMarkSetsFor(snap, view), "kept"); !got["harbor"] {
		t.Fatalf("after keep: kept dirs = %v, want harbor", got)
	}
	view.Marked, view.Kept = nil, nil
	view.MarksRev++
	if got := dedupMarkDirs(dedupMarkSetsFor(snap, view), "marked"); got != nil {
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

// TestDedupCacheSharesRowPayloads pins the read-only rule: flattens of one
// cached tree hand out the same payload pointers, so nothing may write
// through a row's Value.
func TestDedupCacheSharesRowPayloads(t *testing.T) {
	snap := comparepkg.DedupSnapshot{
		Phase:  comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{dedupTestGroup(1, 10, "walnut/quilt", "pepper/quilt")},
	}
	view := DedupViewState{TreeDirs: true, Cache: &DedupCache{}}
	a, _ := DedupRowsFromSnapshot(snap, view)
	view.Main.SetCollapsed("d:walnut", true)
	b, _ := DedupRowsFromSnapshot(snap, view)
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("no rows")
	}
	for _, rb := range b {
		i := DedupRowIndexByID(a, rb.ID)
		if i < 0 || a[i].Value != rb.Value {
			t.Fatalf("row %q payload not shared between flattens", rb.ID)
		}
	}
}

// TestDedupMarkSetsIncrementalMatchesRecount applies random mark/keep changes through
// NoteMarkChange and compares every dir's four answers with a fresh recount after each.
func TestDedupMarkSetsIncrementalMatchesRecount(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	words := []string{"meadow", "copper", "anchor", "harbor", "walnut", "ember", "quilt", "lantern", "orchard", "beacon", "willow", "pebble"}
	snap := comparepkg.DedupSnapshot{Phase: comparepkg.DedupDone}
	var abs []string
	for g := range 14 {
		var rels []string
		for range 2 + rng.Intn(3) {
			rel := fmt.Sprintf("%s-%d", words[rng.Intn(len(words))], rng.Intn(1000))
			for range rng.Intn(4) { // 0..3 nested dirs
				rel = words[rng.Intn(4)] + "/" + rel
			}
			rels = append(rels, rel)
			abs = append(abs, "/root/"+rel)
		}
		snap.Groups = append(snap.Groups, dedupTestGroup(byte(g+1), 10, rels...))
	}
	abs = append(abs, "/root/outside/stranger") // not in the snapshot

	view := DedupViewState{Cache: &DedupCache{}, Marked: map[string]bool{}, Kept: map[string]bool{}}
	same := func(step int) {
		t.Helper()
		got := dedupMarkSetsFor(snap, view)
		want := dedupMarkSetsFor(snap, DedupViewState{Marked: view.Marked, Kept: view.Kept}) // nil Cache: fresh recount
		for rel := range want.t.ids {
			if got.marked(rel) != want.marked(rel) || got.danger(rel) != want.danger(rel) ||
				got.kept(rel) != want.kept(rel) || got.full(rel) != want.full(rel) {
				t.Fatalf("step %d dir %q: incremental m=%v d=%v k=%v f=%v, recount m=%v d=%v k=%v f=%v", step, rel,
					got.marked(rel), got.danger(rel), got.kept(rel), got.full(rel),
					want.marked(rel), want.danger(rel), want.kept(rel), want.full(rel))
			}
		}
		for gi := range snap.Groups {
			if got.groupMarked[gi] != want.groupMarked[gi] {
				t.Fatalf("step %d group %d: marked members %d, recount %d", step, gi, got.groupMarked[gi], want.groupMarked[gi])
			}
		}
	}
	same(0)
	base := view.Cache.marks
	sawDanger, sawFull := false, false
	for step := 1; step <= 600; step++ {
		a, keep, on := abs[rng.Intn(len(abs))], rng.Intn(3) == 0, rng.Intn(100) < 65
		set := view.Marked
		if keep {
			set = view.Kept
		}
		if set[a] == on {
			continue
		}
		if on {
			set[a] = true
		} else {
			delete(set, a)
		}
		if step%97 == 0 { // a writer that only bumps MarksRev: counters go stale, the next draw recounts
			view.MarksRev++
			same(step)
			if view.Cache.marks == base {
				t.Fatal("bump without NoteMarkChange must recount")
			}
			base = view.Cache.marks
			continue
		}
		view.NoteMarkChange(snap, a, keep, on)
		same(step)
		if view.Cache.marks != base {
			t.Fatalf("step %d: NoteMarkChange recounted instead of updating in place", step)
		}
		sawDanger = sawDanger || dedupMarkDirs(view.Cache.marks, "danger") != nil
		sawFull = sawFull || dedupMarkDirs(view.Cache.marks, "full") != nil
	}
	if !sawDanger || !sawFull {
		t.Fatalf("sequence never reached a fully-marked group (%v) or dir (%v)", sawDanger, sawFull)
	}
}
