package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// find.view hides Find behind the viewer; closing the viewer brings it back as it was.
func TestFindViewResumesFindAfterViewerCloses(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"harbor.txt", "lantern.txt", "meadow.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)
	app := newTestApp(t, screen, testOptions(root))
	app.findCtrl.OpenDialog(ui.PrimaryPanel)
	waitFindIndexDone(t, app)

	st := &app.model.FindDialog
	app.findCtrl.HandleDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)) // mark all
	marked := len(st.MarkedPaths)
	if marked == 0 {
		t.Fatal("test setup: nothing marked")
	}
	st.Selected = 1
	wantSelected, wantRanked := st.Selected, len(st.Ranked)

	if !app.findCtrl.RunAction(keymap.ActionFindView) {
		t.Fatal("find.view not handled")
	}
	if st.Open {
		t.Fatal("Find should be hidden while the viewer is open")
	}
	if !app.model.FullscreenFilePreview.Open {
		t.Fatal("viewer did not open")
	}
	if app.inputMode() == InputModeFindDialog {
		t.Fatal("keys still go to the hidden Find")
	}

	app.previewCtrl.CloseFilePreviewFullscreen()

	if !st.Open {
		t.Fatal("Find did not reopen after the viewer closed")
	}
	if st.Selected != wantSelected || len(st.Ranked) != wantRanked || len(st.MarkedPaths) != marked {
		t.Fatalf("restored selected=%d ranked=%d marked=%d, want %d %d %d",
			st.Selected, len(st.Ranked), len(st.MarkedPaths), wantSelected, wantRanked, marked)
	}
	if app.findCtrl.Hidden() {
		t.Fatal("still flagged hidden after reopening")
	}

	// A later viewer not opened from Find leaves a closed Find closed.
	app.findCtrl.CloseDialog()
	if err := app.previewCtrl.OpenFullscreenFilePreviewAt(filepath.Join(root, "harbor.txt"), false); err != nil {
		t.Fatal(err)
	}
	app.previewCtrl.CloseFilePreviewFullscreen()
	if st.Open {
		t.Fatal("closing an unrelated viewer reopened Find")
	}
}

// The find key parks Find (state kept, panel shows the parked icon); pressing it again on the
// same panel brings it back as it was.
func TestFindKeyParksAndRestoresFind(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"harbor.txt", "lantern.txt", "meadow.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)
	app := newTestApp(t, screen, testOptions(root))
	ctrlF := tcell.NewEventKey(tcell.KeyCtrlF, 'f', tcell.ModCtrl)
	app.handleKey(ctrlF)
	if !app.model.FindDialog.Open {
		t.Fatal("C-f did not open Find")
	}
	waitFindIndexDone(t, app)
	st := &app.model.FindDialog
	app.findCtrl.HandleDialogKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)) // mark all
	marked := len(st.MarkedPaths)

	app.handleKey(ctrlF)
	if st.Open || !app.model.FindParkedOn(ui.PrimaryPanel) || app.inputMode() == InputModeFindDialog {
		t.Fatalf("C-f did not park Find: open=%v", st.Open)
	}
	if !screenHasText(screen, app.styles.IconSearchIcon()) {
		t.Fatal("parked find icon not painted")
	}
	app.handleKey(ctrlF)
	if !st.Open || st.Parked || len(st.MarkedPaths) != marked || marked == 0 {
		t.Fatalf("restore: open=%v parked=%v marked=%d want %d", st.Open, st.Parked, len(st.MarkedPaths), marked)
	}
}
