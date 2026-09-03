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
	if d, ok := JobBlockerDialogDecision(conflict, 4); !ok || d != jobs.DecisionOverwriteAllSameSize {
		t.Fatalf("focus 4 = %q %v, want overwrite-all-same-size", d, ok)
	}
	if d, ok := JobBlockerDialogDecision(conflict, 5); !ok || d != jobs.DecisionCancel {
		t.Fatalf("focus 5 = %q %v, want cancel", d, ok)
	}
	if d, ok := JobBlockerDialogDecision(conflict, JobBlockerDialogPostponeFocus(conflict)); ok {
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

	// Skip All (3) -> Down -> Postpone (6, the lone button on row 3)
	got, ok = JobBlockerDialogMoveFocus(conflict, 3, tcell.KeyDown)
	if !ok || got != 6 {
		t.Fatalf("Down from Skip All = %d %v, want 6", got, ok)
	}

	// Postpone (6) -> Up -> Skip All (3)
	got, ok = JobBlockerDialogMoveFocus(conflict, 6, tcell.KeyUp)
	if !ok || got != 3 {
		t.Fatalf("Up from Postpone = %d %v, want 3", got, ok)
	}

	// Postpone (6) -> Right -> stays (last button, no wrap)
	got, ok = JobBlockerDialogMoveFocus(conflict, 6, tcell.KeyRight)
	if !ok || got != 6 {
		t.Fatalf("Right from Postpone = %d %v, want 6 (no move)", got, ok)
	}
}

func TestJobBlockerDialogFocusFromShortcutMatchSize(t *testing.T) {
	t.Parallel()
	conflict := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}
	focus, ok := JobBlockerDialogFocusFromShortcut(conflict, 'm')
	if !ok || focus != 4 {
		t.Fatalf("shortcut m = %d %v, want focus 4", focus, ok)
	}
	focus, ok = JobBlockerDialogFocusFromShortcut(conflict, 'C')
	if !ok || focus != 5 {
		t.Fatalf("shortcut C = %d %v, want focus 5", focus, ok)
	}
	focus, ok = JobBlockerDialogFocusFromShortcut(conflict, 'P')
	if !ok || focus != 6 {
		t.Fatalf("shortcut P = %d %v, want focus 6", focus, ok)
	}
}
