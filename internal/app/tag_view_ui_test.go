package app

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// tvTag tags index paths (host paths) with tags.
func tvTag(t *testing.T, a *App, tags []string, hosts ...string) {
	t.Helper()
	paths := make([]string, len(hosts))
	for i, h := range hosts {
		paths[i] = "fileindex://" + h
	}
	if _, err := a.fileIndex.EditTags(paths, tags, true, false); err != nil {
		t.Fatal(err)
	}
}

// tvOpenBrowseTags opens Browse by tags and waits for it.
func tvOpenBrowseTags(t *testing.T, a *App, screen tcell.SimulationScreen) *dialog.IndexTagBrowseDialogState {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexBrowseTags)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexTagBrowseDialog.Open })
	return &a.model.IndexTagBrowseDialog
}

func tvKey(a *App, k tcell.Key) { a.handleKey(tcell.NewEventKey(k, 0, tcell.ModNone)) }

// tvWaitMessage waits for the toast text.
func tvWaitMessage(t *testing.T, a *App, screen tcell.SimulationScreen, want string) {
	t.Helper()
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message == want })
}

func TestBrowseByTagsOpensAFlatListAndAStructure(t *testing.T) {
	a, screen := tagViewApp(t)
	tvTag(t, a, []string{"frost", "harvest"}, "/orchard/notes.txt")
	st := tvOpenBrowseTags(t, a, screen)
	if st.Focus != dialog.IndexTagsGrid || st.Any || st.Structure || len(st.Tags) != 3 || st.Tags[0].Name != "beach" {
		t.Fatalf("dialog = focus %q any %v structure %v tags %v, want the grid, All, Flat and three tags", st.Focus, st.Any, st.Structure, st.Tags)
	}
	checkDialogInvariants(t, a)
	typeTags(a, " ") // Space toggles the tag under the cursor
	a.handleKey(altKey('o'))
	if st.Open {
		t.Fatal("OK should close the dialog")
	}
	tvWait(t, a, screen, tvList)

	// The session keeps the last Match and View; Structure opens at the deepest directory holding every match.
	tvGoTo(t, a, screen, "fileindex:///")
	st = tvOpenBrowseTags(t, a, screen)
	if st.Structure {
		t.Fatal("the dialog should open with the View used last")
	}
	typeTags(a, " ")
	a.handleKey(altKey('s'))
	a.handleKey(altKey('o'))
	tvWait(t, a, screen, tvStruct)
	tvGoTo(t, a, screen, "fileindex:///")
	if st = tvOpenBrowseTags(t, a, screen); !st.Structure {
		t.Fatal("the dialog should open with the View used last")
	}
}

func TestBrowseByTagsPrefillsFromTheTagView(t *testing.T) {
	a, screen := tagViewApp(t)
	tvTag(t, a, []string{"harvest"}, "/orchard/notes.txt")
	tvGoTo(t, a, screen, "fileindex(beach&harvest&vanished):///orchard")
	st := tvOpenBrowseTags(t, a, screen)
	if got := st.Checked(); strings.Join(got, ",") != "beach,harvest" || st.Any || !st.Structure {
		t.Fatalf("checked %v any %v structure %v, want beach and harvest, All, Structure", got, st.Any, st.Structure)
	}
	tvKey(a, tcell.KeyEsc)
	tvGoTo(t, a, screen, "fileindex@(beach|harvest):///")
	st = tvOpenBrowseTags(t, a, screen)
	if len(st.Checked()) != 2 || !st.Any || st.Structure {
		t.Fatalf("checked %v any %v structure %v, want both, Any, Flat", st.Checked(), st.Any, st.Structure)
	}
}

