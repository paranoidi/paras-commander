package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestMenuBarStripMoveRemovingSourcesKeepsLayout checks that a move removing its copied sources
// only switches the bar to the delete colours: the fraction, speed slot and status groups stay.
func TestMenuBarStripMoveRemovingSourcesKeepsLayout(t *testing.T) {
	t.Parallel()
	state := jobs.NewState()
	h := New(Deps{Host: jobsHostStub{}, Model: &ui.Model{}, State: state, Config: config.Default()})
	job := &jobs.Job{
		ID:          jobs.NewJobID(),
		Type:        jobs.TypeMove,
		Status:      jobs.StatusScanning,
		Sources:     []pathloc.Path{pathloc.MustParse("/orchard/lantern")},
		Destination: pathloc.MustParse("/harbor"),
	}
	state.AddJob(job)
	state.ApplyEvent(jobs.Event{Type: jobs.EventStarted, JobID: job.ID, Status: jobs.StatusRunning})
	state.ApplyEvent(jobs.Event{Type: jobs.EventPlanTotals, JobID: job.ID, Status: jobs.StatusRunning, TotalFiles: 10, TotalBytes: 1000, TotalsComplete: true})
	state.ApplyEvent(jobs.Event{Type: jobs.EventProgress, JobID: job.ID, Status: jobs.StatusRunning, DoneFiles: 4, DoneBytes: 400})

	copying := h.MenuBarStripSnapshot()
	state.ApplyEvent(jobs.Event{Type: jobs.EventRemovingSources, JobID: job.ID, Status: jobs.StatusRunning, Removing: true})
	removing := h.MenuBarStripSnapshot()

	if !removing.Removing || copying.Removing || removing.Deleting {
		t.Fatalf("Removing = %v while copying, %v while removing (Deleting %v); want false, true, false", copying.Removing, removing.Removing, removing.Deleting)
	}
	if removing.ProgressFrac != copying.ProgressFrac || removing.ProgressFrac != 0.4 {
		t.Fatalf("ProgressFrac = %v while removing, %v while copying; want 0.4 both", removing.ProgressFrac, copying.ProgressFrac)
	}
	if !removing.SpeedSlot || !copying.SpeedSlot {
		t.Fatal("speed slot must stay reserved while removing sources")
	}
	if len(removing.Groups) == 0 || len(removing.Groups) != len(copying.Groups) {
		t.Fatalf("groups = %v while removing, %v while copying", removing.Groups, copying.Groups)
	}
}

func TestReconcileSeenFinishedMarksOnLeavingJobsView(t *testing.T) {
	t.Parallel()
	newDone := func(state *jobs.State) {
		state.AddFinishedJob(&jobs.Job{
			ID:          jobs.NewJobID(),
			Type:        jobs.TypeCopy,
			Sources:     []pathloc.Path{pathloc.MustParse("/orchard/lantern")},
			Destination: pathloc.MustParse("/harbor"),
		})
	}
	completed := func(h *Handler) int {
		for _, g := range h.MenuBarStripSnapshot().Groups {
			if g.Status == string(jobs.StatusCompleted) {
				return g.Count
			}
		}
		return 0
	}
	setup := func() (*Handler, *jobs.State, *ui.Model) {
		state := jobs.NewState()
		m := &ui.Model{}
		h := New(Deps{Host: jobsHostStub{}, Model: m, State: state, Config: config.Default()})
		newDone(state)
		return h, state, m
	}

	t.Run("never visited", func(t *testing.T) {
		t.Parallel()
		h, _, m := setup()
		m.ViewMode = ui.ViewBrowser
		h.ReconcileSeenFinished()
		if n := completed(h); n != 1 {
			t.Fatalf("completed count = %d, want 1", n)
		}
	})

	t.Run("not marked while visible", func(t *testing.T) {
		t.Parallel()
		h, _, m := setup()
		m.ViewMode = ui.ViewJobs
		h.ReconcileSeenFinished()
		h.ReconcileSeenFinished()
		if n := completed(h); n != 1 {
			t.Fatalf("completed count = %d, want 1", n)
		}
	})

	for name, to := range map[string]ui.ViewMode{"to browser": ui.ViewBrowser, "to commands": ui.ViewCommands} {
		t.Run("marked on exit "+name, func(t *testing.T) {
			t.Parallel()
			h, state, m := setup()
			m.ViewMode = ui.ViewJobs
			h.ReconcileSeenFinished()
			m.ViewMode = to
			h.ReconcileSeenFinished()
			if n := completed(h); n != 0 {
				t.Fatalf("completed count after exit = %d, want 0", n)
			}
			newDone(state)
			h.ReconcileSeenFinished()
			if n := completed(h); n != 1 {
				t.Fatalf("completed count after new finish = %d, want 1", n)
			}
		})
	}
}
