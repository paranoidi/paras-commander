package dedup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

type dedupHandlerHost struct {
	msg                string
	navPanel           int
	navPath, navSelect string
}

func (h *dedupHandlerHost) NavigatePanelToPath(p int, path, sel string) error {
	h.navPanel, h.navPath, h.navSelect = p, path, sel
	return nil
}
func (h *dedupHandlerHost) EnqueueDeleteJob([]string, bool) {}
func (h *dedupHandlerHost) Config() config.Config           { return config.Default() }
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

func keptHandler(t *testing.T, activePath string) (*Handler, *ui.Model) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := pathloc.MustParse("/scan")
	snap := dedupDoneSnapshot(root, dedupFile("alpha/widget.txt"), dedupFile("beta/widget.txt"))
	h, model := dedupHandlerWithView(t, snap, ui.DedupViewState{
		TreeDirs: true,
		Marked:   map[string]bool{},
		Kept:     map[string]bool{},
	})
	model.Primary.Path = pathloc.MustParse(activePath)
	h.Leave()
	return h, model
}

func TestOpenInKeptRootOffersDialogElsewhereRescans(t *testing.T) {
	h, model := keptHandler(t, "/scan")
	if model.ViewMode != ui.ViewBrowser || !h.HasResults() {
		t.Fatalf("after Leave: ViewMode=%v HasResults=%v", model.ViewMode, h.HasResults())
	}
	h.Open()
	if !model.DedupReturnDialog.Open || h.session != nil {
		t.Fatalf("in root: dialog open=%v session=%v", model.DedupReturnDialog.Open, h.session)
	}
	h.Close()

	dir := t.TempDir()
	h, model = keptHandler(t, dir)
	h.Open()
	if model.DedupReturnDialog.Open || h.session == nil {
		t.Fatalf("other dir: dialog open=%v session=%v", model.DedupReturnDialog.Open, h.session)
	}
	h.Close()
}

// verifiedHandler keeps results over real files so verification can stat them.
func verifiedHandler(t *testing.T, names ...string) (*Handler, *ui.Model, string) {
	t.Helper()
	dir := t.TempDir()
	root := pathloc.MustParse(dir)
	var files []comparepkg.DedupFile
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, comparepkg.DedupFile{Rel: n, Abs: pathloc.MustParse(filepath.Join(dir, n))})
	}
	h, model := dedupHandlerWithView(t, dedupDoneSnapshot(root, files...), ui.DedupViewState{
		TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{},
	})
	model.Primary.Path = root
	h.Leave()
	return h, model, dir
}

func pollVerified(t *testing.T, h *Handler) {
	t.Helper()
	h.VerifyAndShowKept()
	for i := 0; i < 500; i++ {
		h.PollUpdates(WakePayload{})
		if h.model.DedupView.RecheckTotal == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("verify never finished")
}

func TestVerifyDropsVanishedFilesAndCollapsesGroup(t *testing.T) {
	h, model, dir := verifiedHandler(t, "meadow.txt", "lantern.txt", "orchard.txt")
	if err := os.Remove(filepath.Join(dir, "orchard.txt")); err != nil {
		t.Fatal(err)
	}
	pollVerified(t, h)
	if model.ViewMode != ui.ViewDedup || len(model.DedupSnapshot.Groups) != 1 || len(model.DedupSnapshot.Groups[0].Files) != 2 {
		t.Fatalf("ViewMode=%v snapshot=%+v", model.ViewMode, model.DedupSnapshot.Groups)
	}
	h.Leave()
	if err := os.Remove(filepath.Join(dir, "lantern.txt")); err != nil {
		t.Fatal(err)
	}
	pollVerified(t, h)
	if model.ViewMode != ui.ViewBrowser || h.HasResults() {
		t.Fatalf("all gone: ViewMode=%v HasResults=%v", model.ViewMode, h.HasResults())
	}
}

func TestShowKeptWithoutResultsIsNoop(t *testing.T) {
	h := New(Deps{Host: &dedupHandlerHost{}, Model: &ui.Model{ViewMode: ui.ViewBrowser}})
	h.ShowKept()
	if h.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser", h.model.ViewMode)
	}
}

func TestRefreshRescansKeptRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := pathloc.MustParse(dir)
	snap := dedupDoneSnapshot(root, dedupFile("alpha/widget.txt"), dedupFile("beta/widget.txt"))
	h, model := dedupHandlerWithView(t, snap, ui.DedupViewState{Marked: map[string]bool{}, Kept: map[string]bool{}})
	model.Primary.Path = pathloc.MustParse("/elsewhere")
	h.Refresh()
	if h.session == nil {
		t.Fatal("Refresh should start a session")
	}
	if got := h.session.Snapshot().Root.String(); got != dir {
		t.Fatalf("Refresh root = %q, want kept root %q", got, dir)
	}
	h.Close()
}

