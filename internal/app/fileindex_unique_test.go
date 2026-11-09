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

	fileindexctrl "github.com/paranoidi/paras-commander/internal/apphandler/fileindex"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// uniqueApp is fileIndexApp plus two indexed subdirectories: orchard/thistle.txt and
// willow/bramble.txt are unique, orchard/clover.txt has its copy in willow/sedge.txt.
func uniqueApp(t *testing.T) (a *App, screen tcell.SimulationScreen, dir, orchard, willow string) {
	t.Helper()
	a, screen, dir, _ = fileIndexApp(t)
	orchard, willow = filepath.Join(dir, "orchard"), filepath.Join(dir, "willow")
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	scanned := func(dir, name, fill string) db.ScannedFile {
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
	for _, sub := range []string{orchard, willow} {
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return a, screen, dir, orchard, willow
}

// openFindUnique runs the action and waits for its list to land.
func openFindUnique(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		st := a.model.IndexFindDialog
		return st.Open && !st.Searching
	})
}

func uniqueRows(a *App) []string {
	var out []string
	for _, r := range a.model.IndexFindDialog.Results {
		out = append(out, r.Rel)
	}
	return out
}

func TestFindUniqueScopesToPanelDirectory(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	openFindUnique(t, a, screen)
	st := &a.model.IndexFindDialog
	if !st.UniqueMode || st.Root != dir || !slices.Equal(st.SelectionDirRoots, []string{dir}) {
		t.Fatalf("state = %+v", st)
	}
	// The panel directory holds every copy, so all six files are listed from an empty query.
	if got := uniqueRows(a); len(got) != 6 {
		t.Fatalf("rows = %v", got)
	}
	typeIndexFindQuery(t, a, screen, "THISTLE", 1)
	if got := uniqueRows(a); !slices.Equal(got, []string{"orchard/thistle.txt"}) {
		t.Fatalf("filtered rows = %v", got)
	}
}

func TestFindUniqueScopesToSelectedDirectories(t *testing.T) {
	a, screen, _, orchard, willow := uniqueApp(t)
	a.activePanel().AddSelection(orchard)
	openFindUnique(t, a, screen)
	if got := uniqueRows(a); !slices.Equal(got, []string{"orchard/thistle.txt"}) {
		t.Fatalf("one directory: rows = %v (clover has a copy in willow)", got)
	}
	a.fileindexCtrl.CloseFindDialog()

	// Both selected: the copy is inside the scope, so clover and sedge are listed.
	a.activePanel().AddSelection(willow)
	openFindUnique(t, a, screen)
	want := []string{"orchard/clover.txt", "orchard/thistle.txt", "willow/bramble.txt", "willow/sedge.txt"}
	if got := uniqueRows(a); !slices.Equal(got, want) {
		t.Fatalf("two directories: rows = %v, want %v", got, want)
	}
}

func TestFindUniqueRefusesMixedSelection(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	a.activePanel().AddSelection(orchard)
	a.activePanel().AddSelection(filepath.Join(dir, "harbor.txt"))
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message != "" })
	if a.model.IndexFindDialog.Open || a.model.MessageUrgency != ui.MessageUrgencyCritical || !strings.Contains(a.model.Message, "select only") {
		t.Fatalf("open=%v urgency=%v message=%q", a.model.IndexFindDialog.Open, a.model.MessageUrgency, a.model.Message)
	}
}

func TestFindUniqueIgnoresSelectionOfOnlyFiles(t *testing.T) {
	a, screen, dir, _, _ := uniqueApp(t)
	selected := filepath.Join(dir, "harbor.txt")
	a.activePanel().AddSelection(selected)
	openFindUnique(t, a, screen)
	if st := a.model.IndexFindDialog; !slices.Equal(st.SelectionDirRoots, []string{dir}) {
		t.Fatalf("scope = %v, want the panel directory", st.SelectionDirRoots)
	}
	if !a.activePanel().SelectedPaths[selected] {
		t.Fatal("the selection must stay")
	}
}

func TestFindUniqueRefusesIndexRoot(t *testing.T) {
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
	if a.model.IndexFindDialog.Open || !strings.Contains(a.model.Message, "nothing outside") {
		t.Fatalf("open=%v message=%q", a.model.IndexFindDialog.Open, a.model.Message)
	}
}