func TestBrowseByTagsNeedsATagAndWarnsWhenNothingMatches(t *testing.T) {
	a, screen := tagViewApp(t)
	tvTag(t, a, []string{"harvest"}, "/orchard/notes.txt")
	tvTag(t, a, []string{"frost"}, "/cedar/notes.txt")
	st := tvOpenBrowseTags(t, a, screen)
	a.handleKey(altKey('o'))
	if !st.Open || a.model.Message != "Check at least one tag" || a.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("open %v, message %q (%v): no tag checked must keep the dialog open", st.Open, a.model.Message, a.model.MessageUrgency)
	}
	for _, name := range []string{"frost", "harvest"} { // tags sort as beach, frost, harvest
		typeTags(a, name)
		typeTags(a, " ")
	}
	before := a.focusedPanel().PathString()
	a.handleKey(altKey('o'))
	tvWaitMessage(t, a, screen, "No entries tagged frost & harvest")
	if a.focusedPanel().PathString() != before {
		t.Fatalf("the panel moved to %s", a.focusedPanel().PathString())
	}

	tvOpenBrowseTags(t, a, screen)
	for _, name := range []string{"frost", "harvest"} {
		typeTags(a, name)
		typeTags(a, " ")
	}
	a.handleKey(altKey('n')) // Match any
	a.handleKey(altKey('o'))
	tvWait(t, a, screen, "fileindex@(frost|harvest):///")
	if n := a.focusedPanel().VisibleEntryCount(); n != 2 {
		t.Fatalf("any of the tags lists %d rows, want 2", n)
	}
}

// A result that arrives while the event queue is full is delivered, not dropped.
func TestBrowseByTagsOpensThoughTheEventQueueIsFull(t *testing.T) {
	a, screen := tagViewApp(t)
	raw := screen.(losslessSim).SimulationScreen
	for raw.PostEvent(tcell.NewEventInterrupt(nil)) == nil {
	}
	a.dispatch(keymap.ActionFileIndexBrowseTags)
	time.Sleep(100 * time.Millisecond) // the names load and post into the full queue
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexTagBrowseDialog.Open })
}

func TestBrowseByTagsWithoutTagsToasts(t *testing.T) {
	a, screen, _, _ := fileIndexApp(t)
	a.dispatch(keymap.ActionFileIndexBrowseTags)
	tvWaitMessage(t, a, screen, "No tags yet")
	if a.model.IndexTagBrowseDialog.Open {
		t.Fatal("the dialog opened over an index with no tags")
	}
}

func TestBrowseByTagsWorksFromAnSFTPPanel(t *testing.T) {
	a, screen := tagViewApp(t)
	a.focusedPanel().Path = pathloc.MustParse("sftp://user@example.test/srv")
	st := tvOpenBrowseTags(t, a, screen)
	if !st.Open {
		t.Fatal("the dialog should open from any panel")
	}
}

func TestTagViewModeFromTheFlatListShowsTheRowInPlace(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	tvSelect(t, a, tvOrchL)
	a.dispatch(keymap.ActionFileIndexTagViewMode)
	tvWait(t, a, screen, "fileindex(beach):///orchard")
	if got := tvCursor(t, a); got != "fileindex(beach):///orchard/lantern" {
		t.Fatalf("cursor = %s, want the row in its directory", got)
	}
	a.dispatch(keymap.ActionFileIndexTagViewMode)
	tvWait(t, a, screen, tvList)
	if got := tvCursor(t, a); got != tvOrchL {
		t.Fatalf("back in the list the cursor is on %s, want %s", got, tvOrchL)
	}
}

func TestTagViewModeFromInsideARowKeepsTheCursor(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvOrchL)
	if !a.focusedPanel().SelectVisibleEntry("clover.txt") {
		t.Fatal("clover.txt not listed")
	}
	a.dispatch(keymap.ActionFileIndexTagViewMode)
	tvWait(t, a, screen, "fileindex(beach):///orchard/lantern")
	if got := tvCursor(t, a); got != "fileindex(beach):///orchard/lantern/clover.txt" {
		t.Fatalf("cursor = %s", got)
	}
}

