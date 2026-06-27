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
		app.pollJobEvents()
		if app.jobState.JobsWaitingDecision() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for JobsWaitingDecision>=%d (got %d)", want, app.jobState.JobsWaitingDecision())
}

func TestTryOpenJobBlockerDialog(t *testing.T) {
	app := testAppMinimal(t)
	installJobBlockerTransferFunc(app)

	app.jobState.AddJob(&jobs.Job{
		ID:          "block-a",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/src/a"),
		Destination: pathloc.MustParse("/dst"),
	})
	waitJobsWaitingDecision(t, app, 1)

	app.tryOpenJobBlockerDialog()
	if !app.model.ConflictDialog.Open {
		t.Fatal("ConflictDialog.Open = false, want true")
	}
	if app.model.ConflictDialog.JobID != "block-a" {
		t.Fatalf("JobID = %q, want block-a", app.model.ConflictDialog.JobID)
	}
}

func TestJobBlockerDialogPostponeDoesNotSubmit(t *testing.T) {
	app := testAppMinimal(t)
	installJobBlockerTransferFunc(app)

	app.jobState.AddJob(&jobs.Job{
		ID:          "block-postpone",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/src/p"),
		Destination: pathloc.MustParse("/dst"),
	})
	waitJobsWaitingDecision(t, app, 1)
	app.tryOpenJobBlockerDialog()

	app.handleConflictDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ConflictDialog.Open {
		t.Fatal("dialog still open after Esc postpone")
	}
	if app.jobState.JobsWaitingDecision() != 1 {
		t.Fatalf("JobsWaitingDecision = %d, want 1 (still waiting)", app.jobState.JobsWaitingDecision())
	}
}

func TestJobBlockerDialogSubmitDecision(t *testing.T) {
	app := testAppMinimal(t)
	installJobBlockerTransferFunc(app)

	app.jobState.AddJob(&jobs.Job{
		ID:          "block-submit",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/src/s"),
		Destination: pathloc.MustParse("/dst"),
	})
	waitJobsWaitingDecision(t, app, 1)
	app.tryOpenJobBlockerDialog()

	app.model.ConflictDialog.Focus = 1 // Skip
	app.confirmJobBlockerDialog()
	if app.model.ConflictDialog.Open {
		t.Fatal("dialog still open after confirm")
	}
	waitUntilAppJobsFinished(t, app, 3*time.Second)
}

func TestJobBlockerDialogChainOpensNext(t *testing.T) {
	app := testAppMinimal(t)
	installJobBlockerTransferFunc(app)

	for _, id := range []string{"chain-1", "chain-2"} {
		app.jobState.AddJob(&jobs.Job{
			ID:          id,
			Type:        jobs.TypeCopy,
			Status:      jobs.StatusQueued,
			Sources:     pathloc.PathsForTest("/src/" + id),
			Destination: pathloc.MustParse("/dst"),
		})
		if id == "chain-1" {
			waitJobsWaitingDecision(t, app, 1)
		}
	}
	waitJobsWaitingDecision(t, app, 2)

	app.tryOpenJobBlockerDialog()
	if app.model.ConflictDialog.JobID != "chain-1" {
		t.Fatalf("first dialog JobID = %q, want chain-1", app.model.ConflictDialog.JobID)
	}
	app.model.ConflictDialog.Focus = 1
	app.config.Jobs.BlockerDialogNextDebounceMS = 5000
	app.confirmJobBlockerDialog()
	deadline := time.Now().Add(2 * time.Second)
	for app.jobState.JobsWaitingDecision() > 1 && time.Now().Before(deadline) {
		app.pollJobEvents()
		time.Sleep(5 * time.Millisecond)
	}
	if app.jobState.JobsWaitingDecision() != 1 {
		t.Fatalf("JobsWaitingDecision = %d, want 1 after first answer", app.jobState.JobsWaitingDecision())
	}
	gen := app.jobBlockerNextGen.Load()
	if !app.applyJobBlockerNextPayload(jobBlockerNextPayload{gen: gen}) {
		t.Fatal("applyJobBlockerNextPayload returned false")
	}
	if !app.model.ConflictDialog.Open {
		t.Fatal("second dialog not open after chain")
	}
	if app.model.ConflictDialog.JobID != "chain-2" {
		t.Fatalf("second dialog JobID = %q, want chain-2", app.model.ConflictDialog.JobID)
	}
}

func TestJobBlockerNextPayloadStaleGenIgnored(t *testing.T) {
	app := testAppMinimal(t)
	app.jobBlockerNextGen.Add(1)
	if app.applyJobBlockerNextPayload(jobBlockerNextPayload{gen: 0}) {
		t.Fatal("stale gen should not open dialog")
	}
}

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
			if id, ok := app.keys.Lookup(tc.ev); !ok || id != keymap.ActionJobsAnswerBlocker {
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
