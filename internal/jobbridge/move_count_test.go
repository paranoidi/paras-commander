package jobbridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

func collectEvents() (func(jobs.Event), func() []jobs.Event) {
	var mu sync.Mutex
	var evs []jobs.Event
	return func(ev jobs.Event) {
			mu.Lock()
			evs = append(evs, ev)
			mu.Unlock()
		}, func() []jobs.Event {
			mu.Lock()
			defer mu.Unlock()
			return append([]jobs.Event(nil), evs...)
		}
}

// TestMoveCountAddsCopyPhaseToSettledUnits checks that the final totals are the rename phase's
// units plus the counted copy phase, marked complete, and that interim totals keep the estimate
// as a floor.
func TestMoveCountAddsCopyPhaseToSettledUnits(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	phase := ops.MoveCopyPhase{Items: 2, Bytes: 10, Count: func(_ context.Context, _ ops.PlanBuildOptions, sink func(ops.PlanItem)) error {
		sink(ops.PlanItem{IsDir: true})
		sink(ops.PlanItem{FileSize: 300})
		<-release
		sink(ops.PlanItem{FileSize: 40})
		return nil
	}}
	job := &jobs.Job{ID: "lantern", Type: jobs.TypeMove, EstimatedBytes: 5000}
	emit, events := collectEvents()
	mc := startMoveCount(context.Background(), job, phase, time.Millisecond, emit)
	deadline := time.Now().Add(5 * time.Second)
	for len(events()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if evs := events(); len(evs) == 0 || evs[0].TotalsComplete || evs[0].TotalBytes != 5000 {
		t.Fatalf("interim totals = %+v; want incomplete with the 5000-byte estimate floor", evs)
	}
	close(release)
	<-mc.done
	evs := events()
	last := evs[len(evs)-1]
	if last.Type != jobs.EventPlanTotals || !last.TotalsComplete || last.TotalFiles != 5 || last.TotalDirs != 1 || last.TotalBytes != 350 {
		t.Fatalf("final = %+v; want complete 5 items, 1 dir, 350 bytes", last)
	}
}

// TestMoveCountWithoutCopyPhaseIsCompleteAtOnce checks a rename-only move settles immediately.
func TestMoveCountWithoutCopyPhaseIsCompleteAtOnce(t *testing.T) {
	t.Parallel()
	emit, events := collectEvents()
	mc := startMoveCount(context.Background(), &jobs.Job{ID: "meadow", Type: jobs.TypeMove, EstimatedBytes: 900}, ops.MoveCopyPhase{Items: 3, Bytes: 900}, time.Millisecond, emit)
	<-mc.done
	evs := events()
	if len(evs) != 1 || !evs[0].TotalsComplete || evs[0].TotalFiles != 3 || evs[0].TotalBytes != 900 {
		t.Fatalf("events = %+v; want one complete 3 items / 900 bytes", evs)
	}
}
