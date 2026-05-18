package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func findRankedIndexForPath(st *ui.FindDialogState, absPath string) int {
	want := filepath.Clean(absPath)
	for i, entIdx := range st.Ranked {
		if entIdx >= 0 && entIdx < len(st.Entries) && filepath.Clean(st.Entries[entIdx].Path) == want {
			return i
		}
	}
	return 0
}

func waitFindIndexDone(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.pollFindUpdates(findWakePayload{})
		if !app.model.FindDialog.Open {
			t.Fatal("find dialog closed unexpectedly")
		}
		if app.model.FindDialog.IndexDone {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("find index did not finish in time")
}

func TestFindDialogQueryAltBAltFCtrlL(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)

	for _, r := range "foo bar" {
		app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	st := &app.model.FindDialog
	if st.Query != "foo bar" || st.QueryCursor != 7 {
		t.Fatalf("after type: query=%q cursor=%d", st.Query, st.QueryCursor)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt))
	if st.QueryCursor != 4 {
		t.Fatalf("after Alt+b: cursor=%d want 4", st.QueryCursor)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if st.QueryCursor != 7 {
		t.Fatalf("after Alt+f: cursor=%d want 7", st.QueryCursor)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if st.Query != "" || st.QueryCursor != 0 || st.QueryScroll != 0 {
		t.Fatalf("after Ctrl+L: query=%q cursor=%d scroll=%d", st.Query, st.QueryCursor, st.QueryScroll)
	}
}

func TestFindDialogQueryHomeEndAndCtrlHomeEndList(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "beta.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)
	st := &app.model.FindDialog
	if len(st.Ranked) < 2 {
		t.Fatalf("ranked len = %d, want at least 2 entries", len(st.Ranked))
	}

	for _, r := range "ab" {
		app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if st.QueryCursor != 2 {
		t.Fatalf("cursor after type = %d want 2", st.QueryCursor)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if st.QueryCursor != 0 {
		t.Fatalf("after Home: cursor=%d want 0", st.QueryCursor)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if st.QueryCursor != 2 {
		t.Fatalf("after End: cursor=%d want 2", st.QueryCursor)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if len(st.Ranked) < 2 {
		t.Fatalf("ranked len after clear = %d, want at least 2", len(st.Ranked))
	}
	st.Selected = len(st.Ranked) - 1
	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl))
	if st.Selected != 0 {
		t.Fatalf("after Ctrl+Home: selected=%d want 0", st.Selected)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl))
	if st.Selected != len(st.Ranked)-1 {
		t.Fatalf("after Ctrl+End: selected=%d want %d", st.Selected, len(st.Ranked)-1)
	}
}

func TestFindDialogSelectsFile(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sub, "target.go")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	if !app.model.FindDialog.Open {
		t.Fatal("expected find dialog open")
	}
	waitFindIndexDone(t, app)

	for _, r := range "target" {
		app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if len(app.model.FindDialog.Ranked) == 0 {
		t.Fatal("expected fuzzy matches for target")
	}
	app.navigateFindCursor()
	if app.model.FindDialog.Open {
		t.Fatal("expected dialog closed")
	}
	wantDir := filepath.Clean(sub)
	if got := filepath.Clean(app.activePanel().Path); got != wantDir {
		t.Fatalf("panel dir = %q want %q", got, wantDir)
	}
	entry, ok := app.activePanel().CurrentEntry()
	if !ok || entry.Name != "target.go" {
		t.Fatalf("cursor entry = %+v ok=%v, want target.go", entry, ok)
	}
}

func TestFindDialogInsertMarksAndOKAddsToPanelSelection(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "alpha.txt")
	bPath := filepath.Join(root, "beta.txt")
	if err := os.WriteFile(aPath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)

	km := defaultKeymap(t)
	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)
	if id, ok := km.Lookup(ev); !ok || id != keymap.ActionPanelSelectToggle {
		t.Fatalf("Insert lookup = %q ok=%v", id, ok)
	}
	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, aPath)
	app.handleFindDialogKey(ev)
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(aPath)] {
		t.Fatalf("alpha should be marked: %v", app.model.FindDialog.MarkedPaths)
	}

	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, bPath)
	app.handleFindDialogKey(ev)
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(bPath)] {
		t.Fatal("beta should be marked")
	}

	app.activateFindDialogOK()
	if app.model.FindDialog.Open {
		t.Fatal("expected dialog closed")
	}
	p := app.panelByID(ui.LeftPanel)
	if !p.SelectedPaths[filepath.Clean(aPath)] || !p.SelectedPaths[filepath.Clean(bPath)] {
		t.Fatalf("panel selections = %v", p.SelectedPaths)
	}
}

func TestFindDialogStayOnVolumeRestartClearsEntries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)
	if len(app.model.FindDialog.Entries) == 0 {
		t.Fatal("expected at least one indexed entry")
	}

	app.toggleFindStayOnVolume()
	if app.model.FindDialog.Indexing {
		waitFindIndexDone(t, app)
	}
	if len(app.model.FindDialog.Entries) == 0 {
		t.Fatal("expected entries after restart")
	}
}

