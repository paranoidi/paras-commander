package ui

import "testing"

func uniqueCacheState() *UniqueViewState {
	return &UniqueViewState{
		Root: "/scan/root", Cache: &UniqueCache{}, Marked: map[string]bool{},
		Files: []UniqueFile{
			{Path: "/scan/root/amber/birch/cedar.bin", Size: 10},
			{Path: "/scan/root/amber/.dune/ember.bin", Size: 20, Unverified: true},
			{Path: "/scan/root/fjord.bin", Size: 300},
		},
	}
}

func TestUniqueCacheTreeAndMarks(t *testing.T) {
	st := uniqueCacheState()
	ids := UniqueExpandableIDs(st)
	roots := st.shown().tree(st)
	if len(ids) == 0 || len(UniqueRows(st)) != 6 {
		t.Fatalf("ids %v rows %d", ids, len(UniqueRows(st)))
	}

	// Collapsing only re-flattens: same shown set, same roots.
	st.Pane.Collapsed = DedupCollapsedSet(ids)
	if rows := UniqueRows(st); len(rows) != 2 {
		t.Fatalf("collapsed rows = %d, want 2", len(rows))
	}
	if sh := st.shown(); &sh.roots[0] != &roots[0] {
		t.Fatal("collapse rebuilt the tree")
	}
	if sizeW, countW := st.uniqueColumnWidths(); sizeW < 3 || countW != 1 {
		t.Fatalf("widths %d %d", sizeW, countW)
	}

	// A hidden-toggle flip changes the shown set and the tree.
	st.HideHidden = true
	if n := len(st.VisibleFiles()); n != 2 || st.UnverifiedCount() != 0 {
		t.Fatalf("hidden: %d files, %d unverified", n, st.UnverifiedCount())
	}
	if sh := st.shown(); &sh.tree(st)[0] == &roots[0] {
		t.Fatal("hide toggle kept the old tree")
	}
	st.HideHidden = false
	if st.UnverifiedCount() != 1 {
		t.Fatal("unverified count not recomputed")
	}

	// A new Files slice invalidates the tree.
	roots = st.shown().tree(st)
	st.Files = append([]UniqueFile(nil), st.Files[:2]...)
	if sh := st.shown(); len(sh.files) != 2 || &sh.tree(st)[0] == &roots[0] {
		t.Fatal("new Files slice kept the old tree")
	}

	// Mark counts follow MarksRev, not the map.
	if got := st.markedUnderDirs()["/scan/root/amber"]; got != 0 {
		t.Fatalf("marks before = %d", got)
	}
	st.SetMark(st.Files[0], true)
	if got := st.markedUnderDirs()["/scan/root/amber"]; got != 1 {
		t.Fatalf("marks after SetMark = %d", got)
	}
	st.Marked = map[string]bool{} // changed without a bump: still the cached counts
	if got := st.markedUnderDirs()["/scan/root/amber"]; got != 1 {
		t.Fatalf("marks recomputed without a MarksRev bump: %d", got)
	}
	st.MarksRev++
	if got := st.markedUnderDirs()["/scan/root/amber"]; got != 0 {
		t.Fatalf("marks after bump = %d", got)
	}
}
