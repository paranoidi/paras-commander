package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestFirstJobEntryWaitingDecisionIndex(t *testing.T) {
	t.Parallel()
	blocker := &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}
	entries := []JobEntry{
		{ID: "a", Status: string(jobs.StatusRunning)},
		{ID: "b", Status: string(jobs.StatusQueued)},
		{ID: "c", Status: string(jobs.StatusWaitingDecision), PendingBlocker: blocker},
		{ID: "d", Status: string(jobs.StatusWaitingDecision), PendingBlocker: blocker},
	}
	if got := FirstJobEntryWaitingDecisionIndex(entries); got != 2 {
		t.Fatalf("index = %d, want 2", got)
	}
	if got := FirstJobEntryWaitingDecisionIndex(nil); got != -1 {
		t.Fatalf("empty index = %d, want -1", got)
	}
	if got := FirstJobEntryWaitingDecisionIndex([]JobEntry{{ID: "x", Status: string(jobs.StatusRunning)}}); got != -1 {
		t.Fatalf("no blocker index = %d, want -1", got)
	}
}
