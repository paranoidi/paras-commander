package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func findRankedIndexForPath(st *ui.FindDialogState, absPath string) int {
	want := filepath.Clean(absPath)
	for i, entIdx := range st.Ranked {
		if entIdx >= 0 && entIdx < len(st.Entries) && filepath.Clean(st.Entries[entIdx].AbsPath(st.RootPath)) == want {
			return i
		}
	}
	return 0
}

func waitFindIndexDone(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.pollFindUpdates(findctrl.WakePayload{})
		app.handleFindThrottleRankWake()
		app.handleFindDebounceRankWake()
		app.applyFindRank()
		if !app.model.FindDialog.Open {
			t.Fatal("find dialog closed unexpectedly")
		}
		if app.model.FindDialog.IndexDone && !app.model.FindDialog.RankPending {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("find index did not finish in time")
}

func waitFindRankDone(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.handleFindThrottleRankWake()
		app.handleFindDebounceRankWake()
		if app.applyFindRank() {
			return
		}
		if !app.model.FindDialog.RankPending {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("find rank did not finish in time")
}

func TestFindDialogQueryAltVAltDToggleCheckboxes(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	st := &app.model.FindDialog
	if !st.StayOnCurrentVolume {
		t.Fatal("expected stay-on-volume default on")
	}
	if st.OnlyDirectories {
		t.Fatal("expected only-directories default off")
	}
	if st.OnlyFiles {
		t.Fatal("expected only-files default off")
	}

	for _, r := range "find" {
		app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if st.Query != "find" {
		t.Fatalf("query = %q want find", st.Query)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModAlt))
	if st.StayOnCurrentVolume {
		t.Fatal("Alt+V should toggle stay-on-volume while typing filter")
	}
	if st.Focus != 0 {
		t.Fatalf("focus = %d want 0 after Alt+V", st.Focus)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt))
	if !st.OnlyDirectories {
		t.Fatal("Alt+D should toggle only-directories while typing filter")
	}
	if st.OnlyFiles {
		t.Fatal("Alt+D should not enable only-files")
	}
	if st.Focus != 0 {
		t.Fatalf("focus = %d want 0 after Alt+D", st.Focus)
	}

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModAlt))
	if !st.OnlyFiles {
		t.Fatal("Alt+L should toggle only-files while typing filter")
	}
	if st.OnlyDirectories {
		t.Fatal("Alt+L should clear only-directories")
	}
	if st.Focus != 0 {
		t.Fatalf("focus = %d want 0 after Alt+L", st.Focus)
	}
}

func TestFindDialogHandleKeyAltDDoesNotClearDiskUsage(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	if err := os.Mkdir(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scanned, "a.dat"))

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

	left := app.panelByID(ui.PrimaryPanel)
	app.startDiskUsageScanForPanel(ui.PrimaryPanel)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !app.model.DiskUsageShown {
		t.Fatal("expected disk usage to be shown after scan")
	}
	if _, ok := app.diskUsage.Size(scanned); !ok {
		t.Fatal("expected cached size for scanned directory")
	}

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	st := &app.model.FindDialog
	for _, r := range "find" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatal("handleKey quit while typing find query")
		}
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModAlt)); quit {
		t.Fatal("handleKey quit on Alt+D")
	}
	if !st.OnlyDirectories {
		t.Fatal("Alt+D via handleKey should toggle only-directories")
	}
	if !app.model.DiskUsageShown {
		t.Fatal("disk usage should remain shown after Find Alt+D")
	}
	if _, ok := app.diskUsage.Size(scanned); !ok {
		t.Fatal("disk usage cache should not be cleared by Find Alt+D")
	}
	if app.model.Message == "Disk usage data cleared" {
		t.Fatal("Find Alt+D must not trigger panel.disk-usage-clear")
	}
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

	app.openFindDialog(ui.PrimaryPanel)
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

	app.openFindDialog(ui.PrimaryPanel)
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

	app.openFindDialog(ui.PrimaryPanel)
	if !app.model.FindDialog.Open {
		t.Fatal("expected find dialog open")
	}
	waitFindIndexDone(t, app)

	for _, r := range "target" {
		app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	waitFindRankDone(t, app)
	if len(app.model.FindDialog.Ranked) == 0 {
		t.Fatal("expected fuzzy matches for target")
	}
	app.navigateFindCursor()
	if app.model.FindDialog.Open {
		t.Fatal("expected dialog closed")
	}
	wantDir := filepath.Clean(sub)
	if got := filepath.Clean(app.activePanel().Path.String()); got != wantDir {
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

	app.openFindDialog(ui.PrimaryPanel)
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
	p := app.panelByID(ui.PrimaryPanel)
	if !p.SelectedPaths[filepath.Clean(aPath)] || !p.SelectedPaths[filepath.Clean(bPath)] {
		t.Fatalf("panel selections = %v", p.SelectedPaths)
	}
}

func newFindDialogTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	return app
}

