package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestQueueRetainsMultiplePausedTransferJobs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(dir, name))
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(dir, name)
		app.addTransferJob(jobs.TypeCopy, []string{p}, dst, true, app.transferPreserveFromConfig())
	}
	if got := app.jobState.Queue().Len(); got != 3 {
		t.Fatalf("Queue().Len() = %d, want 3", got)
	}
	if got := len(app.jobState.AllJobs()); got != 3 {
		t.Fatalf("AllJobs() len = %d, want 3", got)
	}
	app.syncJobsList()
	if got := len(app.model.JobsList); got != 3 {
		t.Fatalf("JobsList len = %d, want 3", got)
	}
}

func TestFileMenuCopyOpensTransferDialogWithoutEnqueue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if err := app.activePanel().Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dst: %v", err)
	}

	p := filepath.Join(dir, "a.txt")
	app.activePanel().SelectedPaths = map[string]bool{p: true}

	defs := menu.BrowserDefinitions(app.keys, false)
	def, item, ok := findFileMenuItem(defs, "Copy")
	if !ok {
		t.Fatal("file menu should include Copy")
	}
	app.activateMenuSelection(def, item)

	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindCopy {
		t.Fatal("File menu Copy should open transfer dialog")
	}
	if len(app.jobState.AllJobs()) != 0 {
		t.Fatalf("expected no job before confirm, got %d", len(app.jobState.AllJobs()))
	}
	if len(app.activePanel().SelectedPaths) == 0 {
		t.Fatal("opening copy dialog must not clear selection")
	}
}

func TestQuickFilterF5OpensCopyDialogWithoutEnqueue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	if err := app.activePanel().Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dst: %v", err)
	}

	p := filepath.Join(dir, "a.txt")
	app.activePanel().SelectedPaths = map[string]bool{p: true}
	app.activePanel().OpenFilter(app.activeViewportRows())
	app.activePanel().AcceptFilter(app.activeViewportRows())

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if quit {
		t.Fatal("F5 from quick filter should not quit")
	}
	if app.inQuickFilterUI() {
		t.Fatal("F5 should dismiss quick filter")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != dialog.TransferKindCopy {
		t.Fatal("F5 from quick filter should open copy dialog")
	}
	if len(app.jobState.AllJobs()) != 0 {
		t.Fatalf("expected no job before confirm, got %d", len(app.jobState.AllJobs()))
	}
}

func findFileMenuItem(defs []menu.Definition, label string) (menu.Definition, menu.Item, bool) {
	for _, def := range defs {
		if def.ID != menu.TopFile {
			continue
		}
		for _, item := range def.Items {
			if item.Label == label {
				return def, item, true
			}
		}
	}
	return menu.Definition{}, menu.Item{}, false
}