func TestTagViewModeFromTheStructureGoesToTheRowOrStays(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, "fileindex(beach):///orchard")
	if !a.focusedPanel().SelectVisibleEntry("lantern") {
		t.Fatal("lantern not listed")
	}
	a.dispatch(keymap.ActionFileIndexTagViewMode)
	tvWait(t, a, screen, tvList)
	if got := tvCursor(t, a); got != tvOrchL {
		t.Fatalf("cursor = %s, want the lantern row", got)
	}

	// A directory that is no match has no row: the list opens with its cursor wherever it falls.
	tvGoTo(t, a, screen, tvStruct)
	tvSelect(t, a, "fileindex(beach):///orchard")
	a.dispatch(keymap.ActionFileIndexTagViewMode)
	tvWait(t, a, screen, tvList)
	if n := a.focusedPanel().VisibleEntryCount(); n != 4 {
		t.Fatalf("list has %d rows, want 4", n)
	}
}

func TestTagViewModeOfAnEmptyListOpensTheStructureAtTheRoot(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, "fileindex@(missing):///")
	a.dispatch(keymap.ActionFileIndexTagViewMode)
	tvWait(t, a, screen, "fileindex(missing):///")
}

func TestTagViewModeOutsideATagViewWarns(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	for _, p := range []string{dir, "fileindex://" + dir} {
		tvGoTo(t, a, screen, p)
		a.dispatch(keymap.ActionFileIndexTagViewMode)
		if a.model.Message != "Flat or structure works in a tag view" || a.model.MessageUrgency != ui.MessageUrgencyWarn {
			t.Fatalf("%s: message %q", p, a.model.Message)
		}
		if got, want := a.focusedPanel().PathString(), pathloc.MustParse(p).String(); got != want {
			t.Fatalf("the panel moved to %s", got)
		}
	}
}

func TestBrowseFromATagViewGoesToThePlainIndex(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	tvSelect(t, a, tvOrchL)
	a.dispatch(keymap.ActionFileIndexBrowse)
	tvWait(t, a, screen, "fileindex:///orchard")
	if got := tvCursor(t, a); got != "fileindex:///orchard/lantern" {
		t.Fatalf("cursor = %s", got)
	}

	// The structure view goes to the directory it shows, with no entry, to its own host directory.
	tvGoTo(t, a, screen, "fileindex(beach):///orchard/lantern")
	tvSelect(t, a, "fileindex(beach):///orchard/lantern/sub")
	a.dispatch(keymap.ActionFileIndexBrowse)
	tvWait(t, a, screen, "fileindex:///orchard/lantern")
	if got := tvCursor(t, a); got != "fileindex:///orchard/lantern/sub" {
		t.Fatalf("cursor = %s", got)
	}

	// An empty flat list has no entry: the host directory is "/".
	tvGoTo(t, a, screen, "fileindex@(missing):///")
	a.dispatch(keymap.ActionFileIndexBrowse)
	tvWait(t, a, screen, "fileindex:///")
}

// tagViewDiskApp is an app whose index holds two real files tagged beach.
func tagViewDiskApp(t *testing.T) (*App, tcell.SimulationScreen, string) {
	t.Helper()
	a, screen, dir, _ := fileIndexApp(t)
	seedTags(t, a, dir, []string{"beach"}, "harbor.txt", "meadow.txt")
	return a, screen, dir
}

func TestRevealAndBrowseWithFromATagViewUseTheCursorEntry(t *testing.T) {
	a, screen, dir := tagViewDiskApp(t)
	for _, action := range []string{keymap.ActionFileIndexReveal, keymap.ActionFileIndexBrowseWith} {
		tvGoTo(t, a, screen, "fileindex@(beach):///")
		tvSelect(t, a, "fileindex@(beach)://"+dir+"/meadow.txt")
		a.dispatch(action)
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
			p := a.focusedPanel()
			return p.PathString() == dir && !p.ListingPending
		})
		if e, ok := a.focusedPanel().CurrentEntry(); !ok || e.Name != "meadow.txt" {
			t.Fatalf("%s: cursor = %v", action, e.Name)
		}
		a.focusedPanel().WithIndex = false
	}
}

