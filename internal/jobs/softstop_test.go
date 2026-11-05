package jobs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestSoftStopJobRefusesQueuedJobsAndTasks(t *testing.T) {
	s := NewState()
	s.AddJob(&Job{ID: "birch", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest("/x"), TotalFiles: 1})
	if s.SoftStopJob("birch") {
		t.Fatal("SoftStopJob accepted a queued job")
	}
	if s.SoftStopJob("missing") {
		t.Fatal("SoftStopJob accepted an unknown job")
	}
	if TypeTask.SoftStoppable() || !TypeCopy.SoftStoppable() {
		t.Fatal("only task jobs must refuse a soft stop")
	}
}

// TestSoftStopRunningJobEndsCanceled: a soft stop requested mid-run reaches the executor through
// the job context, which stops before the next item and ends the job canceled.
func TestSoftStopRunningJobEndsCanceled(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "lantern.txt")
	if err := os.WriteFile(file, []byte("lantern"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewState()
	stop := make(chan struct{})
	defer close(stop)
	started, release := make(chan struct{}), make(chan struct{})
	s.SetTransferFunc(func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) BlockerAnswer) error {
		close(started)
		<-release
		_, _, err := ops.ExecuteDeletePaths(ctx, []string{file}, nil, nil, nil)
		return err
	})
	s.StartWorker(stop)
	s.AddJob(&Job{ID: "oak", Type: TypeDelete, Status: StatusQueued, Sources: pathloc.PathsForTest(file), TotalFiles: 1})

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("job never started")
	}
	if !s.SoftStopJob("oak") {
		t.Fatal("SoftStopJob refused a running job")
	}
	close(release)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for EventCanceled")
		case ev := <-s.Events():
			if ev.JobID != "oak" {
				continue
			}
			if ev.Type == EventCanceled {
				if _, err := os.Stat(file); err != nil {
					t.Fatalf("item after the soft stop was deleted: %v", err)
				}
				if s.Job("oak").Status != StatusCanceled {
					t.Fatalf("status = %v, want canceled", s.Job("oak").Status)
				}
				return
			}
			if ev.Type == EventCompleted || ev.Type == EventFailed {
				t.Fatalf("job ended %v, want canceled", ev.Type)
			}
		}
	}
}
