package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func openJobsViewWithQueuedJobs(t *testing.T, app *App, n int) {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		name := filepath.Join(dir, "src"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		app.dialogCtrl.AddTransferJob(jobs.TypeCopy, []string{name}, dst, true, app.dialogCtrl.TransferPreserveFromConfig())
	}
	app.jobsCtrl.OpenJobsView()
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", app.model.ViewMode)
	}
}

// TestJobsViewViMotionHJKLOnlyWhenModeOn verifies 'j'/'k' move the jobs-list selection like
// Down/Up only while vi-motion mode is on, mirroring the browser's own hjkl gating.
func TestJobsViewViMotionHJKLOnlyWhenModeOn(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	openJobsViewWithQueuedJobs(t, app, 2)
	app.model.JobsView.FocusPane = 0

	before := app.model.JobsView.Selected
	app.jobsCtrl.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if app.model.JobsView.Selected != before {
		t.Fatalf("vi-motion off: 'j' moved selection from %d to %d", before, app.model.JobsView.Selected)
	}

	app.model.ViMotionMode = true
	app.jobsCtrl.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))
	if app.model.JobsView.Selected == before {
		t.Fatal("vi-motion on: 'j' should move the selection")
	}
	moved := app.model.JobsView.Selected
	app.jobsCtrl.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	if app.model.JobsView.Selected != before {
		t.Fatalf("vi-motion on: 'k' selection = %d, want %d (back from %d)", app.model.JobsView.Selected, before, moved)
	}
}

// TestJobsViewViMotionLeaderLetterOnlyWhenModeOn verifies a bound leader-menu letter (here 'x',
// jobs.close) fires its action directly only while vi-motion mode is on.
func TestJobsViewViMotionLeaderLetterOnlyWhenModeOn(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	openJobsViewWithQueuedJobs(t, app, 1)

	app.jobsCtrl.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatal("vi-motion off: 'x' must not close the jobs view")
	}

	app.model.ViMotionMode = true
	app.jobsCtrl.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("vi-motion on: 'x' should dispatch jobs.close directly, ViewMode = %v", app.model.ViewMode)
	}
}
