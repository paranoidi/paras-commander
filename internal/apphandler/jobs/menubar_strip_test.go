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

	if !removing.Deleting || copying.Deleting {
		t.Fatalf("Deleting = %v while copying, %v while removing; want false, true", copying.Deleting, removing.Deleting)
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