func assertFindDialogGroupSelectViaKey(t *testing.T, app *App, key tcell.Key, wantMode string) {
	t.Helper()
	if quit, _ := app.handleKey(tcell.NewEventKey(key, 0, tcell.ModNone)); quit {
		t.Fatalf("%v: handleKey quit", key)
	}
	if !app.model.GroupSelect.Open || app.model.GroupSelect.Mode != wantMode || app.model.GroupSelect.Context != "find" {
		t.Fatalf("%v: group select = %+v", key, app.model.GroupSelect)
	}
	if !app.model.FindDialog.Open {
		t.Fatal("find dialog should stay open under group select")
	}
	for _, r := range "*.txt" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatalf("handleKey(%q) quit", r)
		}
	}
	if got := app.model.GroupSelect.Text; got != "*.txt" {
		t.Fatalf("pattern after single key open = %q, want *.txt", got)
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)); quit {
		t.Fatal("Esc quit")
	}
	if app.model.GroupSelect.Open {
		t.Fatal("Esc should close group select")
	}
	if !app.model.FindDialog.Open {
		t.Fatal("find dialog should stay open after Esc on group select")
	}
}

func TestFindDialogF6OpensGroupSelectForFindContext(t *testing.T) {
	app := newFindDialogTestApp(t)
	if id, ok := app.keysFindDialog.Lookup(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone)); !ok || id != keymap.ActionFindSelectGroup {
		t.Fatalf("F6 lookup = %q ok=%v", id, ok)
	}
	assertFindDialogGroupSelectViaKey(t, app, tcell.KeyF6, "select")
}

func TestFindDialogF7OpensUnselectGroupSelectForFindContext(t *testing.T) {
	app := newFindDialogTestApp(t)
	if id, ok := app.keysFindDialog.Lookup(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone)); !ok || id != keymap.ActionFindUnselectGroup {
		t.Fatalf("F7 lookup = %q ok=%v", id, ok)
	}
	assertFindDialogGroupSelectViaKey(t, app, tcell.KeyF7, "unselect")
}

func TestFindDialogGroupSelectGlobMarksFullCorpusResults(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "alpha.txt")
	bPath := filepath.Join(root, "beta.go")
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
	for _, r := range "*.txt" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.GroupSelect.Open {
		t.Fatal("group select should close after OK")
	}
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(aPath)] {
		t.Fatalf("alpha.txt should be marked: %v", app.model.FindDialog.MarkedPaths)
	}
	if app.model.FindDialog.MarkedPaths[filepath.Clean(bPath)] {
		t.Fatalf("beta.go should not be marked: %v", app.model.FindDialog.MarkedPaths)
	}
}

func TestFindDialogUnselectAllClearsMarks(t *testing.T) {
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if len(app.model.FindDialog.MarkedPaths) != 2 {
		t.Fatalf("select all marks = %v", app.model.FindDialog.MarkedPaths)
	}

	if id, ok := app.keysFindDialog.Lookup(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone)); !ok || id != keymap.ActionFindUnselectAll {
		t.Fatalf("F4 lookup = %q ok=%v", id, ok)
	}
	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
	if app.model.FindDialog.MarkedPaths != nil {
		t.Fatalf("unselect all cleared marks = %v", app.model.FindDialog.MarkedPaths)
	}
}

func TestFindDialogSelectAllMarksFullCorpusResults(t *testing.T) {
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)

	if id, ok := app.keysFindDialog.Lookup(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); !ok || id != keymap.ActionFindSelectAll {
		t.Fatalf("F5 lookup = %q ok=%v", id, ok)
	}
	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(aPath)] || !app.model.FindDialog.MarkedPaths[filepath.Clean(bPath)] {
		t.Fatalf("select all marks = %v", app.model.FindDialog.MarkedPaths)
	}

	app.model.FindDialog.MarkedPaths = nil
	if id, ok := app.keysFindDialog.Lookup(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl)); !ok || id != keymap.ActionFindSelectAll {
		t.Fatalf("Ctrl+A lookup = %q ok=%v", id, ok)
	}
	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(aPath)] || !app.model.FindDialog.MarkedPaths[filepath.Clean(bPath)] {
		t.Fatalf("Ctrl+A select all marks = %v", app.model.FindDialog.MarkedPaths)
	}
}

