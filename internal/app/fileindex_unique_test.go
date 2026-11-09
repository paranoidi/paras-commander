package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/fileindex/db"

	uniquefilesctrl "github.com/paranoidi/paras-commander/internal/apphandler/uniquefiles"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// uniqueApp is fileIndexApp plus two indexed subdirectories: orchard/thistle.txt and
// willow/bramble.txt are unique, orchard/clover.txt has its copy in willow/sedge.txt.
func uniqueApp(t *testing.T) (a *App, screen tcell.SimulationScreen, dir, orchard, willow string) {
	t.Helper()
	a, screen, dir, _ = fileIndexApp(t)
	orchard, willow = filepath.Join(dir, "orchard"), filepath.Join(dir, "willow")
	for _, sub := range []string{orchard, willow} {
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	scanned := func(dir, name, fill string) db.ScannedFile {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("words"), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := strings.Repeat(fill, 32)
		return db.ScannedFile{Dir: dir, Name: name, Size: 5, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}
	}
	if _, err := d.CommitScanBatch([]db.ScannedFile{
		scanned(orchard, "thistle.txt", "c"),
		scanned(orchard, "clover.txt", "d"),
		scanned(willow, "sedge.txt", "d"),
		scanned(willow, "bramble.txt", "e"),
	}); err != nil {
		t.Fatal(err)
	}
	return a, screen, dir, orchard, willow
}

// openUnique runs the action and waits for the lookup to land.
func openUnique(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.ViewMode == ui.ViewUnique && a.model.UniqueView.Loaded && !a.model.UniqueView.Loading
	})
}

func uniqueFiles(a *App) []string {
	var out []string
	for _, f := range a.model.UniqueView.Files {
		rel, err := filepath.Rel(a.model.UniqueView.Root, f.Path)
		if err != nil {
			rel = f.Path
		}
		out = append(out, rel)
	}
	return out
}

func uniqueRowTexts(a *App) []string {
	var out []string
	for _, r := range a.model.UniqueList {
		out = append(out, r.Value.Display)
	}
	return out
}

func TestUniqueViewScopesToPanelDirectory(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	if st.Root != dir || !slices.Equal(st.Scope, []string{dir}) {
		t.Fatalf("root = %q scope = %v", st.Root, st.Scope)
	}
	// Every copy is inside the panel directory, so all six files are listed.
	if got := uniqueFiles(a); len(got) != 6 {
		t.Fatalf("files = %v", got)
	}
	// Directories start collapsed, before the files directly in the directory.
	want := []string{"orchard", "willow", "harbor.txt", "meadow.txt"}
	if got := uniqueRowTexts(a); !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestUniqueViewScopesToSelectedDirectories(t *testing.T) {
	a, screen, _, orchard, willow := uniqueApp(t)
	a.activePanel().AddSelection(orchard)
	openUnique(t, a, screen)
	if got := uniqueFiles(a); !slices.Equal(got, []string{"orchard/thistle.txt"}) {
		t.Fatalf("one directory: %v (clover has a copy in willow)", got)
	}
	a.closeUniqueView()

	// Both selected: the copy is inside the scope, so clover and sedge are listed, and a new
	// scope is a new lookup rather than the kept results.
	a.activePanel().AddSelection(willow)
	openUnique(t, a, screen)
	want := []string{"orchard/clover.txt", "orchard/thistle.txt", "willow/bramble.txt", "willow/sedge.txt"}
	if got := uniqueFiles(a); !slices.Equal(got, want) {
		t.Fatalf("two directories: %v, want %v", got, want)
	}
}

func TestUniqueViewRefusesMixedSelection(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	a.activePanel().AddSelection(orchard)
	a.activePanel().AddSelection(filepath.Join(dir, "harbor.txt"))
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message != "" })
	if a.model.ViewMode != ui.ViewBrowser || a.model.MessageUrgency != ui.MessageUrgencyCritical || !strings.Contains(a.model.Message, "select only") {
		t.Fatalf("view=%v urgency=%v message=%q", a.model.ViewMode, a.model.MessageUrgency, a.model.Message)
	}
}

func TestUniqueViewIgnoresSelectionOfOnlyFiles(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	selected := filepath.Join(dir, "harbor.txt")
	a.activePanel().AddSelection(selected)
	openUnique(t, a, screen)
	if !slices.Equal(a.model.UniqueView.Scope, []string{dir}) || !a.activePanel().SelectedPaths[selected] {
		t.Fatalf("scope = %v selection = %v", a.model.UniqueView.Scope, a.activePanel().SelectedPaths)
	}
}

