package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
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
	defer app.stopWorker()

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(dir, name)
		app.addTransferJob(jobs.TypeCopy, []string{p}, dst, true)
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

func TestEnqueueCopyJobFileMenuStyleQueuesOneJobPerSelectionRound(t *testing.T) {
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
	app.stopWorker() // keep jobs queued; otherwise copies run and tmpdir cleanup races

	if err := app.activePanel().Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dst: %v", err)
	}
	app.model.ActivePanel = ui.LeftPanel

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(dir, name)
		app.activePanel().SelectedPaths = map[string]bool{p: true}
		app.enqueueCopyJob()
	}

	if got := app.jobState.Queue().Len(); got != 3 {
		t.Fatalf("Queue().Len() = %d, want 3 (enqueueCopyJob once per file)", got)
	}
	app.syncJobsList()
	if got := len(app.model.JobsList); got != 3 {
		t.Fatalf("JobsList len = %d, want 3", got)
	}
}
