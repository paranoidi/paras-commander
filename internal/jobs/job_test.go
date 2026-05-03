package jobs

import (
	"testing"
	"time"
)

func TestJobLifecycle(t *testing.T) {
	job := &Job{
		ID:          NewJobID(),
		Type:        TypeCopy,
		Status:      StatusQueued,
		Sources:     []string{"/src/a.txt"},
		Destination: "/dst",
		TotalFiles:  1,
	}

	if job.Status != StatusQueued {
		t.Fatalf("initial status = %q, want %q", job.Status, StatusQueued)
	}

	job.Status = StatusRunning
	if job.Status != StatusRunning {
		t.Fatalf("running status = %q, want %q", job.Status, StatusRunning)
	}

	job.DoneFiles = 1
	job.DoneBytes = 1024
	job.CurrentPath = "/src/a.txt"
	job.StartedAt = time.Now()
	job.Status = StatusCompleted
	job.FinishedAt = time.Now()

	if !job.Status.IsFinished() {
		t.Fatal("completed should be finished")
	}
}

func TestFinishedStatuses(t *testing.T) {
	for _, s := range FinishedStatuses() {
		if !s.IsFinished() {
			t.Fatalf("%q should be finished", s)
		}
	}
}

func TestNewJobIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for range 100 {
		id := NewJobID()
		if ids[id] {
			t.Fatalf("duplicate job ID: %s", id)
		}
		ids[id] = true
	}
}
