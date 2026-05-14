package jobs

import (
	"context"
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

	s.AddJob(&Job{ID: "paused-front", Type: TypeCopy, Status: StatusPaused, Sources: []string{"/x"}, Destination: "/y"})
	s.AddJob(&Job{ID: "run-second", Type: TypeCopy, Status: StatusQueued, Sources: []string{"/p"}, Destination: "/q"})

	select {
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting first runnable job")
	case id := <-order:
		if id != "run-second" {
			t.Fatalf("first run got %q, want run-second", id)
		}
	}
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
		s.AddJob(&Job{ID: "job-a", Type: TypeCopy, Status: StatusQueued, Sources: []string{"/x"}, Destination: "/y"})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		s.AddJob(&Job{ID: "job-b", Type: TypeCopy, Status: StatusQueued, Sources: []string{"/p"}, Destination: "/q"})
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

func TestStateEmitHook(t *testing.T) {
	var n atomic.Int32
	s := NewState()
	s.SetEmitHook(func(ev Event) {
		if ev.Type == EventEnqueued {
			n.Add(1)
		}
	})
	job := &Job{ID: NewJobID(), Type: TypeCopy, Status: StatusQueued, Sources: []string{"/a"}, Destination: "/b"}
	s.AddJob(job)
	if n.Load() != 1 {
		t.Fatalf("emit hook calls = %d, want 1", n.Load())
	}
}

func TestStateAddJob(t *testing.T) {
	s := NewState()
	job := &Job{ID: NewJobID(), Type: TypeCopy, Status: StatusQueued, Sources: []string{"/a"}, Destination: "/b"}
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
	s.active = job
}

func TestWorkerArchivesFinishedToHistory(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error {
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	job := &Job{ID: "arch-1", Type: TypeCopy, Status: StatusQueued, Sources: []string{"/a"}, Destination: "/b"}
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
		s.AddJob(&Job{ID: NewJobID(), Type: TypeCopy, Status: StatusQueued, Sources: []string{"/a"}, Destination: "/b"})
		if !s.HasUnfinishedWork() {
			t.Fatal("HasUnfinishedWork() = false, want true")
		}
	})
	t.Run("paused_in_queue", func(t *testing.T) {
		s := NewState()
		s.AddJob(&Job{ID: NewJobID(), Type: TypeCopy, Status: StatusPaused, Sources: []string{"/a"}, Destination: "/b"})
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
		s.AddJob(&Job{ID: "q2", Type: TypeMove, Status: StatusQueued, Sources: []string{"/x"}, Destination: "/y"})
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

		job := &Job{ID: "arch-spinner", Type: TypeCopy, Status: StatusQueued, Sources: []string{"/a"}, Destination: "/b"}
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
