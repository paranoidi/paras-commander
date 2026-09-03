package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestScanFlipsToQueuedAfterFirstItemAndGrowsTotals drives a hand-controlled PlanProducer (no
// real filesystem walk) through State to verify two pipelining-specific behaviors: the job
// leaves StatusScanning as soon as the producer's first item is known, not once the whole walk
// finishes; and EventScanTotals is emitted repeatedly with growing counts while the producer
// keeps running, not just once at the end.
func TestScanFlipsToQueuedAfterFirstItemAndGrowsTotals(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: 10 * time.Millisecond})

	firstItem := make(chan struct{})
	doneCh := make(chan struct{})
	var filesN atomic.Int64

	s.SetScanFunc(func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) PlanProducer {
		return PlanProducer{
			Items:     make(chan ops.PlanItem), // never read from in this test; harmless if unused
			FirstItem: firstItem,
			Totals: func() (int, int, int64) {
				n := int(filesN.Load())
				return n, 0, int64(n) * 100
			},
			Done: doneCh,
			Err:  func() error { return nil },
		}
	})

	job := &Job{ID: "streamed-job", Type: TypeCopy, Status: StatusScanning, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	// Drain the EventEnqueued event so it doesn't get mistaken for anything else below.
	deadline := time.After(3 * time.Second)
	select {
	case ev := <-s.Events():
		if ev.Type != EventEnqueued {
			t.Fatalf("first event = %v, want EventEnqueued", ev.Type)
		}
	case <-deadline:
		t.Fatal("timeout waiting EventEnqueued")
	}

	// Job should still be Scanning: the producer hasn't reported a first item yet.
	if all := s.AllJobs(); len(all) != 1 || all[0].Status != StatusScanning {
		t.Fatalf("job status before first item = %+v, want StatusScanning", all)
	}

	filesN.Store(1)
	close(firstItem)

	// Status must flip to Queued promptly (before the walk finishes), not wait for Done.
	deadline = time.After(3 * time.Second)
	for {
		all := s.AllJobs()
		if len(all) == 1 && all[0].Status == StatusQueued {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting StatusQueued after first item; last seen: %+v", all)
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Collect EventScanTotals events, bumping the producer's running totals after the first one
	// arrives, and confirm a later event reflects the growth — i.e. totals are pushed
	// periodically while the producer is still running, not just once at completion.
	var firstTotal = -1
	deadline = time.After(3 * time.Second)
	for {
		select {
		case ev := <-s.Events():
			if ev.Type != EventScanTotals {
				continue
			}
			if firstTotal == -1 {
				firstTotal = ev.TotalFiles
				filesN.Store(9)
				continue
			}
			if ev.TotalFiles > firstTotal {
				goto grown
			}
		case <-deadline:
			t.Fatalf("timeout waiting for growing EventScanTotals; first=%d", firstTotal)
		}
	}
grown:

	// Finish the walk: Done closes, Err returns nil, job.PlanComplete must end up true.
	filesN.Store(20)
	close(doneCh)

	deadline = time.After(3 * time.Second)
	for {
		all := s.AllJobs()
		if len(all) == 1 && all[0].PlanComplete {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting PlanComplete=true; last seen: %+v", all)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestScanFailsJobWhenErrorArrivesBeforeFirstItem is the regression-protection case: a producer
// that never reports a first item (the job never leaves Scanning) and then errors must still
// fail the job through runJobScan's own finishScanFailed path, since no transfer worker can ever
// have taken ownership of it.
func TestScanFailsJobWhenErrorArrivesBeforeFirstItem(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: 10 * time.Millisecond})

	doneCh := make(chan struct{})
	wantErr := errors.New("boulder blocks passage")

	s.SetScanFunc(func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) PlanProducer {
		return PlanProducer{
			Items:     make(chan ops.PlanItem),
			FirstItem: make(chan struct{}), // never closes: no item ever seen
			Totals:    func() (int, int, int64) { return 0, 0, 0 },
			Done:      doneCh,
			Err:       func() error { return wantErr },
		}
	})

	job := &Job{ID: "never-flipped-job", Type: TypeCopy, Status: StatusScanning, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	deadline := time.After(3 * time.Second)
	select {
	case ev := <-s.Events():
		if ev.Type != EventEnqueued {
			t.Fatalf("first event = %v, want EventEnqueued", ev.Type)
		}
	case <-deadline:
		t.Fatal("timeout waiting EventEnqueued")
	}

	close(doneCh)

	deadline = time.After(3 * time.Second)
	for {
		all := s.AllJobs()
		if len(all) == 1 && all[0].Status == StatusFailed {
			if all[0].Error != wantErr.Error() {
				t.Fatalf("job.Error = %q, want %q", all[0].Error, wantErr.Error())
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting StatusFailed; last seen: %+v", all)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestScanDoesNotOverwriteStatusAfterFlipOnLateError proves the correctness fix: once a job has
// already flipped to Queued/Paused (the transfer worker may already be running it, or may have
// already finished it), a walk error that arrives afterward must NOT be applied to job.Status by
// runJobScan — that would race the executor's own event-driven completion path, which owns the
// outcome once transferring has started. runJobScan should only mark PlanComplete and leave
// Status untouched in that case.
func TestScanDoesNotOverwriteStatusAfterFlipOnLateError(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: 10 * time.Millisecond})

	firstItem := make(chan struct{})
	doneCh := make(chan struct{})
	wantErr := errors.New("boulder blocks passage")

	s.SetScanFunc(func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) PlanProducer {
		return PlanProducer{
			Items:     make(chan ops.PlanItem),
			FirstItem: firstItem,
			Totals:    func() (int, int, int64) { return 1, 0, 100 },
			Done:      doneCh,
			Err:       func() error { return wantErr },
		}
	})

	job := &Job{ID: "flipped-then-errored-job", Type: TypeCopy, Status: StatusScanning, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	deadline := time.After(3 * time.Second)
	select {
	case ev := <-s.Events():
		if ev.Type != EventEnqueued {
			t.Fatalf("first event = %v, want EventEnqueued", ev.Type)
		}
	case <-deadline:
		t.Fatal("timeout waiting EventEnqueued")
	}

	close(firstItem)

	deadline = time.After(3 * time.Second)
	for {
		all := s.AllJobs()
		if len(all) == 1 && all[0].Status == StatusQueued {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting StatusQueued after first item; last seen: %+v", all)
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Simulate the transfer worker having picked the job up and started running it, exactly as
	// would happen for real once flipToRunnable moved it to StatusQueued.
	s.mu.Lock()
	job.Status = StatusRunning
	s.mu.Unlock()

	// Now the walk fails. runJobScan must not stomp job.Status with StatusFailed/StatusCanceled;
	// the executor's own error handling (via job.PlanErr) owns the outcome from here.
	close(doneCh)

	deadline = time.After(3 * time.Second)
	for {
		all := s.AllJobs()
		if len(all) == 1 && all[0].PlanComplete {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting PlanComplete=true; last seen: %+v", all)
		case <-time.After(5 * time.Millisecond):
		}
	}

	all := s.AllJobs()
	if len(all) != 1 || all[0].Status != StatusRunning {
		t.Fatalf("job status after late walk error = %+v, want StatusRunning (untouched)", all)
	}
}