func TestFindDialogBulkSelectAllManyFiles(t *testing.T) {
	const n = 10000
	root := t.TempDir()
	for i := range n {
		name := filepath.Join(root, fmt.Sprintf("bulk_%05d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	waitFindRankDone(t, app)

	st := &app.model.FindDialog
	if len(st.PathIsDir) < n {
		t.Fatalf("PathIsDir len = %d, want >= %d", len(st.PathIsDir), n)
	}

	deadline := time.Now().Add(time.Second)
	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if time.Now().After(deadline) {
		t.Fatal("F5 select-all took too long")
	}
	if len(st.MarkedPaths) != n {
		t.Fatalf("marked = %d, want full corpus %d", len(st.MarkedPaths), n)
	}
}

func TestFindDialogBulkSelectAllMixedTree(t *testing.T) {
	const dirs = 100
	const filesPerDir = 100
	wantMarked := dirs * filesPerDir // walk-order: dirs replaced by descendant files
	root := t.TempDir()
	for d := range dirs {
		dir := filepath.Join(root, fmt.Sprintf("mixed_%03d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := range filesPerDir {
			name := filepath.Join(dir, fmt.Sprintf("file_%03d.txt", f))
			if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	waitFindRankDone(t, app)

	st := &app.model.FindDialog
	if len(st.Entries) < dirs+wantMarked {
		t.Fatalf("entries = %d, want >= %d", len(st.Entries), dirs+wantMarked)
	}

	deadline := time.Now().Add(time.Second)
	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if time.Now().After(deadline) {
		t.Fatal("F5 select-all on mixed tree took too long")
	}
	if len(st.MarkedPaths) != wantMarked {
		t.Fatalf("marked = %d, want full corpus %d", len(st.MarkedPaths), wantMarked)
	}
}

func TestFindDialogBulkOKApplyManyFiles(t *testing.T) {
	const n = 800
	root := t.TempDir()
	for i := range n {
		name := filepath.Join(root, fmt.Sprintf("ok_%04d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	waitFindRankDone(t, app)

	app.handleFindDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	st := &app.model.FindDialog
	if len(st.MarkedPaths) != n {
		t.Fatalf("marked = %d, want %d", len(st.MarkedPaths), n)
	}

	deadline := time.Now().Add(3 * time.Second)
	app.activateFindDialogOK()
	if time.Now().After(deadline) {
		t.Fatal("OK apply took too long")
	}
	if app.model.FindDialog.Open {
		t.Fatal("expected dialog closed")
	}
	p := app.panelByID(ui.PrimaryPanel)
	if len(p.SelectedPaths) != n {
		t.Fatalf("panel selected = %d, want %d", len(p.SelectedPaths), n)
	}
}

func TestFindDialogBulkGroupSelectManyFiles(t *testing.T) {
	const n = 400
	root := t.TempDir()
	for i := range n {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("match_%04d.txt", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("skip_%04d.go", i)), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	waitFindRankDone(t, app)

	st := &app.model.FindDialog
	wantEntries := n * 2
	if st.IndexedCount != wantEntries {
		t.Fatalf("indexed = %d, want %d", st.IndexedCount, wantEntries)
	}
	deadline := time.Now().Add(3 * time.Second)
	app.findCtrl.ApplyGroupSelect("select", "*.txt", false, false, false, panel.GroupPatternShell)
	if time.Now().After(deadline) {
		t.Fatal("group select took too long")
	}
	marked := 0
	for path := range st.MarkedPaths {
		if strings.HasSuffix(filepath.Base(path), ".txt") {
			marked++
		}
	}
	if marked != n {
		t.Fatalf("marked txt = %d, want %d in full corpus", marked, n)
	}
}

func findRankedNonDirCount(st *ui.FindDialogState) int {
	n := 0
	for _, idx := range st.Ranked {
		if idx >= 0 && idx < len(st.Entries) && !st.Entries[idx].IsDir {
			n++
		}
	}
	return n
}

func findRankedDirCount(st *ui.FindDialogState) int {
	n := 0
	for _, idx := range st.Ranked {
		if idx >= 0 && idx < len(st.Entries) && st.Entries[idx].IsDir {
			n++
		}
	}
	return n
}

func TestFindDialogOnlyDirectoriesFiltersResults(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "module.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	st := &app.model.FindDialog
	if findRankedNonDirCount(st) == 0 {
		t.Fatal("expected file entries in ranked list before filter")
	}
	totalRanked := len(st.Ranked)

	app.toggleFindOnlyDirectories()
	if !st.OnlyDirectories {
		t.Fatal("expected only-directories on")
	}
	if findRankedNonDirCount(st) != 0 {
		t.Fatalf("ranked still has %d file entries after filter", findRankedNonDirCount(st))
	}
	if len(st.Ranked) >= totalRanked {
		t.Fatalf("ranked len = %d want fewer than %d", len(st.Ranked), totalRanked)
	}

	app.toggleFindOnlyDirectories()
	if st.OnlyDirectories {
		t.Fatal("expected only-directories off")
	}
	if findRankedNonDirCount(st) == 0 {
		t.Fatal("expected file entries back after clearing filter")
	}
	if len(st.Ranked) != totalRanked {
		t.Fatalf("ranked len = %d want %d after filter off", len(st.Ranked), totalRanked)
	}
}

func TestFindDialogOnlyFilesFiltersResults(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "module.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("x"), 0o644); err != nil {
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)
	st := &app.model.FindDialog
	if findRankedDirCount(st) == 0 {
		t.Fatal("expected directory entries in ranked list before filter")
	}
	totalRanked := len(st.Ranked)

	app.toggleFindOnlyFiles()
	if !st.OnlyFiles {
		t.Fatal("expected only-files on")
	}
	if findRankedDirCount(st) != 0 {
		t.Fatalf("ranked still has %d directory entries after filter", findRankedDirCount(st))
	}
	if len(st.Ranked) >= totalRanked {
		t.Fatalf("ranked len = %d want fewer than %d", len(st.Ranked), totalRanked)
	}

	app.toggleFindOnlyFiles()
	if st.OnlyFiles {
		t.Fatal("expected only-files off")
	}
	if findRankedDirCount(st) == 0 {
		t.Fatal("expected directory entries back after clearing filter")
	}
	if len(st.Ranked) != totalRanked {
		t.Fatalf("ranked len = %d want %d after filter off", len(st.Ranked), totalRanked)
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

	app.openFindDialog(ui.PrimaryPanel)
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

	app.openFindDialog(ui.PrimaryPanel)
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

	app.openFindDialog(ui.PrimaryPanel)
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

func TestFindDialogMarkParentThenChildDirRemovesParentMark(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	nested := filepath.Join(sub, "nested")
	for _, d := range []string{sub, nested} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
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

	app.openFindDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)

	ev := tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)
	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, sub)
	app.handleFindDialogKey(ev)
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(sub)] {
		t.Fatal("sub should be marked")
	}

	app.model.FindDialog.Selected = findRankedIndexForPath(&app.model.FindDialog, nested)
	app.handleFindDialogKey(ev)
	if app.model.FindDialog.MarkedPaths[filepath.Clean(sub)] {
		t.Fatal("parent dir mark should be removed when nested child dir is marked")
	}
	if !app.model.FindDialog.MarkedPaths[filepath.Clean(nested)] {
		t.Fatal("nested dir should be marked")
	}
	if app.model.Message != "Removed conflicting selections" {
		t.Fatalf("status = %q", app.model.Message)
	}
}

func findIndexedUnder(st *ui.FindDialogState, dir string) bool {
	dir = filepath.Clean(dir)
	for _, e := range st.Entries {
		p := filepath.Clean(e.AbsPath(st.RootPath))
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

	app.panelByID(ui.PrimaryPanel).AddSelection(filepath.Clean(dirA))
	app.openFindDialog(ui.PrimaryPanel)
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

	app.panelByID(ui.PrimaryPanel).AddSelection(filepath.Clean(dirA))
	app.openFindDialog(ui.PrimaryPanel)
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

	app.panelByID(ui.PrimaryPanel).AddSelection(filepath.Clean(f))
	app.openFindDialog(ui.PrimaryPanel)
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
	app.model.ActivePanel = ui.SecondaryPanel
	app.openFindDialog(ui.PrimaryPanel)
	if app.model.FindDialog.PanelID != ui.PrimaryPanel {
		t.Fatalf("panel id = %d want left", app.model.FindDialog.PanelID)
	}
	app.closeFindDialog()
}