func TestUniqueViewRefusesIndexRoot(t *testing.T) {
	a, screen, _, _, _ := uniqueApp(t)
	if err := a.navigatePanelToDirectory(a.model.ActivePanel, "fileindex:///", ""); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == "fileindex:///" && !p.ListingPending
	})
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message != "" })
	if a.model.ViewMode != ui.ViewBrowser || !strings.Contains(a.model.Message, "nothing outside") {
		t.Fatalf("view=%v message=%q", a.model.ViewMode, a.model.Message)
	}
}

// A selected fileindex:// directory off the listing still counts as a directory.
func TestUniqueViewClassifiesIndexSelection(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	a.dispatch(keymap.ActionFileIndexBrowse)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == "fileindex://"+dir && !p.ListingPending
	})
	a.activePanel().AddSelection("fileindex://" + orchard)
	openUnique(t, a, screen)
	if !slices.Equal(a.model.UniqueView.Scope, []string{orchard}) {
		t.Fatalf("scope = %v", a.model.UniqueView.Scope)
	}
	a.uniqueCtrl.Leave()
	a.model.Message = ""
	a.activePanel().AddSelection("fileindex://" + filepath.Join(dir, "willow", "bramble.txt"))
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message != "" })
	if a.model.ViewMode != ui.ViewBrowser {
		t.Fatal("a mixed index selection opened the view")
	}
}

func TestUniqueViewTreeKeysAndMarks(t *testing.T) {
	a, screen, _, orchard, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView

	// Right opens the cursor directory in place; Left closes it again.
	a.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := uniqueRowTexts(a); !slices.Contains(got, "thistle.txt") || st.Pane.Selected != 0 {
		t.Fatalf("expanded rows = %v cursor = %d", got, st.Pane.Selected)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if got := uniqueRowTexts(a); slices.Contains(got, "thistle.txt") {
		t.Fatalf("collapsed rows = %v", got)
	}

	// Marking a collapsed directory marks what is inside it and moves past it.
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if len(st.Marked) != 2 || !st.Marked[filepath.Join(orchard, "thistle.txt")] || st.MarkedBytes != 10 {
		t.Fatalf("marked = %v bytes = %d", st.Marked, st.MarkedBytes)
	}
	if st.Pane.Selected != 1 {
		t.Fatalf("cursor = %d, want past the directory", st.Pane.Selected)
	}
	sources := a.uniqueCtrl.Sources()
	if len(sources) != 2 {
		t.Fatalf("sources = %v", sources)
	}
	// Marking it again from the row clears the marks.
	st.Pane.Selected = 0
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if len(st.Marked) != 0 || st.MarkedBytes != 0 {
		t.Fatalf("after unmark: %v %d", st.Marked, st.MarkedBytes)
	}
	// With no marks, an operation acts on the cursor directory's files.
	st.Pane.Selected = 0
	if got := a.uniqueCtrl.Sources(); len(got) != 2 {
		t.Fatalf("cursor directory sources = %v", got)
	}
}

func TestUniqueViewHidingDropsMarksOfHiddenFiles(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	hidden := filepath.Join(dir, ".nook")
	if err := os.Mkdir(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("f", 32)
	if _, err := d.CommitScanBatch([]db.ScannedFile{{Dir: hidden, Name: "fern.txt", Size: 5, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	st.Pane.Selected = 0 // the first directory, .nook
	if a.model.UniqueList[0].Value.Display != ".nook" {
		t.Fatalf("rows = %v", uniqueRowTexts(a))
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if len(st.Marked) != 1 {
		t.Fatalf("marked = %v", st.Marked)
	}
	a.dispatch(keymap.ActionPanelToggleHidden)
	if len(st.Marked) != 0 || st.MarkedBytes != 0 || slices.Contains(uniqueRowTexts(a), ".nook") {
		t.Fatalf("marked = %v bytes = %d rows = %v", st.Marked, st.MarkedBytes, uniqueRowTexts(a))
	}
	if len(a.uniqueCtrl.Sources()) == 0 {
		t.Fatal("no sources at the cursor")
	}
}

// Copy and move use the view's files and the inactive panel, and leave the panel selection alone.
func TestUniqueViewCopyAndMoveTargetOtherPanel(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	dest := t.TempDir()
	dest, err := filepath.EvalSymlinks(dest)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.navigatePanelToDirectory(a.inactivePanelID(), dest, ""); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return !a.inactivePanel().ListingPending })
	panelSelected := filepath.Join(dir, "meadow.txt")
	a.activePanel().AddSelection(panelSelected)
	openUnique(t, a, screen)
	st := &a.model.UniqueView

	a.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) // orchard open: clover.txt, thistle.txt
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "thistle.txt")
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	a.dispatch(keymap.ActionCopy)
	if !a.model.TransferDialog.Open || !strings.HasPrefix(a.model.TransferDialog.Destination.Value, dest) {
		t.Fatalf("copy dialog: %+v", a.model.TransferDialog)
	}
	a.dialogCtrl.ConfirmCopy()
	all := a.jobState.AllJobs()
	thistle := filepath.Join(orchard, "thistle.txt")
	if len(all) != 1 || all[0].Type != jobs.TypeCopy || len(all[0].Sources) != 1 || all[0].Sources[0].String() != thistle {
		t.Fatalf("jobs = %+v", all)
	}
	// A copy keeps the file listed; its marks go, the panel selection stays.
	if len(st.Marked) != 0 || !slices.Contains(uniqueFiles(a), "orchard/thistle.txt") || !a.activePanel().SelectedPaths[panelSelected] {
		t.Fatalf("after copy: marks=%v files=%v selection=%v", st.Marked, uniqueFiles(a), a.activePanel().SelectedPaths)
	}

	// A move drops the file from the list.
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "thistle.txt")
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	a.dispatch(keymap.ActionMove)
	a.dialogCtrl.TransferDialogOK()
	if slices.Contains(uniqueFiles(a), "orchard/thistle.txt") || a.model.ViewMode != ui.ViewUnique {
		t.Fatalf("after move: files=%v view=%v", uniqueFiles(a), a.model.ViewMode)
	}
}

func TestUniqueViewDeleteConfirmsAndDropsFiles(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "harbor.txt")
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	a.dispatch(keymap.ActionFileDelete)
	fd := a.model.FileDialog
	harbor := filepath.Join(dir, "harbor.txt")
	if !fd.Open || fd.Note != uniqueDeleteNote || len(fd.DeleteEntries) != 1 || fd.DeleteEntries[0].Path != harbor {
		t.Fatalf("delete dialog: %+v", fd)
	}
	a.dialogCtrl.ExecuteDelete()
	all := a.jobState.AllJobs()
	if len(all) != 1 || all[0].Type != jobs.TypeDelete || all[0].Sources[0].String() != harbor {
		t.Fatalf("jobs = %+v", all)
	}
	if slices.Contains(uniqueFiles(a), "harbor.txt") || a.model.FileDialog.Open {
		t.Fatalf("files = %v dialog open = %v", uniqueFiles(a), a.model.FileDialog.Open)
	}
}

