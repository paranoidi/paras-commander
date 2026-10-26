package jobs

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestScanKeepsEstimateUntilTotalsDone checks that a copy's enqueue-time EstimatedBytes stays the
// byte total while the counting walk's tally is below it, and that the exact count replaces it
// (even when smaller) once the counting walk finishes.
func TestScanKeepsEstimateUntilTotalsDone(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: 5 * time.Millisecond})

	totalsDone := make(chan struct{})
	done := make(chan struct{})
	var counted atomic.Int64
	s.SetScanFunc(func(context.Context, []pathloc.Path, pathloc.Path, ScanWalkHooks) PlanProducer {
		return PlanProducer{
			Items:      make(chan ops.PlanItem),
			FirstItem:  make(chan struct{}),
			Totals:     func() (int, int, int64) { return 1, 0, counted.Load() },
			TotalsDone: totalsDone,
			Done:       done,
			Err:        func() error { return nil },
		}
	})

	const estimate = 5000
	s.AddJob(&Job{
		ID: "estimate-job", Type: TypeCopy, Status: StatusScanning,
		Sources: pathloc.PathsForTest("/willow"), Destination: pathloc.MustParse("/harbor"),
		EstimatedBytes: estimate, TotalBytes: estimate,
	})
	go func() {
		for range s.Events() {
		}
	}()

	waitTotal := func(want int64, complete bool) {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			all := s.AllJobs()
			if len(all) == 1 && all[0].TotalBytes == want && all[0].TotalsComplete == complete {
				return
			}
			select {
			case <-deadline:
				t.Fatalf("timeout waiting TotalBytes=%d complete=%v; last: %+v", want, complete, all)
			case <-time.After(5 * time.Millisecond):
			}
		}
	}

	counted.Store(1200)
	time.Sleep(30 * time.Millisecond) // several ticks with a tally below the estimate
	waitTotal(estimate, false)

	counted.Store(7000)
	waitTotal(7000, false)

	counted.Store(4200)
	close(totalsDone)
	waitTotal(4200, true)
	close(done)
}
