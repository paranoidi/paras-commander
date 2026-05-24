package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestFlattenMixedSelectionShowsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "alpha.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(dir, "bravo")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{
		filepath.Clean(filePath): true,
		filepath.Clean(subDir):   true,
	}

	app.openFlattenDialog()
	if app.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should not open for mixed selection")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyError {
		t.Fatalf("urgency = %v, want error", app.model.MessageUrgency)
	}
}

func TestFileMenuFlattenOpensDialog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "charlie")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "delta")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.activePanel().Load(dir); err != nil {
		t.Fatal(err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatal(err)
	}
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{filepath.Clean(sub): true}

	activateFileMenuItem(t, app, 'F')
	if !app.model.FlattenDialog.Open {
		t.Fatal("flatten dialog should be open")
	}
	if app.model.FlattenDialog.Destination != filepath.Clean(dst) {
		t.Fatalf("destination = %q, want %q", app.model.FlattenDialog.Destination, dst)
	}
	if app.model.FlattenDialog.Recursive {
		t.Fatal("recursive should default false")
	}
	if !app.model.FlattenDialog.RemoveEmpty {
		t.Fatal("remove empty should default true")
	}
}

func TestFlattenConfirmQueuesJob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "echo")
	dst := filepath.Join(dir, "foxtrot")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "golf.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.stopWorker()
	if err := app.activePanel().Load(dir); err != nil {
		t.Fatal(err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatal(err)
	}
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{filepath.Clean(root): true}

	app.openFlattenDialog()
	app.handleFlattenDialogKey(tcell.NewEventKey(tcell.KeyRune, 'O', tcell.ModAlt))

	if app.model.FlattenDialog.Open {
		t.Fatal("dialog should close after OK")
	}
	all := app.jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeFlatten {
		t.Fatalf("job type = %v, want flatten", all[0].Type)
	}
	if len(all[0].Sources) != 1 {
		t.Fatalf("sources = %d, want 1 child", len(all[0].Sources))
	}
}