// A delete that would empty directories asks first: Yes removes them, No keeps them, Cancel deletes
// nothing and keeps the marks. Only the question's answer queues the job.
func TestUniqueViewDeleteAsksAboutEmptyDirs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		answer     dlg.ID
		wantJob    bool
		wantRemove bool
	}{
		{"yes", dialog.DedupEmptyDirsYes, true, true},
		{"default", "", true, true},
		{"no", dialog.DedupEmptyDirsNo, true, false},
		{"cancel", dlg.Cancel, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, screen, _, _, willow := uniqueApp(t)
			if err := os.Remove(filepath.Join(willow, "sedge.txt")); err != nil {
				t.Fatal(err)
			}
			a.activePanel().AddSelection(willow)
			openUnique(t, a, screen)
			st := &a.model.UniqueView
			bramble := filepath.Join(willow, "bramble.txt")
			st.SetMark(ui.UniqueFile{Path: bramble, Size: 5}, true)
			a.dispatch(keymap.ActionFileDelete)
			a.dialogCtrl.ExecuteDelete()
			if q := a.model.DedupEmptyDirsConfirm; !q.Open || q.Focus != dialog.DedupEmptyDirsYes || !slices.Equal(q.Dirs, []string{"willow"}) {
				t.Fatalf("question = %+v", q)
			}
			if n := len(a.jobState.AllJobs()); n != 0 {
				t.Fatalf("%d jobs before the answer", n)
			}
			a.answerDedupEmptyDirsConfirm(tc.answer)
			all := a.jobState.AllJobs()
			if a.model.DedupEmptyDirsConfirm.Open || (len(all) == 1) != tc.wantJob {
				t.Fatalf("open = %v jobs = %+v", a.model.DedupEmptyDirsConfirm.Open, all)
			}
			if tc.wantJob {
				if all[0].Sources[0].String() != bramble || all[0].DeleteRemoveEmptyDirs != tc.wantRemove {
					t.Fatalf("job = %+v", all[0])
				}
				return
			}
			if !st.Marked[bramble] || !slices.Contains(uniqueFiles(a), "willow/bramble.txt") {
				t.Fatalf("cancel: marks = %v files = %v", st.Marked, uniqueFiles(a))
			}
			a.uniqueCtrl.FinishDelete(true) // nothing is left waiting
			if n := len(a.jobState.AllJobs()); n != 0 {
				t.Fatalf("%d jobs after a cancel", n)
			}
		})
	}
}

