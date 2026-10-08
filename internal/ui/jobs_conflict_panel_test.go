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
	if d, ok := JobBlockerDialogDecision(conflict, 2); !ok || d != jobs.DecisionOverwriteAllSameSize {
		t.Fatalf("focus 2 = %q %v, want overwrite-all-same-size", d, ok)
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

	// Rows: [Overwrite 0, Overwrite All 1, Match Size 2] [Skip 3, Skip All 4, Postpone 6] [Cancel 5]
	cases := []struct {
		name string
		from int
		key  tcell.Key
		want int
	}{
		{"Right from Match Size wraps to Skip", 2, tcell.KeyRight, 3},
		{"Left from Skip back to Match Size", 3, tcell.KeyLeft, 2},
		{"Right from Postpone to Cancel", 6, tcell.KeyRight, 5},
		{"Right from Cancel stays", 5, tcell.KeyRight, 5},
		{"Left from Overwrite stays", 0, tcell.KeyLeft, 0},
		{"Down from Match Size to Postpone", 2, tcell.KeyDown, 6},
		{"Down from Skip All to Cancel", 4, tcell.KeyDown, 5},
		{"Up from Cancel to Skip All", 5, tcell.KeyUp, 4},
		{"Up from Overwrite stays", 0, tcell.KeyUp, 0},
		{"Tab from Cancel wraps", 5, tcell.KeyTab, 0},
		{"Backtab from Overwrite wraps", 0, tcell.KeyBacktab, 5},
	}
	for _, tc := range cases {
		got, ok := JobBlockerDialogMoveFocus(conflict, tc.from, tc.key)
		if !ok || got != tc.want {
			t.Fatalf("%s: got %d %v, want %d", tc.name, got, ok, tc.want)
		}
	}

	// Jobs panel: [Overwrite, Overwrite All, Match Size] [Skip, Skip All, Cancel]
	if got, _ := JobsBlockerPanelMoveFocus(conflict, 2, tcell.KeyDown); got != 5 {
		t.Fatalf("panel Down from Match Size = %d, want 5", got)
	}
	if got, _ := JobsBlockerPanelMoveFocus(conflict, 5, tcell.KeyUp); got != 2 {
		t.Fatalf("panel Up from Cancel = %d, want 2", got)
	}
}

func TestJobBlockerDialogFocusFromShortcutMatchSize(t *testing.T) {
	t.Parallel()
	conflict := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}
	focus, ok := JobBlockerDialogFocusFromShortcut(conflict, 'm')
	if !ok || focus != 2 {
		t.Fatalf("shortcut m = %d %v, want focus 2", focus, ok)
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
