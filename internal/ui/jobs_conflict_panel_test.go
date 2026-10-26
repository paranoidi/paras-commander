package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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

func TestJobBlockerDecisionFromFocusButtonOrder(t *testing.T) {
	t.Parallel()
	entry := func(c jobs.ConflictEvent) JobEntry {
		return JobEntry{PendingBlocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &c}}
	}
	for _, c := range []struct {
		name     string
		conflict jobs.ConflictEvent
		want     []jobs.ConflictDecision
	}{
		{"copy", jobs.ConflictEvent{}, []jobs.ConflictDecision{
			jobs.DecisionOverwrite, jobs.DecisionOverwriteAll, "", jobs.DecisionSkip, jobs.DecisionSkipAll, jobs.DecisionCancel}},
		{"move", jobs.ConflictEvent{Move: true}, []jobs.ConflictDecision{
			jobs.DecisionOverwrite, jobs.DecisionOverwriteAll, "", jobs.DecisionSkip, jobs.DecisionSkipAll, jobs.DecisionDeleteSource, jobs.DecisionCancel}},
		{"mismatch", jobs.ConflictEvent{Mismatch: true}, []jobs.ConflictDecision{
			jobs.DecisionRename, jobs.DecisionRenameAll, jobs.DecisionSkip, jobs.DecisionSkipAll, jobs.DecisionCancel}},
		{"move mismatch", jobs.ConflictEvent{Move: true, Mismatch: true}, []jobs.ConflictDecision{
			jobs.DecisionRename, jobs.DecisionRenameAll, jobs.DecisionSkip, jobs.DecisionSkipAll, jobs.DecisionCancel}},
	} {
		e := entry(c.conflict)
		if got := JobsBlockerMaxButtonIndex(e); got != len(c.want)-1 {
			t.Errorf("%s: max index = %d, want %d", c.name, got, len(c.want)-1)
		}
		for idx, want := range c.want {
			if got := JobBlockerDecisionFromFocus(e, idx); got != want {
				t.Errorf("%s: focus %d = %q, want %q", c.name, idx, got, want)
			}
		}
	}
}

func TestJobBlockerPanelMnemonic(t *testing.T) {
	t.Parallel()
	move := JobEntry{PendingBlocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Move: true}}}
	if id, ok := JobBlockerPanelMnemonic(move, 'e'); !ok || id != dialog.ConflictDeleteSource {
		t.Fatalf("move Alt+E = %q %v, want Delete source", id, ok)
	}
	if _, ok := JobBlockerPanelMnemonic(move, 'p'); ok {
		t.Fatal("Postpone is not offered in the panel")
	}
	copyEntry := JobEntry{PendingBlocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}}
	if _, ok := JobBlockerPanelMnemonic(copyEntry, 'e'); ok {
		t.Fatal("a copy has no Delete source")
	}
	if id, ok := JobBlockerPanelMnemonic(copyEntry, 'D'); !ok || id != dialog.ConflictAdvanced {
		t.Fatalf("Alt+D = %q %v, want Advanced", id, ok)
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
	for i := range 5 {
		if JobBlockerPanelIsAdvanced(mismatch, i) {
			t.Fatal("Advanced must not be offered on a mismatch")
		}
	}
}

func TestJobsBlockerPanelMoveFocus(t *testing.T) {
	t.Parallel()
	conflict := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}

	// Jobs panel: [Overwrite, Overwrite All, Advanced…] [Skip, Skip All] [Cancel]
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
		{"Down from Advanced to Skip All", 2, tcell.KeyDown, 4},
		{"Down from Skip All to Cancel", 4, tcell.KeyDown, 5},
		{"Up from Cancel to Skip All", 5, tcell.KeyUp, 4},
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
	if got, _ := JobsBlockerPanelMoveFocus(mismatch, 1, tcell.KeyRight); got != 2 {
		t.Fatalf("Right from Rename All = %d, want 2 (Skip; no Advanced)", got)
	}
	move := jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Move: true}}
	if got, _ := JobsBlockerPanelMoveFocus(move, 4, tcell.KeyDown); got != 6 {
		t.Fatalf("Down from Skip All = %d, want 6 (Cancel next to Delete source)", got)
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
