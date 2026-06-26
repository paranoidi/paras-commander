package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
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

func TestJobBlockerDialogDecision(t *testing.T) {
	t.Parallel()
	conflict := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}
	if d, ok := JobBlockerDialogDecision(conflict, 0); !ok || d != jobs.DecisionOverwrite {
		t.Fatalf("focus 0 = %q %v, want overwrite", d, ok)
	}
	if d, ok := JobBlockerDialogDecision(conflict, 5); ok {
		t.Fatalf("postpone focus should not map to decision, got %q", d)
	}
	if !JobBlockerDialogIsPostpone(conflict, JobBlockerDialogPostponeFocus(conflict)) {
		t.Fatal("expected postpone focus")
	}
	disk := jobs.BlockerDetails{Kind: jobs.BlockerKindDiskSpace, DiskSpace: &jobs.DiskSpaceBlockerDetails{}}
	if d, ok := JobBlockerDialogDecision(disk, 0); !ok || d != jobs.DecisionRetry {
		t.Fatalf("disk retry = %q %v", d, ok)
	}
	if d, ok := JobBlockerDialogDecision(disk, 2); ok {
		t.Fatalf("disk postpone should not map, got %q", d)
	}
}

func TestJobBlockerDialogMoveFocusFileConflictRows(t *testing.T) {
	t.Parallel()
	conflict := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}

	// Overwrite All (2) -> Right -> Skip All (3)
	focus := 2
	got, ok := JobBlockerDialogMoveFocus(conflict, focus, tcell.KeyRight)
	if !ok || got != 3 {
		t.Fatalf("Right from Overwrite All = %d %v, want 3", got, ok)
	}

	// Skip All (3) -> Left -> Overwrite All (2)
	got, ok = JobBlockerDialogMoveFocus(conflict, 3, tcell.KeyLeft)
	if !ok || got != 2 {
		t.Fatalf("Left from Skip All = %d %v, want 2", got, ok)
	}
}