func TestRevealAndBrowseWithRefuseAnEmptyTagList(t *testing.T) {
	a, screen, _ := tagViewDiskApp(t)
	tvGoTo(t, a, screen, "fileindex@(missing):///")
	for _, action := range []string{keymap.ActionFileIndexReveal, keymap.ActionFileIndexBrowseWith} {
		a.model.Message = ""
		a.dispatch(action)
		tvQuiet(t, a, screen)
		if a.model.Message != pathloc.TagListRefusal {
			t.Fatalf("%s: message %q", action, a.model.Message)
		}
		if got := a.focusedPanel().PathString(); got != "fileindex@(missing):///" {
			t.Fatalf("%s: moved to %s", action, got)
		}
	}
}

// jobCount is how many jobs the app has queued.
func jobCount(a *App) int { return len(a.jobState.AllJobs()) }

// TestTagListRefusesActionsThatNeedADirectory runs every action that falls back to the panel
// directory on the (empty) flat list: each toasts the refusal and queues nothing.
func TestTagListRefusesActionsThatNeedADirectory(t *testing.T) {
	a, screen, _ := tagViewDiskApp(t)
	tvGoTo(t, a, screen, "fileindex@(missing):///")
	for _, action := range []string{
		keymap.ActionFileIndexScan, keymap.ActionFileIndexScanFast, keymap.ActionFileIndexPrune,
		keymap.ActionFileIndexReportDifferences, keymap.ActionFileIndexCopiesOutside, keymap.ActionFileIndexFindDuplicates,
		keymap.ActionFileIndexFindUnique, keymap.ActionFileIndexTags, keymap.ActionPanelFindDuplicates,
	} {
		a.model.Message = ""
		a.dispatch(action)
		tvWaitMessage(t, a, screen, pathloc.TagListRefusal)
		if n := jobCount(a); n != 0 {
			t.Fatalf("%s queued %d jobs", action, n)
		}
		if a.model.IndexTagsDialog.Open || a.model.DedupProgressDialog.Open || a.model.ViewMode != ui.ViewBrowser {
			t.Fatalf("%s opened something", action)
		}
	}
}

func TestTagListGuardsGiveWayToASelection(t *testing.T) {
	a, screen, dir := tagViewDiskApp(t)
	tvGoTo(t, a, screen, "fileindex@(beach):///")
	row := "fileindex@(beach):/" + dir + "/harbor.txt"

	// Scan works on the selection, and only the selection (never the cursor row).
	tvSelect(t, a, "fileindex@(beach)://"+dir+"/meadow.txt")
	a.focusedPanel().SelectedPaths = map[string]bool{row: true}
	a.dispatch(keymap.ActionFileIndexScan)
	tvWaitMessage(t, a, screen, "fileindex scan queued")

	// Tags works on the cursor row too.
	a.dispatch(keymap.ActionFileIndexTags)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexTagsDialog.Open })
	if got := a.model.IndexTagsDialog.Targets; len(got) != 1 || !strings.HasSuffix(got[0], "meadow.txt") {
		t.Fatalf("tag targets = %v", got)
	}
	a.fileindexCtrl.CloseTagDialog()
}

func TestTagListUniqueFilesWorksOnSelectedDirectories(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	a.focusedPanel().SelectedPaths = map[string]bool{tvOrchL: true}
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.ViewMode == ui.ViewUnique })
	if root := a.model.UniqueView.Root; root != "/" {
		t.Fatalf("unique root = %q", root)
	}
	if got := a.model.UniqueView.Scope; len(got) != 1 || got[0] != "/orchard/lantern" {
		t.Fatalf("unique scope = %v", got)
	}
}

func TestScanAfterCommandRefusesATagList(t *testing.T) {
	a, _ := tagViewApp(t)
	a.fileindexCtrl.ScanAfterCommand(pathloc.MustParse(tvList))
	if a.model.Message != pathloc.TagListRefusal || jobCount(a) != 0 {
		t.Fatalf("message %q, %d jobs", a.model.Message, jobCount(a))
	}
}

func TestJumpFromATagViewLandsInThePlainIndex(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	st := dialog.JumpDialogState{Source: dialog.JumpIndex}
	if got := st.PanelPath(dialog.JumpResult{Path: "/orchard/lantern"}); got != "fileindex:///orchard/lantern" {
		t.Fatalf("jump result path = %s", got)
	}
	a.openJumpDialog(a.focusedPanelID())
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.JumpDialog.Open })
	if got := a.model.JumpDialog; got.Root != "/" || got.Source != dialog.JumpIndex {
		t.Fatalf("jump dialog root %q source %v, want the whole index", got.Root, got.Source)
	}
}