// The question and the job climb through emptied parents and stop at the checked directory.
func TestUniqueViewEmptyDirsClimbToScope(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	deep := filepath.Join(dir, "marsh", "north", "east")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	fern := filepath.Join(deep, "fern.txt")
	if err := os.WriteFile(fern, []byte("words"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("9", 32)
	if _, err := d.CommitScanBatch([]db.ScannedFile{{Dir: deep, Name: "fern.txt", Size: 5, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	openUnique(t, a, screen)
	a.model.UniqueView.SetMark(ui.UniqueFile{Path: fern, Size: 5}, true)
	a.dispatch(keymap.ActionFileDelete)
	a.dialogCtrl.ExecuteDelete()
	want := []string{"marsh/north/east", "marsh/north", "marsh"}
	if got := a.model.DedupEmptyDirsConfirm.Dirs; !slices.Equal(got, want) {
		t.Fatalf("question = %v, want %v", got, want)
	}
	a.answerDedupEmptyDirsConfirm(dialog.DedupEmptyDirsYes)
	flushBackgroundJobs(t, a)
	if _, err := os.Stat(filepath.Join(dir, "marsh")); !os.IsNotExist(err) {
		t.Fatalf("marsh still there: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the scope directory went: %v", err)
	}
	all := a.jobState.AllJobs()
	if len(all) != 1 || len(all[0].DeleteEmptyDirStops) != 1 || all[0].DeleteEmptyDirStops[0].String() != dir {
		t.Fatalf("jobs = %+v", all)
	}
}

// Leaving keeps the results and marks; the same scope shows them without a new lookup.
func TestUniqueViewKeepsResultsOnLeave(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	marks := len(st.Marked)
	a.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.model.ViewMode != ui.ViewBrowser || !strings.Contains(a.model.Message, "kept") {
		t.Fatalf("view=%v message=%q", a.model.ViewMode, a.model.Message)
	}
	st.Files = append(st.Files, ui.UniqueFile{Path: filepath.Join(dir, "sentinel")})
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.ViewMode == ui.ViewUnique })
	if st.Loading || len(st.Marked) != marks || !slices.Contains(uniqueFiles(a), "sentinel") {
		t.Fatalf("view=%v loading=%v marked=%v files=%v", a.model.ViewMode, st.Loading, st.Marked, uniqueFiles(a))
	}
	// The opener in the view closes it, like the other display views.
	a.handleKey(findUniqueChord(t, a))
	if a.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("view = %v", a.model.ViewMode)
	}
}

// Refresh keeps what still applies; a stale lookup lands on nothing.
func TestUniqueViewRefresh(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	a.handleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) // orchard stays open across the refresh
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "thistle.txt")
	thistle := filepath.Join(orchard, "thistle.txt")
	a.uniqueCtrl.SelectToggleAndAdvance()
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "thistle.txt")
	st.SetMark(ui.UniqueFile{Path: filepath.Join(dir, "gone.txt"), Size: 7}, true)

	a.uniqueCtrl.HandleWake(uniquefilesctrl.LookupPayload{Gen: -1, Result: fi.UniqueResult{}})
	if len(st.Files) != 6 {
		t.Fatalf("stale lookup landed: %v", uniqueFiles(a))
	}
	a.dispatch(keymap.ActionPanelRefresh)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return !st.Loading })
	if !st.Marked[thistle] || st.Marked[filepath.Join(dir, "gone.txt")] || st.MarkedBytes != 5 {
		t.Fatalf("marks = %v bytes = %d", st.Marked, st.MarkedBytes)
	}
	if r := a.model.UniqueList[st.Pane.Selected]; r.Value.Path != thistle {
		t.Fatalf("cursor on %q, want %q", r.Value.Path, thistle)
	}
}

