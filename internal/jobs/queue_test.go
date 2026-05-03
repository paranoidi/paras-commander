package jobs

import (
	"testing"
)

func TestQueueRemoveJobByID(t *testing.T) {
	q := NewQueue()
	j1 := &Job{ID: "a", Status: StatusQueued}
	j2 := &Job{ID: "b", Status: StatusQueued}
	q.Enqueue(j1)
	q.Enqueue(j2)
	if !q.RemoveJobByID("a") {
		t.Fatal("expected remove a")
	}
	if q.Len() != 1 {
		t.Fatalf("len = %d, want 1", q.Len())
	}
	if q.Peek().ID != "b" {
		t.Fatalf("front = %s, want b", q.Peek().ID)
	}
	if q.RemoveJobByID("x") {
		t.Fatal("remove missing should be false")
	}
}

func TestQueuePauseQueuedJob(t *testing.T) {
	q := NewQueue()
	j := &Job{ID: "x", Status: StatusQueued}
	q.Enqueue(j)
	if !q.PauseQueuedJob("x") {
		t.Fatal("PauseQueuedJob")
	}
	if q.Peek().Status != StatusPaused {
		t.Fatalf("status = %q", q.Peek().Status)
	}
	if q.PauseQueuedJob("x") || q.PauseQueuedJob("missing") {
		t.Fatal("PauseQueuedJob should fail when not queued")
	}
}

func TestQueueDequeueRunnableSkipsPaused(t *testing.T) {
	q := NewQueue()
	a := &Job{ID: "a", Status: StatusPaused}
	b := &Job{ID: "b", Status: StatusQueued}
	q.Enqueue(a)
	q.Enqueue(b)
	got := q.DequeueRunnable()
	if got == nil || got.ID != "b" {
		t.Fatalf("DequeueRunnable = %#v, want job b", got)
	}
	if q.Len() != 1 || q.Peek().ID != "a" {
		t.Fatalf("queue after = %#v", q.Jobs())
	}
	if q.DequeueRunnable() != nil {
		t.Fatal("expected nil when only paused remains")
	}
	if !q.ResumePausedJob("a") {
		t.Fatal("ResumePausedJob")
	}
	if q.Peek().Status != StatusQueued {
		t.Fatalf("status = %q", q.Peek().Status)
	}
	got = q.DequeueRunnable()
	if got == nil || got.ID != "a" {
		t.Fatalf("second DequeueRunnable = %#v", got)
	}
}

func TestQueueSwapQueued(t *testing.T) {
	q := NewQueue()
	q.Enqueue(&Job{ID: "1", Status: StatusQueued})
	q.Enqueue(&Job{ID: "2", Status: StatusQueued})
	q.Enqueue(&Job{ID: "3", Status: StatusQueued})
	if !q.SwapQueued(0, 2) {
		t.Fatal("swap failed")
	}
	if q.Jobs()[0].ID != "3" || q.Jobs()[1].ID != "2" || q.Jobs()[2].ID != "1" {
		t.Fatalf("order = %#v", q.Jobs())
	}
	if q.SwapQueued(0, 5) {
		t.Fatal("swap out of range should fail")
	}
}