func TestFindDialogMarkDirRemovesDescendantMarks(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(sub, "child.txt")
	if err := os.WriteFile(child, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)

	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)
	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, child)
	app.handleFindDialogKey(ev)
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(child)] {
		t.Fatal("child should be marked")
	}

	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, sub)
	app.handleFindDialogKey(ev)
	if app.model.FindDialog.MarkedPaths[filepath.Clean(child)] {
		t.Fatal("child mark should be removed when parent dir is marked")
	}
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(sub)] {
		t.Fatal("sub should be marked")
	}
	if app.model.Message != "Removed conflicting selections" {
		t.Fatalf("status = %q", app.model.Message)
	}
}

func TestFindDialogMarkFileRemovesAncestorDirMarks(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(sub, "child.txt")
	if err := os.WriteFile(child, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)

	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)
	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, sub)
	app.handleFindDialogKey(ev)
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(sub)] {
		t.Fatal("sub should be marked")
	}

	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, child)
	app.handleFindDialogKey(ev)
	if app.model.FindDialog.MarkedPaths[filepath.Clean(sub)] {
		t.Fatal("parent dir mark should be removed when child file is marked")
	}
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(child)] {
		t.Fatal("child should be marked")
	}
	if app.model.Message != "Removed conflicting selections" {
		t.Fatalf("status = %q", app.model.Message)
	}
}

func findIndexedUnder(st *ui.FindDialogState, dir string) bool {
	dir = filepath.Clean(dir)
	for _, e := range st.Entries {
		p := filepath.Clean(e.Path)
		if p == dir || panel.IsStrictPathDescendant(dir, p) {
			return true
		}
	}
	return false
}

func TestFindDialogSearchOnlySelectionsDefaultScoped(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")
	for _, d := range []string{dirA, dirB} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dirA, "in-a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "in-b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.panelByID(ui.LeftPanel).AddSelection(filepath.Clean(dirA))
	app.openFindDialog(ui.LeftPanel)
	if !app.model.FindDialog.ShowSearchSelectionsOption {
		t.Fatal("expected search-selections checkbox")
	}
	if !app.model.FindDialog.SearchOnlySelections {
		t.Fatal("expected search-only selections default on")
	}
	waitFindIndexDone(t, app)
	if !findIndexedUnder(&app.model.FindDialog, dirA) {
		t.Fatal("expected entries under selected dir a")
	}
	if findIndexedUnder(&app.model.FindDialog, dirB) {
		t.Fatal("did not expect entries under dir b")
	}
}

func TestFindDialogSearchOnlySelectionsWidenAndNarrow(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")
	for _, d := range []string{dirA, dirB} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dirA, "in-a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "in-b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.panelByID(ui.LeftPanel).AddSelection(filepath.Clean(dirA))
	app.openFindDialog(ui.LeftPanel)
	waitFindIndexDone(t, app)
	countScoped := len(app.model.FindDialog.Entries)

	app.toggleFindSearchOnlySelections()
	if app.model.FindDialog.SearchOnlySelections {
		t.Fatal("expected search-only off after toggle")
	}
	waitFindIndexDone(t, app)
	if !findIndexedUnder(&app.model.FindDialog, dirB) {
		t.Fatal("expected entries under b after widening scope")
	}
	if len(app.model.FindDialog.Entries) <= countScoped {
		t.Fatalf("entries should grow after widen: before=%d after=%d", countScoped, len(app.model.FindDialog.Entries))
	}

	app.toggleFindSearchOnlySelections()
	if !app.model.FindDialog.SearchOnlySelections {
		t.Fatal("expected search-only on after second toggle")
	}
	if findIndexedUnder(&app.model.FindDialog, dirB) {
		t.Fatal("did not expect entries under b after narrowing")
	}
	if !findIndexedUnder(&app.model.FindDialog, dirA) {
		t.Fatal("expected entries under a preserved after narrow")
	}
}

func TestFindDialogNoSearchSelectionsCheckboxForFilesOnly(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "only.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	app.panelByID(ui.LeftPanel).AddSelection(filepath.Clean(f))
	app.openFindDialog(ui.LeftPanel)
	if app.model.FindDialog.ShowSearchSelectionsOption {
		t.Fatal("checkbox should be hidden when only files are selected")
	}
	waitFindIndexDone(t, app)
	if len(app.model.FindDialog.Entries) == 0 {
		t.Fatal("expected full-tree index")
	}
}

func TestFindDialogScopedMenuUsesPanel(t *testing.T) {
	root := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	app.model.ActivePanel = ui.RightPanel
	app.openFindDialog(ui.LeftPanel)
	if app.model.FindDialog.PanelID != ui.LeftPanel {
		t.Fatalf("panel id = %d want left", app.model.FindDialog.PanelID)
	}
	app.closeFindDialog()
}