// A selected fileindex:// directory off the listing still counts as a directory.
func TestFindUniqueClassifiesIndexSelection(t *testing.T) {
	a, screen, dir, orchard, _ := uniqueApp(t)
	a.dispatch(keymap.ActionFileIndexBrowse)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == "fileindex://"+dir && !p.ListingPending
	})
	a.activePanel().AddSelection("fileindex://" + orchard)
	openFindUnique(t, a, screen)
	if st := a.model.IndexFindDialog; !slices.Equal(st.SelectionDirRoots, []string{orchard}) || !st.FromIndexPanel {
		t.Fatalf("scope = %v fromIndex=%v", st.SelectionDirRoots, st.FromIndexPanel)
	}
	a.fileindexCtrl.CloseFindDialog()
	a.model.Message = ""
	a.activePanel().AddSelection("fileindex://" + filepath.Join(dir, "willow", "bramble.txt"))
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message != "" })
	if a.model.IndexFindDialog.Open {
		t.Fatal("a mixed index selection opened the dialog")
	}
}

func TestFindUniqueMarksAddSelections(t *testing.T) {
	a, screen, _, orchard, _ := uniqueApp(t)
	a.activePanel().AddSelection(orchard)
	openFindUnique(t, a, screen)
	st := &a.model.IndexFindDialog
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if st.MarkedCount() != 1 {
		t.Fatalf("marked = %d", st.MarkedCount())
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if st.Open || !a.activePanel().SelectedPaths[filepath.Join(orchard, "thistle.txt")] {
		t.Fatalf("open=%v selection=%v", st.Open, a.activePanel().SelectedPaths)
	}
}

// A closed dialog's lookup lands on nothing, and a query typed while it runs takes effect when it
// lands: the query's own generation must not make the lookup stale.
func TestFindUniqueLookupGenerations(t *testing.T) {
	a, screen, _, _, _ := uniqueApp(t)
	openFindUnique(t, a, screen)
	a.fileindexCtrl.CloseFindDialog()
	a.fileindexCtrl.HandleUnique(fileindexctrl.UniquePayload{Result: fi.UniqueResult{Files: []db.Found{{Path: "/x"}}}})
	if st := a.model.IndexFindDialog; st.Searching || len(st.Results) != 0 {
		t.Fatalf("stale lookup landed: %+v", st)
	}

	a.dispatch(keymap.ActionFileIndexFindUnique)
	for !a.model.IndexFindDialog.Open {
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexFindDialog.Open })
	}
	for _, r := range "bramble" {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		st := a.model.IndexFindDialog
		return !st.Searching && len(st.Results) == 1
	})
	if got := uniqueRows(a); !slices.Equal(got, []string{"willow/bramble.txt"}) {
		t.Fatalf("rows = %v", got)
	}
}

func TestFindUniqueParkRestoreAndRelocate(t *testing.T) {
	a, screen, _, orchard, _ := uniqueApp(t)
	openFindUnique(t, a, screen)
	typeIndexFindQuery(t, a, screen, "thistle", 1)
	st := &a.model.IndexFindDialog

	// The opener action parks and brings it back as it was.
	a.fileindexCtrl.HandleFindKey(findUniqueChord(t, a))
	if st.Open || !st.Parked {
		t.Fatalf("open=%v parked=%v", st.Open, st.Parked)
	}
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return st.Open })
	if st.Parked || st.Query != "thistle" || len(st.Results) != 1 {
		t.Fatalf("restored %+v", st)
	}

	// Another scope asks; New search starts over on it.
	a.fileindexCtrl.HandleFindKey(findUniqueChord(t, a))
	a.activePanel().AddSelection(orchard)
	a.dispatch(keymap.ActionFileIndexFindUnique)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.FindRelocate.Open })
	if !a.model.FindRelocate.Unique || st.Open {
		t.Fatalf("relocate = %+v open=%v", a.model.FindRelocate, st.Open)
	}
	a.answerFindRelocate(dialog.FindRelocateNew)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return st.Open && !st.Searching })
	if !st.UniqueMode || st.Query != "" || !slices.Equal(st.SelectionDirRoots, []string{orchard}) {
		t.Fatalf("new search = %+v", st)
	}

	// The plain Find in index does not take a parked Find unique files for itself.
	a.fileindexCtrl.HandleFindKey(findUniqueChord(t, a))
	a.dispatch(keymap.ActionFileIndexFind)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.FindRelocate.Open })
	if a.model.FindRelocate.Unique {
		t.Fatalf("relocate = %+v", a.model.FindRelocate)
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
