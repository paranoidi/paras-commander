package app

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func installJobBlockerTransferFunc(app *App) {
	app.jobState.SetTransferFunc(func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
		_ = waitBlocker(jobs.BlockerRequest{
			Kind: jobs.BlockerKindConflict,
			Conflict: &jobs.ConflictRequest{
				Source:      job.Sources[0].String(),
				Destination: job.Destination.String() + "/x",
			},
		})
		return nil
	})
}

func waitJobsWaitingDecision(t *testing.T, app *App, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		app.jobsCtrl.PollEvents()
		if app.jobState.JobsWaitingDecision() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for JobsWaitingDecision>=%d (got %d)", want, app.jobState.JobsWaitingDecision())
}

// TestHandleKeyCtrlQOpensJobBlockerDialog exercises the quick-blocker dialog through the real
// App key-dispatch path (keymap.Global lookup -> handleKey), which needs a full App rather than
// the jobs handler in isolation. Handler-level blocker dialog behavior (open/postpone/confirm/
// chain) is covered in internal/apphandler/jobs/blocker_dialog_test.go.
func TestHandleKeyCtrlQOpensJobBlockerDialog(t *testing.T) {
	app := testAppMinimal(t)
	installJobBlockerTransferFunc(app)

	app.jobState.AddJob(&jobs.Job{
		ID:          "ctrlq",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/src/q"),
		Destination: pathloc.MustParse("/dst"),
	})
	waitJobsWaitingDecision(t, app, 1)

	for _, tc := range []struct {
		name string
		ev   *tcell.EventKey
	}{
		{"KeyCtrlQ", tcell.NewEventKey(tcell.KeyCtrlQ, 0, tcell.ModNone)},
		{"legacy key 17", tcell.NewEventKey(tcell.Key(17), 0, tcell.ModNone)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app.model.ConflictDialog = dialog.ConflictDialogState{}
			if id, ok := app.keys.Global.Lookup(tc.ev); !ok || id != keymap.ActionJobsAnswerBlocker {
				t.Fatalf("Ctrl+Q lookup = %q %v, want jobs.answer-blocker", id, ok)
			}
			quit, rendered := app.handleKey(tc.ev)
			if quit {
				t.Fatal("handleKey quit")
			}
			if !rendered {
				t.Fatal("handleKey should render")
			}
			if !app.model.ConflictDialog.Open {
				t.Fatal("ConflictDialog not open after Ctrl+Q")
			}
		})
	}
}
