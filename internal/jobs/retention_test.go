package jobs

import (
	"testing"
)

func jobIDs(jobs []*Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		if j != nil {
			out[i] = j.ID
		}
	}
	return out
}

func TestRetentionPolicyShowFinishedFalse(t *testing.T) {
	q := NewQueue()
	q.Enqueue(&Job{ID: "a", Status: StatusCompleted})
	q.Enqueue(&Job{ID: "b", Status: StatusQueued})

	p := RetentionPolicy{ShowFinished: false, KeepFinished: 20}
	p.Apply(q)

	remaining := q.Jobs()
	if len(remaining) != 1 || remaining[0].ID != "b" {
		t.Fatalf("remaining = %v, want just 'b'", jobIDs(remaining))
	}
}

func TestRetentionPolicyKeepFinished(t *testing.T) {
	q := NewQueue()
	q.Enqueue(&Job{ID: "a", Status: StatusCompleted})
	q.Enqueue(&Job{ID: "b", Status: StatusCompleted})
	q.Enqueue(&Job{ID: "c", Status: StatusCompleted})
	q.Enqueue(&Job{ID: "d", Status: StatusQueued})

	p := RetentionPolicy{ShowFinished: true, KeepFinished: 2}
	p.Apply(q)

	remaining := q.Jobs()
	if len(remaining) != 3 { // 2 retained finished + 1 queued
		t.Fatalf("remaining = %d, want 3, got IDs: %v", len(remaining), jobIDs(remaining))
	}
	if remaining[0].ID != "b" || remaining[1].ID != "c" || remaining[2].ID != "d" {
		t.Fatalf("expected 'b','c','d', got %v", jobIDs(remaining))
	}
}

func TestDefaultRetentionPolicy(t *testing.T) {
	p := DefaultRetentionPolicy()
	if !p.ShowFinished {
		t.Fatal("default should show finished")
	}
	if p.KeepFinished != 20 {
		t.Fatalf("KeepFinished = %d, want 20", p.KeepFinished)
	}
}