// The destination is the real inactive panel: browsing it from the view moves that panel.
func TestUniqueViewDestinationPanelFocus(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	if err := a.navigatePanelToDirectory(a.inactivePanelID(), dir, ""); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return !a.inactivePanel().ListingPending })
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	active := a.model.ActivePanel
	a.dispatch(keymap.ActionPanelSwitch)
	if !st.FocusPanel {
		t.Fatal("tab did not focus the destination")
	}
	p := a.inactivePanel()
	if !p.SelectVisibleEntry("orchard") {
		t.Fatal("no orchard entry in the destination")
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return p.PathString() == orchard && !p.ListingPending })
	if a.model.ActivePanel != active || a.model.ViewMode != ui.ViewUnique {
		t.Fatalf("active = %d view = %v", a.model.ActivePanel, a.model.ViewMode)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("view = %v", a.model.ViewMode)
	}
}

func TestUniqueViewHashUnverifiedQueuesAndRefreshes(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("UPDATE instance SET file_id = NULL WHERE filename = ?", "harbor.txt"); err != nil {
		t.Fatal(err)
	}
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	harbor := filepath.Join(dir, "harbor.txt")
	if !slices.Contains(st.ToHash, harbor) || st.UnverifiedCount() != 1 {
		t.Fatalf("toHash = %v unverified = %d", st.ToHash, st.UnverifiedCount())
	}
	a.dispatch(keymap.ActionUniqueHashUnverified)
	if len(a.jobState.AllJobs()) != 1 {
		t.Fatalf("jobs = %+v", a.jobState.AllJobs())
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool { return st.UnverifiedCount() == 0 && !st.Loading })
}

// Marks across directories get the multi-location preview, relative to their common root.
func TestUniqueViewCopyAcrossDirectoriesPreview(t *testing.T) {
	a, screen, dir, orchard, willow := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	st.SetMark(ui.UniqueFile{Path: filepath.Join(orchard, "thistle.txt"), Size: 5}, true)
	st.SetMark(ui.UniqueFile{Path: filepath.Join(willow, "bramble.txt"), Size: 5}, true)
	a.dispatch(keymap.ActionCopy)
	d := a.model.TransferDialog
	if !d.Open || d.CommonRoot != dir || len(d.Entries) != 2 {
		t.Fatalf("root = %q entries = %+v", d.CommonRoot, d.Entries)
	}
	if got := a.dialogCtrl.ActivePanelSources(); len(got) != 2 {
		t.Fatalf("sources = %v", got)
	}
}

// The viewer returns to the view, as it does for the duplicates view; a directory has nothing to view.
func TestUniqueViewFileViewerReturns(t *testing.T) {
	a, screen, _, _, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	a.dispatch(keymap.ActionFileView)
	if a.model.ViewMode != ui.ViewUnique || a.model.Message != "View: not a file" {
		t.Fatalf("view = %v message = %q", a.model.ViewMode, a.model.Message)
	}
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "harbor.txt")
	a.dispatch(keymap.ActionFileView)
	if a.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("view = %v, want the viewer", a.model.ViewMode)
	}
	a.previewCtrl.CloseFilePreviewFullscreen()
	if a.model.ViewMode != ui.ViewUnique {
		t.Fatalf("view after closing the viewer = %v", a.model.ViewMode)
	}
}

// The delete summary is the view's own, not recomputed from the panel's cursor entry.
func TestUniqueViewDeleteSummaryIsNotReplaced(t *testing.T) {
	a, screen, _, _, _ := uniqueApp(t)
	openUnique(t, a, screen)
	st := &a.model.UniqueView
	st.Pane.Selected = slices.Index(uniqueRowTexts(a), "harbor.txt")
	a.dispatch(keymap.ActionFileDelete)
	want := a.model.FileDialog.DeleteSummary
	a.dialogCtrl.RefreshDeleteDialogSummary()
	if a.model.FileDialog.DeleteSummary != want || a.model.FileDialog.DeleteEntries[0].Name != "harbor.txt" {
		t.Fatalf("summary = %q, want %q; entries = %+v", a.model.FileDialog.DeleteSummary, want, a.model.FileDialog.DeleteEntries)
	}
}

// findUniqueChord is the key event the action is bound to.
func findUniqueChord(t *testing.T, a *App) *tcell.EventKey {
	t.Helper()
	ch, err := keymap.ParseKey(a.keys.Global.MenuBindingLabel(keymap.ActionFileIndexFindUnique))
	if err != nil {
		t.Fatal(err)
	}
	return tcell.NewEventKey(ch.Key, ch.Rune, ch.Mod)
}
