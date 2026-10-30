package jobs

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// applyUntilFinished applies the state's events like the app loop until job id finishes and
// returns its final copy.
func applyUntilFinished(t *testing.T, s *State, id string) *Job {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-s.Events():
			s.ApplyEvent(ev)
			if ev.JobID == id && ev.Status.IsFinished() {
				return s.Job(id)
			}
		case <-deadline:
			t.Fatalf("job %s did not finish", id)
			return nil
		}
	}
}

func TestTaskJobReportsProgressAndWarnings(t *testing.T) {
	s := NewState()
	s.SetScanConfig(ScanConfig{ProgressMinInterval: 0})
	stop := make(chan struct{})
	s.StartWorker(stop)
	defer close(stop)

	task := &Task{Title: "lantern sweep", Lease: "lantern", Run: func(_ context.Context, r TaskReporter) error {
		for i := 1; i <= 3; i++ {
			r.Progress(TaskProgress{TotalFiles: 3, DoneFiles: i, DoneBytes: int64(i) * 100, CurrentPath: "/orchard/pebble"})
		}
		r.Warn("skipped /orchard/thistle: unreadable")
		return nil
	}}
	job := NewTaskJob(task, pathloc.PathsForTest("/orchard"))
	s.AddJob(job)
	got := applyUntilFinished(t, s, job.ID)

	if got.Status != StatusCompleted {
		t.Fatalf("status = %s, want completed", got.Status)
	}
	if got.Type != TypeTask || got.Title() != "lantern sweep" {
		t.Fatalf("type/title = %s/%q", got.Type, got.Title())
	}
	if got.TotalFiles != 3 || got.DoneFiles != 3 || got.DoneBytes != 300 {
		t.Fatalf("totals = %d/%d files %d bytes, want 3/3 300", got.DoneFiles, got.TotalFiles, got.DoneBytes)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "skipped /orchard/thistle: unreadable" {
		t.Fatalf("warnings = %v", got.Warnings)
	}
}

// TestTaskJobStartSourceAdvancesPhases: a task working through its sources finishes each one
// as it starts the next.
func TestTaskJobStartSourceAdvancesPhases(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	s.StartWorker(stop)
	defer close(stop)

	var mid []SourcePhase
	var job *Job
	job = NewTaskJob(&Task{Title: "ferry", Lease: "ferry", Run: func(_ context.Context, r TaskReporter) error {
		r.StartSource(0)
		r.StartSource(1)
		mid = job.SourcePhases()
		return nil
	}}, pathloc.PathsForTest("/orchard/acorn", "/orchard/bramble", "/orchard/clover"))
	s.AddJob(job)
	applyUntilFinished(t, s, job.ID)
	if want := []SourcePhase{SourceFinished, SourceStarted, SourceQueued}; !slices.Equal(mid, want) {
		t.Fatalf("phases = %v, want %v", mid, want)
	}
}

func TestTaskJobFailsWithError(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	s.StartWorker(stop)
	defer close(stop)

	job := NewTaskJob(&Task{Title: "meadow", Lease: "meadow", Run: func(context.Context, TaskReporter) error {
		return errors.New("index is locked")
	}}, nil)
	s.AddJob(job)
	got := applyUntilFinished(t, s, job.ID)
	if got.Status != StatusFailed || got.Error != "index is locked" {
		t.Fatalf("status/error = %s/%q", got.Status, got.Error)
	}
}

func TestTaskJobCancelCancelsContext(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	s.StartWorker(stop)
	defer close(stop)

	running := make(chan struct{})
	job := NewTaskJob(&Task{Title: "harbor", Lease: "harbor", Run: func(ctx context.Context, r TaskReporter) error {
		r.Progress(TaskProgress{DoneFiles: 1, CurrentPath: "/harbor/rope"})
		close(running)
		<-ctx.Done()
		return ctx.Err()
	}}, pathloc.PathsForTest("/harbor"))
	s.AddJob(job)
	<-running
	if !s.HasUnfinishedWork() {
		t.Fatal("a running task must count as unfinished work (quit confirmation)")
	}
	if !s.CancelJob(job.ID) {
		t.Fatal("CancelJob did not find the running task")
	}
	got := applyUntilFinished(t, s, job.ID)
	if got.Status != StatusCanceled {
		t.Fatalf("status = %s, want canceled", got.Status)
	}
	if got.DoneFiles != 1 {
		t.Fatalf("done files = %d, want the last reported 1", got.DoneFiles)
	}
}

// TestTaskLeaseSerializesSameName checks two tasks on one lease never overlap, while a task on
// another lease and a transfer run beside them.
func TestTaskLeaseSerializesSameName(t *testing.T) {
	s := NewState()
	stop := make(chan struct{})
	release := make(chan struct{})
	copyStarted := make(chan struct{})
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) BlockerAnswer) error {
		close(copyStarted)
		<-release
		return nil
	})
	s.StartWorker(stop)
	defer close(stop)

	s.AddJob(&Job{ID: "copy-walnut", Type: TypeCopy, Status: StatusQueued, Sources: pathloc.PathsForTest("/walnut"), Destination: pathloc.MustParse("/cedar")})
	<-copyStarted

	var mu sync.Mutex
	inLease, maxInLease := 0, 0
	otherRan := make(chan struct{})
	sameLease := func(context.Context, TaskReporter) error {
		mu.Lock()
		inLease++
		maxInLease = max(maxInLease, inLease)
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		inLease--
		mu.Unlock()
		return nil
	}
	a := NewTaskJob(&Task{Title: "a", Lease: "fileindex", Run: sameLease}, nil)
	b := NewTaskJob(&Task{Title: "b", Lease: "fileindex", Run: sameLease}, nil)
	c := NewTaskJob(&Task{Title: "c", Lease: "compass", Run: func(context.Context, TaskReporter) error {
		close(otherRan)
		return nil
	}}, nil)
	s.AddJob(a)
	s.AddJob(b)
	s.AddJob(c)

	select {
	case <-otherRan:
	case <-time.After(3 * time.Second):
		t.Fatal("a task on its own lease must run while the copy holds the transfer lease")
	}
	applyUntilFinished(t, s, a.ID)
	applyUntilFinished(t, s, b.ID)
	close(release)
	mu.Lock()
	defer mu.Unlock()
	if maxInLease != 1 {
		t.Fatalf("tasks on one lease overlapped: %d at once", maxInLease)
	}
}

func TestTaskJobCannotPauseOrRetry(t *testing.T) {
	s := NewState()
	job := NewTaskJob(&Task{Title: "ember", Lease: "ember"}, nil)
	s.AddJob(job) // no worker: it stays queued
	if s.PauseQueuedJob(job.ID) {
		t.Fatal("a queued task must not pause")
	}
	if !s.CancelJob(job.ID) {
		t.Fatal("cancel queued task")
	}
	for range len(s.Events()) {
		s.ApplyEvent(<-s.Events())
	}
	if got := s.Job(job.ID); got == nil || got.Status != StatusCanceled {
		t.Fatalf("job = %+v, want canceled", got)
	}
	if s.RetryJob(job.ID) {
		t.Fatal("a task must not retry")
	}
}
