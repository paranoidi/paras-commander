package jobs

import (
	"context"
	"errors"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerSkipsPausedJobInFavorOfQueued(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	order := make(chan string, 4)
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		order <- job.ID
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "paused-front", Type: TypeCopy, Status: StatusPaused, Sources: pathloc.PathsForTest("/x"), Destination: pathloc.MustParse("/y")})
	s.AddJob(&Job{ID: "run-second", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/p"), Destination: pathloc.MustParse("/q")})

	select {
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting first runnable job")
	case id := <-order:
		if id != "run-second" {
			t.Fatalf("first run got %q, want run-second", id)
		}
	}
}

func TestDisjointDeleteJobRunsWhileTransferHoldsLease(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	started := make(chan string, 2)
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		started <- job.ID
		if job.Type == TypeCopy {
			<-release
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "copy-1", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow"), Destination: pathloc.MustParse("/maple")})

	select {
	case id := <-started:
		if id != "copy-1" {
			t.Fatalf("first started = %q, want copy-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for copy to start")
	}

	s.AddJob(&Job{ID: "del-1", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/birch")})

	select {
	case id := <-started:
		if id != "del-1" {
			t.Fatalf("second started = %q, want del-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for disjoint delete to run while copy is blocked")
	}

	// The delete's status flip to StatusCompleted happens on the worker goroutine after
	// TransferFunc returns, so poll briefly instead of asserting immediately.
	deadline := time.Now().Add(3 * time.Second)
	var deleteCompleted, sawOverlap, copyStillRunning bool
	for time.Now().Before(deadline) {
		all := s.AllJobs()
		var haveCopy, haveDelete bool
		for _, j := range all {
			switch j.ID {
			case "copy-1":
				haveCopy = true
				if !j.Status.IsFinished() {
					copyStillRunning = true
				}
			case "del-1":
				haveDelete = true
				if j.Status == StatusCompleted {
					deleteCompleted = true
				}
			}
		}
		if haveCopy && haveDelete {
			sawOverlap = true
		}
		if deleteCompleted {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)

	if !sawOverlap {
		t.Fatal("AllJobs() never listed both copy and delete jobs during the overlap")
	}
	if !deleteCompleted {
		t.Fatal("disjoint delete job never reached StatusCompleted while the copy job was still holding the transfer lease")
	}
	if !copyStillRunning {
		t.Fatal("copy job should still be running (blocked) while the disjoint delete job completed")
	}
}

func TestOverlappingDeleteWaitsForTransferLease(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	started := make(chan string, 2)
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		started <- job.ID
		if job.Type == TypeCopy {
			<-release
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "copy-1", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow/branch"), Destination: pathloc.MustParse("/maple/trunk")})

	select {
	case id := <-started:
		if id != "copy-1" {
			t.Fatalf("first started = %q, want copy-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for copy to start")
	}

	s.AddJob(&Job{ID: "del-1", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow/branch")})
	waitPendingDequeuedCount(t, s, 1)

	select {
	case id := <-started:
		t.Fatalf("overlapping delete started while transfer held the lease: %s", id)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case id := <-started:
		if id != "del-1" {
			t.Fatalf("after transfer released, started = %q, want del-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for overlapping delete after transfer released the lease")
	}
}

func TestOverlappingDeleteOfTransferDestWaitsForLease(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	started := make(chan string, 2)
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		started <- job.ID
		if job.Type == TypeCopy {
			<-release
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "copy-1", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/cedar"), Destination: pathloc.MustParse("/oak")})

	select {
	case id := <-started:
		if id != "copy-1" {
			t.Fatalf("first started = %q, want copy-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for copy to start")
	}

	s.AddJob(&Job{ID: "del-1", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/oak/leaf.txt")})
	waitPendingDequeuedCount(t, s, 1)

	select {
	case id := <-started:
		t.Fatalf("dest-overlapping delete started while transfer held the lease: %s", id)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case id := <-started:
		if id != "del-1" {
			t.Fatalf("after transfer released, started = %q, want del-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for dest-overlapping delete after transfer released the lease")
	}
}

func TestOverlappingDeleteAncestorWaitsForTransferLease(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	started := make(chan string, 2)
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		started <- job.ID
		if job.Type == TypeCopy {
			<-release
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "copy-1", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow/branch/leaf.txt"), Destination: pathloc.MustParse("/maple")})

	select {
	case id := <-started:
		if id != "copy-1" {
			t.Fatalf("first started = %q, want copy-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for copy to start")
	}

	s.AddJob(&Job{ID: "del-1", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow")})
	waitPendingDequeuedCount(t, s, 1)

	select {
	case id := <-started:
		t.Fatalf("ancestor-overlapping delete started while transfer held the lease: %s", id)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case id := <-started:
		if id != "del-1" {
			t.Fatalf("after transfer released, started = %q, want del-1", id)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for ancestor-overlapping delete after transfer released the lease")
	}
}

func TestOverlappingDeleteCanceledWhileWaitingForLease(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	holderEntered := make(chan struct{})
	holderLeft := make(chan struct{})
	var enteredIDs sync.Map

	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		enteredIDs.Store(job.ID, true)
		if job.ID == "copy-1" {
			close(holderEntered)
			<-release
			close(holderLeft)
			return nil
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "copy-1", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow"), Destination: pathloc.MustParse("/maple")})
	select {
	case <-holderEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for lease holder to enter TransferFunc")
	}

	del := &Job{ID: "del-1", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/willow/branch")}
	s.AddJob(del)
	waitPendingDequeuedCount(t, s, 1)

	if !s.CancelJob("del-1") {
		t.Fatal("CancelJob(del-1) = false, want true")
	}

	close(release)
	select {
	case <-holderLeft:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for lease holder to leave TransferFunc")
	}
	assertPendingNeverStarted(t, s, &enteredIDs, del)
}

func TestWorkerYieldsTransferLeaseWhileWaitingConflictDecision(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	order := make(chan string, 4)

	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		switch job.ID {
		case "job-a":
			order <- "a-start"
			_ = waitBlocker(BlockerRequest{
				Kind:     BlockerKindConflict,
				Conflict: &ConflictRequest{JobID: job.ID, Source: "/a", Destination: "/b", ExistingDetails: "file exists"},
			})
			order <- "a-after"
		case "job-b":
			order <- "b-start"
			order <- "b-end"
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.AddJob(&Job{ID: "job-a", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), Destination: pathloc.MustParse("/y")})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		s.AddJob(&Job{ID: "job-b", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/p"), Destination: pathloc.MustParse("/q")})
	}()

	deadline := time.After(5 * time.Second)
	want := []string{"a-start", "b-start", "b-end"}
	for _, w := range want {
		select {
		case <-deadline:
			t.Fatal("timeout waiting order", w)
		case got := <-order:
			if got != w {
				t.Fatalf("order got %q want %q", got, w)
			}
		}
	}
	s.SubmitConflictDecision("job-a", DecisionSkip)
	select {
	case <-deadline:
		t.Fatal("timeout waiting a-after")
	case got := <-order:
		if got != "a-after" {
			t.Fatalf("order got %q want a-after", got)
		}
	}
	wg.Wait()
}

func TestWorkerEmitsJobResumedAfterBlockerDecision(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	var wg sync.WaitGroup

	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		if job.ID != "job-a" {
			return nil
		}
		_ = waitBlocker(BlockerRequest{
			Kind:     BlockerKindConflict,
			Conflict: &ConflictRequest{JobID: job.ID, Source: "/a", Destination: "/b", ExistingDetails: "file exists"},
		})
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.AddJob(&Job{ID: "job-a", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), Destination: pathloc.MustParse("/y")})
	}()

	// SubmitConflictDecision only takes effect once the worker has registered
	// job-a's blocker channel, which happens before EventJobBlockerRequest is
	// emitted. Waiting on that event (rather than a signal fired from inside
	// the transferFunc closure, before registration) avoids racing ahead of
	// the registration and having the decision silently dropped.
	deadline := time.After(5 * time.Second)
	var gotBlockerRequest bool
	for !gotBlockerRequest {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for blocker request")
		case ev := <-s.Events():
			if ev.Type == EventJobBlockerRequest && ev.JobID == "job-a" {
				gotBlockerRequest = true
			}
		}
	}
	s.SubmitConflictDecision("job-a", DecisionOverwriteAll)

	var gotResumed bool
	for !gotResumed {
		select {
		case <-deadline:
			t.Fatal("timeout waiting EventJobResumed")
		case ev := <-s.Events():
			if ev.Type == EventJobResumed && ev.JobID == "job-a" && ev.Status == StatusRunning {
				gotResumed = true
			}
		}
	}
	wg.Wait()
}

func TestWorkerBlockerAllJobsListsSingleEntry(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	blockerEntered := make(chan struct{})
	var wg sync.WaitGroup

	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		if job.ID != "job-a" {
			return nil
		}
		close(blockerEntered)
		_ = waitBlocker(BlockerRequest{
			Kind:     BlockerKindConflict,
			Conflict: &ConflictRequest{JobID: job.ID, Source: "/a", Destination: "/b", ExistingDetails: "file exists"},
		})
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.AddJob(&Job{ID: "job-a", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), Destination: pathloc.MustParse("/y")})
	}()

	deadline := time.After(5 * time.Second)
	select {
	case <-deadline:
		t.Fatal("timeout waiting for blocker")
	case <-blockerEntered:
	}
	if all := s.AllJobs(); len(all) != 1 {
		t.Fatalf("AllJobs() len = %d, want 1 while blocked", len(all))
	} else if all[0].ID != "job-a" {
		t.Fatalf("job ID = %q, want job-a", all[0].ID)
	}
	s.SubmitConflictDecision("job-a", DecisionSkip)
	wg.Wait()
}

func TestStateEmitHook(t *testing.T) {
	var n atomic.Int32
	s := NewState()
	s.SetEmitHook(func(ev Event) {
		if ev.Type == EventEnqueued {
			n.Add(1)
		}
	})
	job := &Job{ID: NewJobID(), Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)
	if n.Load() != 1 {
		t.Fatalf("emit hook calls = %d, want 1", n.Load())
	}
}

func TestStateAddJob(t *testing.T) {
	s := NewState()
	job := &Job{ID: NewJobID(), Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	// Job should be in queue.
	snapshot := s.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	if snapshot[0].ID != job.ID {
		t.Fatalf("snapshot job ID = %s, want %s", snapshot[0].ID, job.ID)
	}
	if snapshot[0].Status != StatusQueued {
		t.Fatalf("status = %q, want %q", snapshot[0].Status, StatusQueued)
	}

	// Enqueue event should have been emitted.
	select {
	case ev := <-s.Events():
		if ev.Type != EventEnqueued {
			t.Fatalf("event type = %q, want %q", ev.Type, EventEnqueued)
		}
		if ev.JobID != job.ID {
			t.Fatalf("event job ID = %s, want %s", ev.JobID, job.ID)
		}
	default:
		t.Fatal("expected enqueue event, got none")
	}
}

func TestStateEventApplication(t *testing.T) {
	s := NewState()
	job := &Job{ID: "test-1", Type: TypeCopy, Status: StatusQueued}
	s.AddJob(job)

	// Apply started event.
	s.ApplyEvent(Event{Type: EventStarted, JobID: "test-1", Status: StatusRunning})
	active := s.ActiveJob()
	if active == nil {
		t.Fatal("expected active job, got nil")
	}
	if active.ID != "test-1" {
		t.Fatalf("active job ID = %s, want test-1", active.ID)
	}
	if active.Status != StatusRunning {
		t.Fatalf("active status = %q, want %q", active.Status, StatusRunning)
	}

	// Apply progress.
	s.ApplyEvent(Event{Type: EventProgress, JobID: "test-1", DoneFiles: 5, DoneBytes: 1024})
	active = s.ActiveJob()
	if active.DoneFiles != 5 {
		t.Fatalf("DoneFiles = %d, want 5", active.DoneFiles)
	}
	if active.DoneBytes != 1024 {
		t.Fatalf("DoneBytes = %d, want 1024", active.DoneBytes)
	}

	// Apply completed.
	s.ApplyEvent(Event{Type: EventCompleted, JobID: "test-1"})
	active = s.ActiveJob()
	if active != nil {
		t.Fatal("active job should be nil after completion")
	}

	// Job should be completed in queue.
	snapshot := s.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	if snapshot[0].Status != StatusCompleted {
		t.Fatalf("status = %q, want %q", snapshot[0].Status, StatusCompleted)
	}
}

func TestStateThroughputChartDisabledSkipsStrip(t *testing.T) {
	t.Parallel()
	s := NewState()
	s.SetThroughputChart(time.Second, 30*time.Second, false)
	job := &Job{ID: "chart-off", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)
	s.ApplyEvent(Event{Type: EventStarted, JobID: job.ID})
	s.ApplyEvent(Event{Type: EventProgress, JobID: job.ID, DoneBytes: 1_000_000, DoneFiles: 1})
	active := s.ActiveJob()
	if active == nil {
		t.Fatal("expected active job")
	}
	if len(active.ThroughputStrip) != 0 {
		t.Fatalf("ThroughputStrip = %v, want empty when chart disabled", active.ThroughputStrip)
	}
	if active.throughputStripOpenSet {
		t.Fatal("strip anchor should not be set when chart disabled")
	}
}

func TestStateEventFailed(t *testing.T) {
	s := NewState()
	job := &Job{ID: "test-2", Type: TypeCopy, Status: StatusQueued}
	s.AddJob(job)
	s.ApplyEvent(Event{Type: EventStarted, JobID: "test-2"})
	s.ApplyEvent(Event{Type: EventFailed, JobID: "test-2", Error: "permission denied"})

	if a := s.ActiveJob(); a != nil {
		t.Fatal("active job should be nil after failure")
	}
	snapshot := s.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	if snapshot[0].Status != StatusFailed {
		t.Fatalf("status = %q, want %q", snapshot[0].Status, StatusFailed)
	}
	if snapshot[0].Error != "permission denied" {
		t.Fatalf("error = %q, want %q", snapshot[0].Error, "permission denied")
	}
}

func TestStateCancelQueuedJobKeepsInQueue(t *testing.T) {
	s := NewState()
	job := &Job{ID: "queued-cancel", Type: TypeCopy, Status: StatusQueued}
	s.AddJob(job)
	if !s.CancelJob("queued-cancel") {
		t.Fatal("CancelJob should succeed for queued job")
	}
	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("AllJobs len = %d, want 1", len(all))
	}
	if all[0].Status != StatusCanceled {
		t.Fatalf("status = %q, want canceled", all[0].Status)
	}
	if all[0].FinishedAt.IsZero() {
		t.Fatal("expected FinishedAt set")
	}
}

func TestWorkerUserCancelFromConflict(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		return ErrUserCanceled
	})
	s.StartWorker(stop)
	defer close(stop)

	job := &Job{ID: "user-cancel", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	deadline := time.After(3 * time.Second)
	var gotCanceled Event
	for gotCanceled.Type != EventCanceled {
		select {
		case <-deadline:
			t.Fatal("timeout waiting EventCanceled")
		case ev := <-s.Events():
			if ev.JobID == job.ID && ev.Type == EventCanceled {
				gotCanceled = ev
			}
		}
	}

	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("AllJobs len = %d, want 1", len(all))
	}
	if all[0].Status != StatusCanceled {
		t.Fatalf("status = %q, want %q", all[0].Status, StatusCanceled)
	}
	if gotCanceled.Status != StatusCanceled {
		t.Fatalf("event status = %q, want %q", gotCanceled.Status, StatusCanceled)
	}
}

func TestStateCanceled(t *testing.T) {
	s := NewState()
	job := &Job{ID: "test-3", Type: TypeCopy, Status: StatusQueued}
	s.AddJob(job)
	s.ApplyEvent(Event{Type: EventStarted, JobID: "test-3"})
	s.ApplyEvent(Event{Type: EventCanceled, JobID: "test-3"})

	if a := s.ActiveJob(); a != nil {
		t.Fatal("active job should be nil after cancel")
	}
	snapshot := s.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	if snapshot[0].Status != StatusCanceled {
		t.Fatalf("status = %q, want %q", snapshot[0].Status, StatusCanceled)
	}
}

func TestStateSnapshotIncludesActive(t *testing.T) {
	s := NewState()
	s.AddJob(&Job{ID: "queued-1", Type: TypeCopy, Status: StatusQueued})

	// Simulate worker starting: dequeue and set active.
	job := s.Queue().Dequeue()
	if job == nil {
		t.Fatal("dequeue returned nil")
	}
	s.setActiveForTest(job)

	snapshot := s.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1 (active job), got %+v", len(snapshot), snapshot)
	}
	if snapshot[0].ID != "queued-1" {
		t.Fatalf("snapshot job ID = %s, want queued-1", snapshot[0].ID)
	}
}

func (s *State) setActiveForTest(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = []*Job{job}
}

func TestWorkerArchivesFinishedToHistory(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	job := &Job{ID: "arch-1", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	deadline := time.After(3 * time.Second)
	sawComplete := false
	for !sawComplete {
		select {
		case <-deadline:
			t.Fatal("timeout waiting EventCompleted")
		case ev := <-s.Events():
			if ev.Type == EventCompleted && ev.JobID == job.ID {
				sawComplete = true
			}
		}
	}

	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("AllJobs len = %d, want 1 (finished archived)", len(all))
	}
	if all[0].ID != job.ID || all[0].Status != StatusCompleted {
		t.Fatalf("unexpected job: %+v", all[0])
	}
	if all[0].FinishedAt.IsZero() {
		t.Fatal("expected FinishedAt on archived completed job")
	}

	s.ApplyRetention(RetentionPolicy{ShowFinished: false})
	all = s.AllJobs()
	if len(all) != 0 {
		t.Fatalf("after ShowFinished false, len=%d want 0", len(all))
	}
}

func TestStateSnapshotWithQueuedAndActive(t *testing.T) {
	s := NewState()
	s.AddJob(&Job{ID: "queued-1", Type: TypeCopy, Status: StatusQueued})
	s.AddJob(&Job{ID: "queued-2", Type: TypeMove, Status: StatusQueued})

	// Simulate the worker starting the first job.
	job := s.Queue().Dequeue()
	s.setActiveForTest(job)

	snapshot := s.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot length = %d, want 2, got %+v", len(snapshot), snapshot)
	}
}

func TestStateHasUnfinishedWork(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		s := NewState()
		if s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = true, want false")
		}
	})
	t.Run("queued", func(t *testing.T) {
		s := NewState()
		s.AddJob(&Job{ID: NewJobID(), Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")})
		if !s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = false, want true")
		}
	})
	t.Run("paused_in_queue", func(t *testing.T) {
		s := NewState()
		s.AddJob(&Job{ID: NewJobID(), Type: TypeCopy, Status: StatusPaused, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")})
		if !s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = false, want true")
		}
	})
	t.Run("active_running", func(t *testing.T) {
		s := NewState()
		j := &Job{ID: "run-1", Type: TypeCopy, Status: StatusRunning}
		s.setActiveForTest(j)
		if !s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = false, want true")
		}
	})
	t.Run("active_waiting_decision", func(t *testing.T) {
		s := NewState()
		j := &Job{ID: "wait-1", Type: TypeCopy, Status: StatusWaitingDecision}
		s.setActiveForTest(j)
		if !s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = false, want true")
		}
	})
	t.Run("after_completed_apply_event", func(t *testing.T) {
		s := NewState()
		job := &Job{ID: "done-1", Type: TypeCopy, Status: StatusQueued}
		s.AddJob(job)
		s.ApplyEvent(Event{Type: EventStarted, JobID: "done-1", Status: StatusRunning})
		s.ApplyEvent(Event{Type: EventCompleted, JobID: "done-1"})
		if s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = true, want false")
		}
	})
	t.Run("queued_and_running_active", func(t *testing.T) {
		s := NewState()
		s.AddJob(&Job{ID: "q2", Type: TypeMove, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), Destination: pathloc.MustParse("/y")})
		j := s.Queue().Dequeue()
		s.setActiveForTest(j)
		if !s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = false, want true")
		}
	})
	t.Run("finished_archive_only", func(t *testing.T) {
		s := NewState()
		stop := make(chan struct{})
		s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
			return nil
		})
		s.StartWorker(stop)
		defer close(stop)

		job := &Job{ID: "arch-spinner", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
		s.AddJob(job)

		deadline := time.After(3 * time.Second)
		for {
			select {
			case <-deadline:
				t.Fatal("timeout waiting EventCompleted")
			case ev := <-s.Events():
				if ev.Type == EventCompleted && ev.JobID == job.ID {
					if s.HasUnfinishedWork() {
						t.Fatal("HasUnfinishedWork() = true, want false when only archive remains")
					}
					return
				}
			}
		}
	})
}

func TestAllJobsIncludesEveryDequeuedRunnableBeforeLease(t *testing.T) {
	s := NewState()
	for _, id := range []string{"j0", "j1", "j2"} {
		s.AddJob(&Job{ID: id, Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), Destination: pathloc.MustParse("/y")})
	}
	_ = s.dequeueJob()
	_ = s.dequeueJob()
	s.mu.Lock()
	pendN := len(s.pendingDequeued)
	s.mu.Unlock()
	if pendN != 2 {
		t.Fatalf("len(pendingDequeued)=%d want 2", pendN)
	}
	if n := s.Queue().Len(); n != 1 {
		t.Fatalf("queue.Len()=%d want 1", n)
	}
	if n := len(s.AllJobs()); n != 3 {
		t.Fatalf("AllJobs() len=%d want 3", n)
	}
}

func TestMenuBarStripStatusesOrderDoneOngoingQueued(t *testing.T) {
	s := NewState()
	s.mu.Lock()
	s.finished = []*Job{
		{ID: "d1", Status: StatusCompleted},
		{ID: "d2", Status: StatusFailed},
	}
	s.active = []*Job{{ID: "run", Status: StatusRunning}}
	s.mu.Unlock()
	s.queue.Enqueue(&Job{ID: "scan", Status: StatusScanning, Type: TypeCopy, Sources: pathloc.PathsForTest("/s"), Destination: pathloc.MustParse("/t")})
	s.queue.Enqueue(&Job{ID: "q1", Status: StatusQueued, Type: TypeCopy, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")})
	s.queue.Enqueue(&Job{ID: "q2", Status: StatusPaused, Type: TypeCopy, Sources: pathloc.PathsForTest("/c"), Destination: pathloc.MustParse("/d")})

	got := s.MenuBarStripStatuses()
	want := []string{
		string(StatusCompleted), string(StatusFailed),
		string(StatusRunning),
		string(StatusScanning), string(StatusQueued), string(StatusPaused),
	}
	if len(got) != len(want) {
		t.Fatalf("got len %d %#v, want len %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx %d: got %q want %q (full %#v)", i, got[i], want[i], got)
		}
	}
}

func TestEmitDropsProgressWhenChannelFull(t *testing.T) {
	t.Parallel()
	s := NewState()
	for i := 0; i < cap(s.events)+5; i++ {
		s.emit(Event{Type: EventProgress, JobID: "j1", Status: StatusRunning, DoneFiles: i})
	}
	if len(s.events) != cap(s.events) {
		t.Fatalf("channel len = %d, want full %d", len(s.events), cap(s.events))
	}
}

func TestEmitBlocksUntilCompletedDelivered(t *testing.T) {
	t.Parallel()
	s := NewState()
	for i := 0; i < cap(s.events); i++ {
		s.emit(Event{Type: EventProgress, JobID: "j1", Status: StatusRunning, DoneFiles: i})
	}
	sent := make(chan struct{})
	go func() {
		s.emit(Event{Type: EventCompleted, JobID: "j1", Status: StatusCompleted})
		close(sent)
	}()
	select {
	case <-sent:
		t.Fatal("completed send should block until channel has space")
	case <-time.After(20 * time.Millisecond):
	}
	<-s.events // free one slot
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("completed not delivered after making channel space")
	}
	found := false
	for len(s.events) > 0 {
		ev := <-s.events
		if ev.Type == EventCompleted {
			found = true
		}
	}
	if !found {
		t.Fatal("completed event not in channel")
	}
}

func TestEventTypeDroppableWhenChannelFull(t *testing.T) {
	t.Parallel()
	if !EventProgress.DroppableWhenChannelFull() {
		t.Fatal("progress should be droppable")
	}
	if EventCompleted.DroppableWhenChannelFull() {
		t.Fatal("completed should not be droppable")
	}
}

func TestFirstWaitingBlockerJobFIFO(t *testing.T) {
	t.Parallel()
	s := NewState()
	b1 := &BlockerDetails{Kind: BlockerKindConflict, Conflict: &ConflictEvent{Source: "/a"}}
	b2 := &BlockerDetails{Kind: BlockerKindConflict, Conflict: &ConflictEvent{Source: "/b"}}
	j1 := &Job{ID: "first", Status: StatusWaitingDecision, PendingBlocker: b1}
	j2 := &Job{ID: "second", Status: StatusWaitingDecision, PendingBlocker: b2}
	s.mu.Lock()
	s.waitingBlocker = []*Job{j1, j2}
	s.mu.Unlock()

	got := s.FirstWaitingBlockerJob()
	if got == nil {
		t.Fatal("FirstWaitingBlockerJob() = nil, want first job")
	}
	if got.ID != "first" {
		t.Fatalf("FirstWaitingBlockerJob().ID = %q, want first", got.ID)
	}
	if got.PendingBlocker == nil || got.PendingBlocker.Conflict == nil || got.PendingBlocker.Conflict.Source != "/a" {
		t.Fatalf("unexpected blocker snapshot: %+v", got.PendingBlocker)
	}
	if s.FirstWaitingBlockerJob() == j1 {
		t.Fatal("FirstWaitingBlockerJob should return a copy, not the stored pointer")
	}
}

// TestCancelJobWhileQueuedButStillStreamingCancelsProducer covers the pipelining correctness
// fix in CancelJob: under the streamed pre-scan, a job's plan producer can still be running
// (blocked trying to send its next item into a full downstream channel) long after the job has
// left StatusScanning for StatusQueued. Before the fix, only the StatusScanning branch of
// CancelJob canceled the producer's context, so canceling a queued-but-still-streaming job left
// its producer goroutine blocked forever. This test never starts the worker, so the job stays
// queued (never dequeued) with its producer still live when CancelJob runs.
func TestCancelJobWhileQueuedButStillStreamingCancelsProducer(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: 5 * time.Millisecond})

	firstItem := make(chan struct{})
	producerCtxCanceled := make(chan struct{})
	producerExited := make(chan struct{})
	before := runtime.NumGoroutine()

	s.SetScanFunc(func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) PlanProducer {
		itemsCh := make(chan ops.PlanItem) // unbuffered and never read: models a producer
		doneCh := make(chan struct{})      // still walking with a full downstream channel.
		go func() {
			defer close(producerExited)
			defer close(doneCh)
			close(firstItem)
			select {
			case itemsCh <- ops.PlanItem{}:
				t.Error("producer send should not succeed: nothing reads itemsCh in this test")
			case <-ctx.Done():
				close(producerCtxCanceled)
			}
		}()
		return PlanProducer{
			Items:     itemsCh,
			FirstItem: firstItem,
			Totals:    func() (int, int, int64) { return 0, 0, 0 },
			Done:      doneCh,
			Err:       func() error { return ctx.Err() },
		}
	})

	job := &Job{ID: "still-streaming", Type: TypeCopy, Status: StatusScanning, Sources: pathloc.PathsForTest("/a"), Destination: pathloc.MustParse("/b")}
	s.AddJob(job)

	deadline := time.After(3 * time.Second)
	for {
		all := s.AllJobs()
		if len(all) == 1 && all[0].Status == StatusQueued {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting StatusQueued; last seen: %+v", all)
		case <-time.After(5 * time.Millisecond):
		}
	}

	// The producer must still be blocked on its send at this point (queued, never dequeued).
	select {
	case <-producerCtxCanceled:
		t.Fatal("producer context canceled before CancelJob was even called")
	default:
	}

	if !s.CancelJob(job.ID) {
		t.Fatal("CancelJob() = false, want true for a queued job")
	}

	select {
	case <-producerCtxCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for producer's context to be canceled by CancelJob")
	}
	select {
	case <-producerExited:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for producer goroutine to exit after cancellation")
	}

	// Secondary, coarser check: goroutine count should settle back near its starting point
	// rather than staying elevated (allow some slack for unrelated background goroutines).
	var after int
	for i := 0; i < 20; i++ {
		after = runtime.NumGoroutine()
		if after <= before+2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if after > before+2 {
		t.Fatalf("NumGoroutine() after cancel = %d, want <= %d (before=%d)", after, before+2, before)
	}
}

// TestRetryJobRequeuesFailedJob covers RetryJob re-queuing a failed job under the same ID and
// clearing its error, and rejecting retry once the job is no longer failed.
func TestRetryJobRequeuesFailedJob(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	defer close(stop)

	var attempts atomic.Int32
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		if attempts.Add(1) == 1 {
			return errors.New("permission denied")
		}
		return nil
	})
	s.StartWorker(stop)

	job := &Job{ID: "retry-me", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), TotalFiles: 1}
	s.AddJob(job)

	deadline := time.After(5 * time.Second)
	var gotFailed bool
	for !gotFailed {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for EventFailed")
		case ev := <-s.Events():
			if ev.Type == EventFailed && ev.JobID == "retry-me" {
				gotFailed = true
			}
		}
	}

	if !s.RetryJob("retry-me") {
		t.Fatal("RetryJob() = false, want true for a failed job")
	}
	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("AllJobs len = %d, want 1", len(all))
	}
	if all[0].Status != StatusQueued {
		t.Fatalf("status after retry = %q, want %q", all[0].Status, StatusQueued)
	}
	if all[0].Error != "" {
		t.Fatalf("error after retry = %q, want empty", all[0].Error)
	}

	var gotCompleted bool
	for !gotCompleted {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for EventCompleted")
		case ev := <-s.Events():
			if ev.Type == EventCompleted && ev.JobID == "retry-me" {
				gotCompleted = true
			}
		}
	}

	if s.RetryJob("retry-me") {
		t.Fatal("RetryJob() = true, want false for a completed job")
	}
}

