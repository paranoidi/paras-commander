package dedup

import (
	"testing"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

type dedupHandlerHost struct {
	msg string
}

func (h *dedupHandlerHost) NavigatePanelToPath(int, string, string) error { return nil }
func (h *dedupHandlerHost) EnqueueDeleteJob([]string, bool)               {}
func (h *dedupHandlerHost) SetTransientMessage(text string, _ ui.MessageUrgency) {
	h.msg = text
}
func (dedupHandlerHost) DedupMenuDefinitions() []menu.Definition   { return nil }
func (dedupHandlerHost) BrowserMenuDefinitions() []menu.Definition { return nil }

func dedupFile(rel string) comparepkg.DedupFile {
	abs := pathloc.MustParse("/scan/" + rel)
	return comparepkg.DedupFile{Rel: rel, Abs: abs}
}

func dedupDoneSnapshot(root pathloc.Path, files ...comparepkg.DedupFile) comparepkg.DedupSnapshot {
	return comparepkg.DedupSnapshot{
		Root:        root,
		DisplayRoot: root,
		Phase:       comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{
			{Size: 100, Files: files},
		},
	}
}

func dedupHandlerWithView(t *testing.T, snap comparepkg.DedupSnapshot, view ui.DedupViewState) (*Handler, *ui.Model) {
	t.Helper()
	model := &ui.Model{
		ViewMode:        ui.ViewDedup,
		DedupSnapshot:   snap,
		DedupView:       view,
		DedupList:       nil,
		DedupCopiesList: nil,
	}
	h := New(Deps{Host: &dedupHandlerHost{}, Model: model})
	h.syncDedupList()
	return h, model
}

func TestCompareDirsFromSelection_requiresMainFileRow(t *testing.T) {
	root := pathloc.MustParse("/scan")
	snap := dedupDoneSnapshot(root, dedupFile("alpha/widget.txt"), dedupFile("beta/widget.txt"))
	view := ui.DedupViewState{
		TreeDirs: true,
		Marked:   map[string]bool{},
		Kept:     map[string]bool{},
	}
	host := &dedupHandlerHost{}
	h := New(Deps{Host: host, Model: &ui.Model{
		ViewMode:      ui.ViewDedup,
		DedupSnapshot: snap,
		DedupView:     view,
	}})
	h.syncDedupList()
	// Select the alpha directory row, not a file.
	for i, row := range h.model.DedupList {
		if row.Value.Kind == ui.DedupRowDir && row.Value.DirRel == "alpha" {
			h.model.DedupView.Main.Selected = i
			break
		}
	}

	_, _, ok := h.CompareDirsFromSelection()
	if ok {
		t.Fatal("CompareDirsFromSelection on main dir row: ok = true, want false")
	}
	if host.msg == "" {
		t.Fatal("expected transient message")
	}
}

func TestCompareDirsFromSelection_copiesDirRow(t *testing.T) {
	root := pathloc.MustParse("/scan")
	fMain := dedupFile("alpha/widget.txt")
	fCopy := dedupFile("beta/widget.txt")
	snap := dedupDoneSnapshot(root, fMain, fCopy)
	view := ui.DedupViewState{
		TreeDirs: true,
		Marked:   map[string]bool{},
		Kept:     map[string]bool{},
	}
	h, model := dedupHandlerWithView(t, snap, view)

	mainIdx := ui.DedupRowIndexByID(model.DedupList, fMain.Abs.String())
	if mainIdx < 0 {
		t.Fatalf("main file row %q not found", fMain.Abs)
	}
	model.DedupView.Main.Selected = mainIdx
	h.syncCopies()

	var copyDirIdx int
	for i, row := range model.DedupCopiesList {
		if row.Value.Kind == ui.DedupRowDir && row.Value.DirRel == "beta" {
			copyDirIdx = i
			break
		}
	}
	if copyDirIdx < 0 {
		t.Fatal("copies pane missing beta directory row")
	}
	model.DedupView.Copies.Selected = copyDirIdx
	model.DedupView.FocusCopies = true

	primary, secondary, ok := h.CompareDirsFromSelection()
	if !ok {
		t.Fatal("CompareDirsFromSelection: ok = false, want true")
	}
	if primary.String() != "/scan/alpha" {
		t.Fatalf("primary = %q, want /scan/alpha", primary)
	}
	if secondary.String() != "/scan/beta" {
		t.Fatalf("secondary = %q, want /scan/beta", secondary)
	}
}

func TestCompareDirsFromSelection_copiesFileRow(t *testing.T) {
	root := pathloc.MustParse("/scan")
	fMain := dedupFile("alpha/widget.txt")
	fCopy := dedupFile("beta/widget.txt")
	snap := dedupDoneSnapshot(root, fMain, fCopy)
	view := ui.DedupViewState{
		TreeDirs: true,
		Marked:   map[string]bool{},
		Kept:     map[string]bool{},
	}
	h, model := dedupHandlerWithView(t, snap, view)

	mainIdx := ui.DedupRowIndexByID(model.DedupList, fMain.Abs.String())
	model.DedupView.Main.Selected = mainIdx
	h.syncCopies()

	copyIdx := ui.DedupRowIndexByID(model.DedupCopiesList, fCopy.Abs.String())
	if copyIdx < 0 {
		t.Fatalf("copy file row %q not found", fCopy.Abs)
	}
	model.DedupView.Copies.Selected = copyIdx
	model.DedupView.FocusCopies = true

	primary, secondary, ok := h.CompareDirsFromSelection()
	if !ok {
		t.Fatal("CompareDirsFromSelection: ok = false, want true")
	}
	if primary.String() != "/scan/alpha" {
		t.Fatalf("primary = %q, want /scan/alpha", primary)
	}
	if secondary.String() != "/scan/beta" {
		t.Fatalf("secondary = %q, want /scan/beta", secondary)
	}
}

func TestApplyPending_prunesMarksAndRestoresCollapse(t *testing.T) {
	root := pathloc.MustParse("/scan")
	keptFile := dedupFile("alpha/kept.txt")
	markedGone := dedupFile("beta/gone.txt")
	markedStay := dedupFile("gamma/stay.txt")
	oldSnap := dedupDoneSnapshot(root, keptFile, markedGone, markedStay)
	newSnap := dedupDoneSnapshot(root, keptFile, markedStay)

	view := ui.DedupViewState{
		TreeDirs:              true,
		SortByWasted:          true,
		IgnoreEmpty:           false,
		GroupsCollapsePending: true,
		Marked:                map[string]bool{},
		Kept:                  map[string]bool{},
	}
	h, model := dedupHandlerWithView(t, oldSnap, view)
	mainIdx := ui.DedupRowIndexByID(model.DedupList, keptFile.Abs.String())
	if mainIdx < 0 {
		t.Fatalf("main file row %q not found", keptFile.Abs)
	}
	model.DedupView.Main.Selected = mainIdx
	h.syncCopies()
	mainRow, ok := h.paneRow(&model.DedupView.Main, model.DedupList)
	if !ok {
		t.Fatal("no main row")
	}
	if ui.DedupRowIndexByID(model.DedupCopiesList, markedGone.Abs.String()) < 0 {
		t.Fatal("copies pane should list beta/gone.txt for alpha/kept.txt selection")
	}

	h.pending = &dedupPendingState{
		marked: map[string]bool{
			markedGone.Abs.String(): true,
			markedStay.Abs.String(): true,
		},
		kept: map[string]bool{
			keptFile.Abs.String(): true,
		},
		mainCollapsed:   map[string]bool{"d:beta": true},
		copiesCollapsed: map[string]bool{},
		treeDirs:        true,
		sortByWasted:    true,
		ignoreEmpty:     false,
		prevExpandable:  ui.DedupExpandableIDs(oldSnap, model.DedupView),
		mainRowID:       mainRow.ID,
		mainRowAbsKey:   mainRow.Value.AbsKey,
		focusCopies:     true,
		copiesRowID:     markedStay.Abs.String(),
		copiesRowAbsKey: markedStay.Abs.String(),
	}

	model.DedupView = ui.DedupViewState{
		Marked:                map[string]bool{},
		Kept:                  map[string]bool{},
		DirsCollapsePending:   true,
		GroupsCollapsePending: true,
	}
	model.DedupSnapshot = newSnap
	h.applyPending(newSnap)

	st := model.DedupView
	if st.Marked[markedGone.Abs.String()] {
		t.Fatal("pruned mark for file absent from rescan should not be restored")
	}
	if !st.Marked[markedStay.Abs.String()] {
		t.Fatal("mark for surviving file should be restored")
	}
	if st.MarkedCount != 1 {
		t.Fatalf("MarkedCount = %d, want 1", st.MarkedCount)
	}
	if st.MarkedReclaimBytes != 100 {
		t.Fatalf("MarkedReclaimBytes = %d, want 100", st.MarkedReclaimBytes)
	}
	if !st.Kept[keptFile.Abs.String()] {
		t.Fatal("kept survivor should be restored")
	}
	if !st.TreeDirs || !st.SortByWasted || st.IgnoreEmpty {
		t.Fatalf("tree toggles not restored: %+v", st)
	}
	if st.DirsCollapsePending {
		t.Fatal("DirsCollapsePending should be cleared after restoring dirs mode")
	}
	if !st.GroupsCollapsePending {
		t.Fatal("GroupsCollapsePending should stay set for the other mode")
	}
	if !st.Main.Collapsed["d:beta"] {
		t.Fatal("main collapse map not restored")
	}

	h.syncDedupList()
	h.restorePendingCursor()
	if h.pending != nil {
		t.Fatal("pending should be cleared after restorePendingCursor")
	}
	if ui.DedupRowIndexByID(model.DedupList, mainRow.ID) != model.DedupView.Main.Selected {
		t.Fatalf("cursor not restored to row %q (selected=%d)", mainRow.ID, model.DedupView.Main.Selected)
	}
	if !model.DedupView.FocusCopies {
		t.Fatal("FocusCopies should be restored when copies pane had focus")
	}
	copyIdx := ui.DedupRowIndexByID(model.DedupCopiesList, markedStay.Abs.String())
	if copyIdx < 0 {
		t.Fatal("copy file row missing after rescan")
	}
	if model.DedupView.Copies.Selected != copyIdx {
		t.Fatalf("copies Selected = %d, want %d", model.DedupView.Copies.Selected, copyIdx)
	}
}
