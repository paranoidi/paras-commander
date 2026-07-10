package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestMenuBarPermissionHiddenInJobsView(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ViewMode = ui.ViewJobs
	if got := app.menuBarPermissionText(); got != "" {
		t.Fatalf("jobs view: menuBarPermissionText() = %q, want empty", got)
	}

	app.model.ViewMode = ui.ViewBrowser
	if got := app.menuBarPermissionText(); got == "" {
		t.Fatal("browser view: menuBarPermissionText should show mode for selected entry")
	}
}

func TestHelpViewEnterRunsCopyLikeKeyboardShortcut(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openHelpDialog()
	if !app.model.HelpView.Open {
		t.Fatal("HelpView should open")
	}

	copyEntryIdx := -1
	for i, e := range app.model.HelpView.Entries {
		if e.ActionID == keymap.ActionCopy {
			copyEntryIdx = i
			break
		}
	}
	if copyEntryIdx < 0 {
		t.Fatal("help entries should include Copy action")
	}
	sel := -1
	for i, idx := range app.model.HelpView.Ranked {
		if idx == copyEntryIdx {
			sel = i
			break
		}
	}
	if sel < 0 {
		t.Fatal("ranked list should include Copy entry")
	}
	app.model.HelpView.Selected = sel
	app.model.HelpView.Focus = 0

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter on Copy should not quit")
	}
	if app.model.HelpView.Open {
		t.Fatal("HelpView should close after activating action")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindCopy {
		t.Fatal("Copy dialog should open (keyboard parity)")
	}
}

func TestActiveFooterKeysBrowserShowsF7JobsViewUsesJobsLegend(t *testing.T) {
	dir := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	browserKeys := menu.FunctionKeys
	if got := app.activeFooterKeys(); len(got) != len(browserKeys) {
		t.Fatalf("browser footer len = %d, want %d", len(got), len(browserKeys))
	}
	var f7Hint string
	for _, fk := range app.activeFooterKeys() {
		if fk.Key == tcell.KeyF7 {
			f7Hint = fk.Hint
			break
		}
	}
	if f7Hint == "" {
		t.Fatal("browser footer: F7 should have a hint (Mkdir)")
	}

	jobsKeys := menu.FunctionKeysJobsView()
	app.model.ViewMode = ui.ViewJobs
	gotJobs := app.activeFooterKeys()
	if len(gotJobs) != len(jobsKeys) {
		t.Fatalf("jobs footer len = %d, want %d", len(gotJobs), len(jobsKeys))
	}
	for i := range jobsKeys {
		if gotJobs[i].Key != jobsKeys[i].Key || gotJobs[i].KeyLabel != jobsKeys[i].KeyLabel || gotJobs[i].Hint != jobsKeys[i].Hint {
			t.Fatalf("jobs footer key %d = %+v, want %+v", i, gotJobs[i], jobsKeys[i])
		}
	}
	for _, fk := range gotJobs {
		if fk.Key == tcell.KeyF7 {
			t.Fatal("jobs footer should not list F7 (mkdir is for file panels)")
		}
	}
}

func TestJobsViewEscClosesViewDoesNotQuit(t *testing.T) {
	testJobsViewDismissKeyClosesViewDoesNotQuit(t, tcell.KeyEsc)
}

func TestJobsViewLeftClosesViewDoesNotQuit(t *testing.T) {
	testJobsViewDismissKeyClosesViewDoesNotQuit(t, tcell.KeyLeft)
}

func testJobsViewDismissKeyClosesViewDoesNotQuit(t *testing.T, key tcell.Key) {
	t.Helper()
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openJobsView()
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", app.model.ViewMode)
	}

	quit, _ := app.handleKey(tcell.NewEventKey(key, 0, tcell.ModNone))
	if quit {
		t.Fatalf("%v in jobs view must not quit the application", key)
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser after %v", app.model.ViewMode, key)
	}
}

func TestMessagesViewLeftClosesView(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openMessagesView()
	if app.model.ViewMode != ui.ViewMessages {
		t.Fatalf("ViewMode = %v, want ViewMessages", app.model.ViewMode)
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser after Left", app.model.ViewMode)
	}
}

func TestCommandsViewLeftClosesView(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openCommandsView()
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser after Left", app.model.ViewMode)
	}
}

func TestMenuShortcutCopyOpensTransferDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	// Open pulldown first, then press shortcut.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindCopy {
		t.Fatal("File menu Copy shortcut should open transfer dialog")
	}
	if len(app.jobState.AllJobs()) != 0 {
		t.Fatalf("expected no job before confirm, got %d", len(app.jobState.AllJobs()))
	}
}

func TestQuitImmediateSkipsConfirmation(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	root := t.TempDir()
	job := &jobs.Job{ID: "j", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: pathloc.PathsForTest(root)}
	app.jobState.Queue().Enqueue(job)

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if quit {
		t.Fatal("F10 with active job should not quit immediately")
	}
	if !app.model.QuitConfirm.Open {
		t.Fatal("expected quit confirmation dialog")
	}

	quit2, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModShift))
	if !quit2 {
		t.Fatal("Shift+F10 should quit immediately")
	}
	if app.model.QuitConfirm.Open {
		t.Fatal("quit confirm should be cleared on immediate quit")
	}
}

func TestQuitConfirmDialogAltShortcuts(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	root := t.TempDir()
	job := &jobs.Job{ID: "j", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: pathloc.PathsForTest(root)}
	app.jobState.Queue().Enqueue(job)

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !app.model.QuitConfirm.Open {
		t.Fatal("expected quit confirmation dialog")
	}

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if quit {
		t.Fatal("Alt+S should stay in the app")
	}
	if app.model.QuitConfirm.Open {
		t.Fatal("Alt+S should close quit confirmation")
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !app.model.QuitConfirm.Open {
		t.Fatal("expected quit confirmation dialog again")
	}

	quit, _ = app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'Q', tcell.ModAlt))
	if !quit {
		t.Fatal("Alt+Q should quit")
	}
	if app.model.QuitConfirm.Open {
		t.Fatal("Alt+Q should close quit confirmation")
	}
}