// waitPendingDequeuedCount waits until exactly n jobs sit in pendingDequeued (dequeued, not yet
// holding the transfer lease).
func waitPendingDequeuedCount(t *testing.T, s *State, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		got := len(s.pendingDequeued)
		s.mu.Unlock()
		if got == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	s.mu.Lock()
	got := len(s.pendingDequeued)
	s.mu.Unlock()
	t.Fatalf("timeout waiting pendingDequeued len=%d (got %d)", n, got)
}

func jobStatusLocked(s *State, job *Job) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return job.Status
}

// assertPendingNeverStarted fails if a lease-waiting job entered TransferFunc or left StatusCanceled
// after the holder released the lease (the R04-001 race window).
func assertPendingNeverStarted(t *testing.T, s *State, enteredIDs *sync.Map, pending ...*Job) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, job := range pending {
			if _, saw := enteredIDs.Load(job.ID); saw {
				t.Fatalf("TransferFunc entered for %s after cancel/stop", job.ID)
			}
			st := jobStatusLocked(s, job)
			if st != StatusCanceled {
				t.Fatalf("%s status = %q, want %q (started after cancel/stop)", job.ID, st, StatusCanceled)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, job := range pending {
		if _, saw := enteredIDs.Load(job.ID); saw {
			t.Fatalf("TransferFunc entered for %s after cancel/stop", job.ID)
		}
		if st := jobStatusLocked(s, job); st != StatusCanceled {
			t.Fatalf("%s status = %q, want %q", job.ID, st, StatusCanceled)
		}
	}
}

// TestCancelJobPendingDequeuedNeverEntersTransferFunc covers R04-001 for ordinary CancelJob:
// two transfers waiting on the lease must not enter TransferFunc after they are canceled,
// and their runJob goroutines must exit once the holder releases the lease.
func TestCancelJobPendingDequeuedNeverEntersTransferFunc(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	holderEntered := make(chan struct{})
	holderLeft := make(chan struct{})
	var enteredIDs sync.Map
	var extraEntries atomic.Int32

	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		if _, loaded := enteredIDs.LoadOrStore(job.ID, true); loaded {
			extraEntries.Add(1)
		}
		if job.ID == "holder" {
			close(holderEntered)
			<-release
			close(holderLeft)
			return nil
		}
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	before := runtime.NumGoroutine()
	holder := &Job{ID: "holder", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/alpha"), Destination: pathloc.MustParse("/bravo")}
	s.AddJob(holder)
	select {
	case <-holderEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for lease holder to enter TransferFunc")
	}

	pendA := &Job{ID: "pend-a", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/charlie"), Destination: pathloc.MustParse("/delta")}
	pendB := &Job{ID: "pend-b", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/echo"), Destination: pathloc.MustParse("/foxtrot")}
	s.AddJob(pendA)
	s.AddJob(pendB)
	waitPendingDequeuedCount(t, s, 2)

	if !s.CancelJob("pend-a") {
		t.Fatal("CancelJob(pend-a) = false, want true")
	}
	if !s.CancelJob("pend-b") {
		t.Fatal("CancelJob(pend-b) = false, want true")
	}

	close(release)
	select {
	case <-holderLeft:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for lease holder to leave TransferFunc")
	}
	assertPendingNeverStarted(t, s, &enteredIDs, pendA, pendB)

	if extraEntries.Load() != 0 {
		t.Fatalf("duplicate TransferFunc entries = %d, want 0", extraEntries.Load())
	}

	var after int
	for i := 0; i < 40; i++ {
		after = runtime.NumGoroutine()
		if after <= before+3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("NumGoroutine() after cancel = %d, want <= %d (before=%d)", after, before+3, before)
}

// TestShutdownPendingDequeuedNeverEntersTransferFunc covers R04-001 for worker stop:
// lease-waiting transfers must not start after shutdown, and all runJob goroutines must exit
// once the holder releases the lease.
func TestShutdownPendingDequeuedNeverEntersTransferFunc(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	holderEntered := make(chan struct{})
	holderLeft := make(chan struct{})
	var enteredIDs sync.Map

	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		enteredIDs.Store(job.ID, true)
		if job.ID == "holder" {
			close(holderEntered)
			<-release
			close(holderLeft)
			return nil
		}
		return nil
	})
	s.StartWorker(stop)

	before := runtime.NumGoroutine()
	holder := &Job{ID: "holder", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/golf"), Destination: pathloc.MustParse("/hotel")}
	s.AddJob(holder)
	select {
	case <-holderEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for lease holder to enter TransferFunc")
	}

	pendA := &Job{ID: "pend-a", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/india"), Destination: pathloc.MustParse("/juliet")}
	pendB := &Job{ID: "pend-b", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/kilo"), Destination: pathloc.MustParse("/lima")}
	s.AddJob(pendA)
	s.AddJob(pendB)
	waitPendingDequeuedCount(t, s, 2)

	close(stop)
	close(release)
	select {
	case <-holderLeft:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for lease holder to leave TransferFunc")
	}
	assertPendingNeverStarted(t, s, &enteredIDs, pendA, pendB)

	var after int
	for i := 0; i < 40; i++ {
		after = runtime.NumGoroutine()
		if after <= before+3 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("NumGoroutine() after stop = %d, want <= %d (before=%d)", after, before+3, before)
}
