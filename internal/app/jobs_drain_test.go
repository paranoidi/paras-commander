package app

import (
	"testing"

	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestDrainDiscardProgressEventsCapsDiscards(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.model.ViewMode = ui.ViewBrowser
	ch := app.jobState.Events()
	for i := 0; i < jobsctrl.MaxProgressEventsDiscardPerDrain+20; i++ {
		app.jobState.QueueTestEvent(jobs.Event{Type: jobs.EventProgress, JobID: "j1"})
	}
	app.drainDiscardProgressEvents()
	remaining := 0
	for {
		select {
		case <-ch:
			remaining++
		default:
			goto done
		}
	}
done:
	if remaining < 1 {
		t.Fatalf("expected progress events left in channel, got %d", remaining)
	}
}

func TestDrainDiscardProgressEventsAppliesNonProgress(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.model.ViewMode = ui.ViewBrowser
	root := t.TempDir()
	job := &jobs.Job{ID: "j1", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: []string{root}}
	app.jobState.Queue().Enqueue(job)
	app.jobState.QueueTestEvent(jobs.Event{Type: jobs.EventProgress, JobID: "j1"})
	app.jobState.QueueTestEvent(jobs.Event{
		Type:   jobs.EventJobBlockerRequest,
		JobID:  "j1",
		Status: jobs.StatusWaitingDecision,
	})
	app.drainDiscardProgressEvents()
	var found *jobs.Job
	for _, j := range app.jobState.AllJobs() {
		if j.ID == "j1" {
			found = j
			break
		}
	}
	if found == nil || found.Status != jobs.StatusWaitingDecision {
		t.Fatalf("job status = %v, want %s", found, jobs.StatusWaitingDecision)
	}
}