func TestTagViewRenameOfAFlatRowStaysIndexOnly(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	tvSelect(t, a, tvOrchN)
	dispatchFileOp(t, a, keymap.ActionFileRename)
	if !strings.Contains(a.model.FileDialog.Note, "Renames in the fileindex only") {
		t.Fatalf("rename note = %q", a.model.FileDialog.Note)
	}
	a.model.FileDialog.Fields[0].Value = "anchor.txt"
	a.dialogCtrl.ExecuteFileDialog()
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, oldIndexed, _ := d.FileInfo("/orchard/notes.txt")
		_, newIndexed, _ := d.FileInfo("/orchard/anchor.txt")
		return !oldIndexed && newIndexed
	})
	if _, indexed, _ := d.FileInfo("/cedar/notes.txt"); !indexed {
		t.Fatal("the equally named file of the other directory was touched")
	}
}

func TestTagViewRenameOfAFlatRowFocusesTheRow(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	tvSelect(t, a, tvOrchN)
	dispatchFileOp(t, a, keymap.ActionFileRename)
	a.model.FileDialog.Fields[0].Value = "anchor.txt"
	a.model.FileDialog.RenameFocusAfter = true
	a.dialogCtrl.ExecuteFileDialog()
	want := "fileindex@(beach):///orchard/anchor.txt" // the row's path, not one inside a row ("//")
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		e, ok := a.focusedPanel().CurrentEntry()
		return ok && e.Path == want
	})
}

func TestTagViewDeleteOfAFlatRowRemovesOnlyThatRecord(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvList)
	tvSelect(t, a, tvOrchN)
	dispatchFileOp(t, a, keymap.ActionFileDelete)
	if !strings.Contains(a.model.FileDialog.Note, "Removes from the fileindex only") {
		t.Fatalf("delete note = %q", a.model.FileDialog.Note)
	}
	a.dialogCtrl.ExecuteDelete()
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, indexed, _ := d.FileInfo("/orchard/notes.txt")
		return !indexed
	})
	if _, indexed, _ := d.FileInfo("/cedar/notes.txt"); !indexed {
		t.Fatal("the equally named file of the other directory was removed")
	}
}

// partialDirRecordsSurvive checks the records under the partial structure directory /orchard, the
// ones the tag view hides included.
func partialDirRecordsSurvive(t *testing.T, a *App) {
	t.Helper()
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"/orchard/notes.txt", "/orchard/lantern/clover.txt", "/orchard/lantern/sub/moss.txt"} {
		if _, indexed, _ := d.FileInfo(f); !indexed {
			t.Fatalf("the record of %s is gone", f)
		}
	}
}

func TestTagViewRenameOfAPartialDirectoryIsRefused(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvStruct)
	tvSelect(t, a, "fileindex(beach):///orchard")
	dispatchFileOp(t, a, keymap.ActionFileRename)
	a.model.FileDialog.Fields[0].Value = "grove"
	a.dialogCtrl.ExecuteFileDialog()
	tvWaitMessage(t, a, screen, "rename failed: Not tagged itself: orchard")
	partialDirRecordsSurvive(t, a)
}

func TestTagViewDeleteOfAPartialDirectoryIsRefused(t *testing.T) {
	a, screen := tagViewApp(t)
	tvGoTo(t, a, screen, tvStruct)
	tvSelect(t, a, "fileindex(beach):///orchard")
	dispatchFileOp(t, a, keymap.ActionFileDelete)
	a.dialogCtrl.ExecuteDelete()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		for _, j := range a.jobState.AllJobs() {
			if j.Status == jobs.StatusFailed && strings.Contains(j.Error, "Not tagged itself: orchard") {
				return true
			}
		}
		return false
	})
	partialDirRecordsSurvive(t, a)
}
