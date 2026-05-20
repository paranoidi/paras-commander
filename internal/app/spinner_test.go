package app

import (
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestArmSpinnerRedrawTimerRearmsAfterBusyTick(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	root := t.TempDir()
	job := &jobs.Job{ID: "spin", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: pathloc.PathsForTest(root)}
	app.jobState.Queue().Enqueue(job)
	if !app.menuBarSpinnerBusy() {
		t.Fatal("expected spinner busy with running job")
	}
	app.armSpinnerRedrawTimer()
	if app.spinnerRedrawTimer == nil {
		t.Fatal("expected spinner timer armed")
	}
	// Simulate timer callback: nil timer then post tick (we only need timer cleared).
	app.spinnerRedrawTimer.Stop()
	app.spinnerRedrawTimer = nil
	// End-of-loop behavior: busy always re-arms.
	app.armSpinnerRedrawTimer()
	if app.spinnerRedrawTimer == nil {
		t.Fatal("expected spinner timer re-armed when still busy")
	}
}

func TestArmSpinnerRedrawTimerStopsWhenIdle(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.armSpinnerRedrawTimer()
	if app.spinnerRedrawTimer != nil {
		t.Fatal("expected no timer when idle")
	}
}
