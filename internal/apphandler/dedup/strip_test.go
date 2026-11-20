package dedup

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func stripTestHandler(t *testing.T) (*Handler, *ui.Model) {
	t.Helper()
	snap := dedupDoneSnapshot(pathloc.MustParse("/scan"),
		dedupFile("orchard/lantern.txt"), dedupFile("meadow/quartz/lantern.txt"))
	return dedupHandlerWithView(t, snap, ui.DedupViewState{
		TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{}, Cache: &ui.DedupCache{},
	})
}

func TestStripJoinsFocusCycleOnlyWithMarks(t *testing.T) {
	h, model := stripTestHandler(t)
	st := &model.DedupView
	if h.ToggleStripFocus() {
		t.Fatal("ToggleStripFocus with nothing marked = true, want false")
	}
	for range 4 {
		h.SwitchPane()
		if st.FocusSelected {
			t.Fatal("strip focused by Tab without marks")
		}
	}
	h.setMark("/scan/orchard/lantern.txt", 100, true)
	seen := false
	for range 4 {
		h.SwitchPane()
		seen = seen || st.FocusSelected
	}
	if !seen {
		t.Fatal("Tab never reached the strip with a mark present")
	}
	h.focusPane(dedupPaneMain)
	if !h.ToggleStripFocus() || !st.FocusSelected {
		t.Fatal("ToggleStripFocus with marks must focus the strip")
	}
	if !h.ToggleStripFocus() || st.FocusSelected {
		t.Fatal("second ToggleStripFocus must return to the main tree")
	}
}

func TestUnmarkStripRowDropsFocusWhenEmpty(t *testing.T) {
	h, model := stripTestHandler(t)
	st := &model.DedupView
	h.setMark("/scan/orchard/lantern.txt", 100, true)
	h.setMark("/scan/meadow/quartz/lantern.txt", 100, true)
	h.ToggleStripFocus()
	h.MoveSelection(1)
	if st.Strip.Cursor != 1 {
		t.Fatalf("strip cursor = %d, want 1", st.Strip.Cursor)
	}
	h.UnmarkStripRow()
	if st.MarkedCount != 1 || st.MarkedReclaimBytes != 100 || !st.Marked["/scan/meadow/quartz/lantern.txt"] {
		t.Fatalf("after unmark: count %d bytes %d marked %v", st.MarkedCount, st.MarkedReclaimBytes, st.Marked)
	}
	h.EnsureSelectionVisible(5)
	if st.Strip.Cursor != 0 {
		t.Fatalf("cursor not clamped: %d", st.Strip.Cursor)
	}
	h.UnmarkStripRow()
	h.ReconcileStripFocus()
	if st.MarkedCount != 0 || st.FocusSelected {
		t.Fatalf("empty marks: count %d focusSelected %v, want 0/false", st.MarkedCount, st.FocusSelected)
	}
}

func TestRevealStripRowExpandsAncestorsAndFocusesMain(t *testing.T) {
	h, model := stripTestHandler(t)
	st := &model.DedupView
	const target = "/scan/meadow/quartz/lantern.txt"
	for _, id := range ui.DedupExpandableIDs(model.DedupSnapshot, *st) {
		st.Main.SetCollapsed(id, true)
	}
	h.resyncPreservingCursor()
	if ui.DedupRowIndexByID(model.DedupList, target) >= 0 {
		t.Fatal("target row visible before reveal; fixture not collapsed")
	}
	h.setMark(target, 100, true)
	h.ToggleStripFocus()
	h.RevealStripRow()
	if st.FocusSelected {
		t.Fatal("reveal must move focus to the main tree")
	}
	row, ok := h.paneRow(&st.Main, model.DedupList)
	if !ok || row.Value.AbsKey != target {
		t.Fatalf("main cursor on %q (ok=%v), want %q", row.Value.AbsKey, ok, target)
	}
}

func TestStripTreeFolderRowUnmarksSubtree(t *testing.T) {
	snap := dedupDoneSnapshot(pathloc.MustParse("/scan"),
		dedupFile("orchard/lantern.txt"), dedupFile("meadow/quartz/lantern.txt"), dedupFile("meadow/quartz/garnet.txt"),
		dedupFile("meadow/willow/ember.txt"))
	h, model := dedupHandlerWithView(t, snap, ui.DedupViewState{
		TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{}, Cache: &ui.DedupCache{},
	})
	st := &model.DedupView
	for _, f := range snap.Groups[0].Files {
		h.setMark(f.Abs.String(), 100, true)
	}
	h.ToggleStripFocus()
	h.ToggleStripTree()
	if !model.SelectedStripTree() {
		t.Fatal("ToggleStripTree did not enable tree mode")
	}
	rows := h.stripRows()
	// meadow/ (quartz/, willow/ children), orchard/ folded with its single file.
	var meadow = -1
	for i, r := range rows {
		if r.Group && r.Display == "meadow" {
			meadow = i
		}
	}
	if meadow < 0 {
		t.Fatalf("no meadow folder row in %+v", rows)
	}
	st.Strip.Cursor = meadow
	h.UnmarkStripRow()
	if st.MarkedCount != 1 || !st.Marked["/scan/orchard/lantern.txt"] || st.MarkedReclaimBytes != 100 {
		t.Fatalf("after folder unmark: count %d bytes %d marked %v", st.MarkedCount, st.MarkedReclaimBytes, st.Marked)
	}
	h.ToggleStripTree()
	if model.SelectedStripTree() || len(h.stripRows()) != 1 {
		t.Fatalf("flat mode rows = %+v", h.stripRows())
	}
}
