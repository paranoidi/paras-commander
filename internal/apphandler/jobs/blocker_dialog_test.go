package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// newBlockerTestHandler wires a Handler over a live jobs.State whose worker is running, so
// blocker jobs actually enter state.waitingBlocker (ApplyEvent alone does not — only a running
// worker does).
func newBlockerTestHandler(t *testing.T, cfg config.Config) (*Handler, *jobs.State) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)

	state := jobs.NewState()
	state.SetTransferFunc(func(_ context.Context, job *jobs.Job, _ func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
		_ = waitBlocker(jobs.BlockerRequest{
			Kind: jobs.BlockerKindConflict,
			Conflict: &jobs.ConflictRequest{
				Source:      job.Sources[0].String(),
				Destination: job.Destination.String() + "/x",
			},
		})
		return nil
	})
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	state.StartWorker(stop)

	h := New(Deps{
		Host:   jobsHostStub{},
		Screen: screen,
		Model:  &ui.Model{},
		State:  state,
		Config: cfg,
	})
	return h, state
}

func addBlockerTestJob(state *jobs.State, id string) {
	state.AddJob(&jobs.Job{
		ID:          id,
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/src/" + id),
		Destination: pathloc.MustParse("/dst"),
	})
}

func waitJobsWaitingDecision(t *testing.T, h *Handler, state *jobs.State, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h.PollEvents()
		if state.JobsWaitingDecision() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for JobsWaitingDecision>=%d (got %d)", want, state.JobsWaitingDecision())
}

func TestHandleAnswerBlockerKeyOpensDialog(t *testing.T) {
	t.Parallel()
	h, state := newBlockerTestHandler(t, config.Default())
	addBlockerTestJob(state, "block-a")
	waitJobsWaitingDecision(t, h, state, 1)

	if !h.HandleAnswerBlockerKey() {
		t.Fatal("HandleAnswerBlockerKey() = false, want true")
	}
	if !h.model.ConflictDialog.Open {
		t.Fatal("ConflictDialog.Open = false, want true")
	}
	if h.model.ConflictDialog.JobID != "block-a" {
		t.Fatalf("JobID = %q, want block-a", h.model.ConflictDialog.JobID)
	}
}

func TestBlockerDialogPostponeDoesNotSubmit(t *testing.T) {
	t.Parallel()
	h, state := newBlockerTestHandler(t, config.Default())
	addBlockerTestJob(state, "block-postpone")
	waitJobsWaitingDecision(t, h, state, 1)
	h.HandleAnswerBlockerKey()

	h.HandleBlockerDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if h.model.ConflictDialog.Open {
		t.Fatal("dialog still open after Esc postpone")
	}
	if state.JobsWaitingDecision() != 1 {
		t.Fatalf("JobsWaitingDecision = %d, want 1 (still waiting)", state.JobsWaitingDecision())
	}
}

func TestBlockerDialogSubmitDecision(t *testing.T) {
	t.Parallel()
	h, state := newBlockerTestHandler(t, config.Default())
	addBlockerTestJob(state, "block-submit")
	waitJobsWaitingDecision(t, h, state, 1)
	h.HandleAnswerBlockerKey()

	h.model.ConflictDialog.Focus = 1 // Skip
	h.HandleBlockerDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if h.model.ConflictDialog.Open {
		t.Fatal("dialog still open after confirm")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		idle := true
		for _, j := range state.AllJobs() {
			if j != nil && !j.Status.IsFinished() {
				idle = false
				break
			}
		}
		if idle {
			return
		}
		h.PollEvents()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timeout waiting for job to finish")
}

func TestBlockerDialogChainOpensNext(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Jobs.BlockerDialogNextDebounceMS = 5000 // keep the real timer from firing during the test
	h, state := newBlockerTestHandler(t, cfg)

	addBlockerTestJob(state, "chain-1")
	waitJobsWaitingDecision(t, h, state, 1)
	addBlockerTestJob(state, "chain-2")
	waitJobsWaitingDecision(t, h, state, 2)

	h.HandleAnswerBlockerKey()
	if h.model.ConflictDialog.JobID != "chain-1" {
		t.Fatalf("first dialog JobID = %q, want chain-1", h.model.ConflictDialog.JobID)
	}
	h.model.ConflictDialog.Focus = 1 // Skip
	h.HandleBlockerDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	deadline := time.Now().Add(2 * time.Second)
	for state.JobsWaitingDecision() > 1 && time.Now().Before(deadline) {
		h.PollEvents()
		time.Sleep(5 * time.Millisecond)
	}
	if state.JobsWaitingDecision() != 1 {
		t.Fatalf("JobsWaitingDecision = %d, want 1 after first answer", state.JobsWaitingDecision())
	}

	gen := h.jobBlockerNextGen.Load()
	if !h.ApplyBlockerNextPayload(JobBlockerNextPayload{gen: gen}) {
		t.Fatal("ApplyBlockerNextPayload returned false")
	}
	if !h.model.ConflictDialog.Open {
		t.Fatal("second dialog not open after chain")
	}
	if h.model.ConflictDialog.JobID != "chain-2" {
		t.Fatalf("second dialog JobID = %q, want chain-2", h.model.ConflictDialog.JobID)
	}
}

func TestJobBlockerNextPayloadStaleGenIgnored(t *testing.T) {
	t.Parallel()
	h := New(Deps{Host: jobsHostStub{}, Model: &ui.Model{}, State: jobs.NewState(), Config: config.Default()})
	h.jobBlockerNextGen.Add(1)
	if h.ApplyBlockerNextPayload(JobBlockerNextPayload{gen: 0}) {
		t.Fatal("stale gen should not open dialog")
	}
}
