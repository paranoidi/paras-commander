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

func TestConflictDecisionFromButtonIndex(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		idx      int
		mismatch bool
		want     jobs.ConflictDecision
	}{
		{0, false, jobs.DecisionOverwrite},
		{1, false, jobs.DecisionOverwriteAll},
		{2, false, ""}, // Advanced… submits no decision
		{3, false, jobs.DecisionSkip},
		{4, false, jobs.DecisionSkipAll},
		{5, false, jobs.DecisionCancel},
		{0, true, jobs.DecisionRename},
		{1, true, jobs.DecisionRenameAll},
		{3, true, jobs.DecisionSkip},
		{5, true, jobs.DecisionCancel},
	} {
		if got := ConflictDecisionFromButtonIndex(c.idx, c.mismatch); got != c.want {
			t.Errorf("ConflictDecisionFromButtonIndex(%d, %v) = %q, want %q", c.idx, c.mismatch, got, c.want)
		}
	}
}

func TestJobBlockerDecisionFromFocus(t *testing.T) {
	t.Parallel()
	conflict := JobEntry{PendingBlocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}}
	if d := JobBlockerDecisionFromFocus(conflict, 0); d != jobs.DecisionOverwrite {
		t.Fatalf("focus 0 = %q, want overwrite", d)
	}
	if !JobBlockerPanelIsAdvanced(conflict, 2) || JobBlockerPanelIsAdvanced(conflict, 0) {
		t.Fatal("expected focus 2 to be Advanced")
	}
	disk := JobEntry{PendingBlocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindDiskSpace, DiskSpace: &jobs.DiskSpaceBlockerDetails{}}}
	if d := JobBlockerDecisionFromFocus(disk, 0); d != jobs.DecisionRetry {
		t.Fatalf("disk retry = %q", d)
	}
	if d := JobBlockerDecisionFromFocus(disk, 1); d != jobs.DecisionCancel {
		t.Fatalf("disk abort = %q", d)
	}
	mismatch := JobEntry{PendingBlocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Mismatch: true, DestIsDir: true}}}
	if d := JobBlockerDecisionFromFocus(mismatch, 0); d != jobs.DecisionRename {
		t.Fatalf("mismatch focus 0 = %q, want rename", d)
	}
	if JobBlockerPanelIsAdvanced(mismatch, 2) {
		t.Fatal("Advanced must not be offered on a mismatch")
	}
}

func TestJobsBlockerPanelMoveFocus(t *testing.T) {
	t.Parallel()
	conflict := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}

	// Jobs panel: [Overwrite, Overwrite All, Advanced…] [Skip, Skip All, Cancel]
	cases := []struct {
		name string
		from int
		key  tcell.Key
		want int
	}{
		{"Right from Advanced wraps to Skip", 2, tcell.KeyRight, 3},
		{"Left from Skip back to Advanced", 3, tcell.KeyLeft, 2},
		{"Right from Cancel stays", 5, tcell.KeyRight, 5},
		{"Left from Overwrite stays", 0, tcell.KeyLeft, 0},
		{"Down from Advanced to Cancel", 2, tcell.KeyDown, 5},
		{"Up from Cancel to Advanced", 5, tcell.KeyUp, 2},
		{"Up from Overwrite stays", 0, tcell.KeyUp, 0},
		{"Tab from Cancel wraps", 5, tcell.KeyTab, 0},
		{"Backtab from Overwrite wraps", 0, tcell.KeyBacktab, 5},
	}
	for _, tc := range cases {
		got, ok := JobsBlockerPanelMoveFocus(conflict, tc.from, tc.key)
		if !ok || got != tc.want {
			t.Fatalf("%s: got %d %v, want %d", tc.name, got, ok, tc.want)
		}
	}

	mismatch := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Mismatch: true, DestIsDir: true}}
	if got, _ := JobsBlockerPanelMoveFocus(mismatch, 1, tcell.KeyRight); got != 3 {
		t.Fatalf("Right from Rename All = %d, want 3 (Advanced is skipped)", got)
	}

	disk := jobs.BlockerDetails{Kind: jobs.BlockerKindDiskSpace, DiskSpace: &jobs.DiskSpaceBlockerDetails{}}
	for _, tc := range []struct {
		from int
		key  tcell.Key
		want int
	}{
		{0, tcell.KeyRight, 1}, {2, tcell.KeyRight, 2}, {1, tcell.KeyLeft, 0}, {0, tcell.KeyLeft, 0},
		{2, tcell.KeyTab, 0}, {0, tcell.KeyBacktab, 2}, {1, tcell.KeyDown, 1}, {1, tcell.KeyUp, 1},
	} {
		if got, ok := JobsBlockerPanelMoveFocus(disk, tc.from, tc.key); !ok || got != tc.want {
			t.Fatalf("disk %v from %d = %d %v, want %d", tc.key, tc.from, got, ok, tc.want)
		}
	}
	if _, ok := JobsBlockerPanelMoveFocus(disk, 0, tcell.KeyEnter); ok {
		t.Fatal("Enter must not be consumed by the disk pane navigation")
	}
}
