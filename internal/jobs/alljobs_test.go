package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestDedupeJobsByIDKeepsFirstBucket(t *testing.T) {
	t.Parallel()
	a := &Job{ID: "a", Status: StatusRunning}
	b := &Job{ID: "b", Status: StatusQueued}
	dup := &Job{ID: "a", Status: StatusWaitingDecision}
	got := dedupeJobsByID([]*Job{a, dup, b, dup})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[0].Status != StatusRunning {
		t.Fatalf("first = %+v, want active running a", got[0])
	}
	if got[1].ID != "b" {
		t.Fatalf("second = %s, want b", got[1].ID)
	}
}

func TestAllJobsDedupesSameIDInActiveAndWaitingBlocker(t *testing.T) {
	t.Parallel()
	s := NewState()
	j := &Job{
		ID:          "copy-1",
		Type:        TypeCopy,
		Status:      StatusWaitingDecision,
		Sources:     pathloc.PathsForTest("/src"),
		Destination: pathloc.MustParse("/dst"),
	}
	s.mu.Lock()
	s.active = j
	s.waitingBlocker = []*Job{j}
	s.mu.Unlock()

	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("AllJobs() len = %d, want 1 (deduped)", len(all))
	}
	if all[0].ID != "copy-1" {
		t.Fatalf("job ID = %q", all[0].ID)
	}
}

func TestAllJobsDedupesPendingDequeuedAndWaitingBlocker(t *testing.T) {
	t.Parallel()
	s := NewState()
	j := &Job{
		ID:          "copy-2",
		Type:        TypeCopy,
		Status:      StatusWaitingDecision,
		Sources:     pathloc.PathsForTest("/a"),
		Destination: pathloc.MustParse("/b"),
	}
	s.mu.Lock()
	s.pendingDequeued = []*Job{j}
	s.waitingBlocker = []*Job{j}
	s.mu.Unlock()

	if len(s.AllJobs()) != 1 {
		t.Fatalf("AllJobs() len = %d, want 1", len(s.AllJobs()))
	}
}

func TestMenuBarStripStatusesDedupesOverlappingBuckets(t *testing.T) {
	t.Parallel()
	s := NewState()
	j := &Job{
		ID:          "copy-1",
		Type:        TypeCopy,
		Status:      StatusWaitingDecision,
		Sources:     pathloc.PathsForTest("/src"),
		Destination: pathloc.MustParse("/dst"),
	}
	s.mu.Lock()
	s.active = j
	s.waitingBlocker = []*Job{j}
	s.mu.Unlock()

	got := s.MenuBarStripStatuses()
	if len(got) != 1 {
		t.Fatalf("MenuBarStripStatuses() = %#v, want one glyph", got)
	}
	if got[0] != string(StatusWaitingDecision) {
		t.Fatalf("status = %q, want %q", got[0], StatusWaitingDecision)
	}
}

func TestAppendPendingDequeuedSkipsDuplicateID(t *testing.T) {
	t.Parallel()
	s := NewState()
	j := &Job{ID: "j1", Type: TypeCopy, Status: StatusQueued}
	s.mu.Lock()
	s.appendPendingDequeuedUnlocked(j)
	s.appendPendingDequeuedUnlocked(j)
	s.mu.Unlock()
	s.mu.Lock()
	n := len(s.pendingDequeued)
	s.mu.Unlock()
	if n != 1 {
		t.Fatalf("pendingDequeued len = %d, want 1", n)
	}
}