func TestDeleteMarked_keepsCursorNearDeletedRow(t *testing.T) {
	root := pathloc.MustParse("/scan")
	group := func(name string) comparepkg.DedupGroup {
		return comparepkg.DedupGroup{Size: 100, Files: []comparepkg.DedupFile{
			dedupFile("amber/" + name), dedupFile("cobalt/" + name),
		}}
	}
	snap := comparepkg.DedupSnapshot{
		Root: root, DisplayRoot: root, Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{group("lantern.txt"), group("meadow.txt"), group("quiver.txt")},
	}
	h, model := dedupHandlerWithView(t, snap, ui.DedupViewState{
		TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{},
	})
	deleteAt := func(dir, name string) {
		t.Helper()
		i := ui.DedupRowIndexByID(model.DedupList, "/scan/"+dir+"/"+name)
		if i < 0 {
			t.Fatalf("row for %s/%s not found", dir, name)
		}
		model.DedupView.Main.Selected = i
		h.setMark("/scan/amber/"+name, 100, true)
		h.setMark("/scan/cobalt/"+name, 100, true)
		h.DeleteMarked(false)
	}
	selectedID := func() string {
		row, _ := h.paneRow(&model.DedupView.Main, model.DedupList)
		return row.ID
	}

	deleteAt("amber", "meadow.txt") // middle row gone → next surviving row
	if got := selectedID(); got != "/scan/amber/quiver.txt" {
		t.Fatalf("after middle delete selected %q, want /scan/amber/quiver.txt", got)
	}
	if last := model.DedupList[len(model.DedupList)-1].ID; last != "/scan/cobalt/quiver.txt" {
		t.Fatalf("last row = %q, want /scan/cobalt/quiver.txt", last)
	}
	deleteAt("cobalt", "quiver.txt") // tail rows gone → previous surviving row
	if got := selectedID(); got != "/scan/cobalt/lantern.txt" {
		t.Fatalf("after tail delete selected %q, want /scan/cobalt/lantern.txt", got)
	}
}

func TestOpenInPanel_navigatesAndKeepsView(t *testing.T) {
	host := &dedupHandlerHost{}
	model := &ui.Model{ViewMode: ui.ViewDedup}
	h := New(Deps{Host: host, Model: model})
	h.OpenInPanel(ui.SecondaryPanel, "/scan/alpha/widget.txt", false)
	if host.navPanel != ui.SecondaryPanel || host.navPath != "/scan/alpha" || host.navSelect != "widget.txt" {
		t.Fatalf("nav = %d %q %q", host.navPanel, host.navPath, host.navSelect)
	}
	if host.msg != "Opened widget.txt in secondary panel" || model.ViewMode != ui.ViewDedup {
		t.Fatalf("msg %q view %v", host.msg, model.ViewMode)
	}
	h.OpenInPanel(ui.PrimaryPanel, "/scan/alpha", true)
	if host.navPath != "/scan/alpha" || host.navSelect != "" {
		t.Fatalf("dir nav = %q %q", host.navPath, host.navSelect)
	}
}

func TestTypeToJumpFilter(t *testing.T) {
	root := pathloc.MustParse("/scan")
	group := func(name string) comparepkg.DedupGroup {
		return comparepkg.DedupGroup{Size: 100, Files: []comparepkg.DedupFile{
			dedupFile("amber/" + name), dedupFile("cobalt/" + name),
		}}
	}
	snap := comparepkg.DedupSnapshot{
		Root: root, DisplayRoot: root, Phase: comparepkg.DedupDone,
		Groups: []comparepkg.DedupGroup{group("lantern.txt"), group("meadow.txt"), group("quiver.txt")},
	}
	h, model := dedupHandlerWithView(t, snap, ui.DedupViewState{
		TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{},
	})
	st := &model.DedupView
	selID := func() string {
		row, _ := h.paneRow(&st.Main, model.DedupList)
		return row.ID
	}

	for _, r := range "quiv" {
		h.AppendFilterRune(r)
	}
	if !h.FocusedFilter().UIActive() {
		t.Fatal("filter should be active")
	}
	first := selID()
	if !strings.HasSuffix(first, "quiver.txt") {
		t.Fatalf("selected %q, want a quiver.txt row", first)
	}
	if len(model.DedupCopiesList) == 0 {
		t.Fatal("copies pane did not resync to the jumped row")
	}

	h.CycleFilterMatch(1)
	second := selID()
	if second == first || !strings.HasSuffix(second, "quiver.txt") {
		t.Fatalf("cycle moved to %q, want the other quiver.txt row", second)
	}
	h.CycleFilterMatch(1)
	if selID() != first {
		t.Fatalf("cycle did not wrap back to %q", first)
	}

	h.CancelFilter()
	if h.FocusedFilter().UIActive() || st.Main.Filter.Query != "" {
		t.Fatal("cancel should clear the filter")
	}
	if selID() != first {
		t.Fatal("cancel must keep the cursor")
	}

	h.AppendFilterRune('m')
	h.SwitchPane()
	if st.Main.Filter.UIActive() {
		t.Fatal("switching panes should clear the filter")
	}
}

func TestSwitchPanePrevCyclesInReverse(t *testing.T) {
	root := pathloc.MustParse("/scan")
	snap := dedupDoneSnapshot(root, dedupFile("alpha/widget.txt"), dedupFile("beta/widget.txt"))
	h, model := dedupHandlerWithView(t, snap, ui.DedupViewState{TreeDirs: true, Marked: map[string]bool{}, Kept: map[string]bool{}})
	model.DedupCopiesList = []ui.DedupRow{{}}
	st := &model.DedupView
	focus := func() string {
		switch {
		case st.FocusPanel:
			return "panel"
		case st.FocusCopies:
			return "copies"
		}
		return "main"
	}
	for _, want := range []string{"panel", "copies", "main", "panel"} {
		h.SwitchPanePrev()
		if got := focus(); got != want {
			t.Fatalf("SwitchPanePrev focus = %s, want %s", got, want)
		}
	}
	for _, want := range []string{"main", "copies", "panel"} {
		h.SwitchPane()
		if got := focus(); got != want {
			t.Fatalf("SwitchPane focus = %s, want %s", got, want)
		}
	}
	model.DedupCopiesList = nil
	st.FocusPanel = true
	h.SwitchPanePrev()
	if got := focus(); got != "main" {
		t.Fatalf("SwitchPanePrev without copies = %s, want main", got)
	}
}
