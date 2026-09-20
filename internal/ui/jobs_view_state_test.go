package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestEnsureSelectionVisibleTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		selected    int
		listScroll  int
		total       int
		visibleRows int
		wantSel     int
		wantScroll  int
	}{
		{name: "empty", selected: 3, listScroll: 2, total: 0, visibleRows: 4, wantSel: 0, wantScroll: 0},
		{name: "negative selected", selected: -4, listScroll: 0, total: 10, visibleRows: 4, wantSel: 0, wantScroll: 0},
		{name: "oversized selected", selected: 99, listScroll: 0, total: 10, visibleRows: 4, wantSel: 9, wantScroll: 6},
		{name: "zero visible keeps selection", selected: 7, listScroll: 2, total: 10, visibleRows: 0, wantSel: 7, wantScroll: 2},
		{name: "negative visible keeps selection", selected: 7, listScroll: 2, total: 10, visibleRows: -1, wantSel: 7, wantScroll: 2},
		{name: "page edge first page", selected: 3, listScroll: 0, total: 10, visibleRows: 4, wantSel: 3, wantScroll: 0},
		{name: "page edge scrolls down", selected: 4, listScroll: 0, total: 10, visibleRows: 4, wantSel: 4, wantScroll: 1},
		{name: "selection above viewport", selected: 1, listScroll: 5, total: 10, visibleRows: 4, wantSel: 1, wantScroll: 1},
		{name: "max scroll clamp", selected: 9, listScroll: 100, total: 10, visibleRows: 4, wantSel: 9, wantScroll: 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs := JobsViewState{Selected: tt.selected, ListScroll: tt.listScroll}
			jobs.EnsureSelectionVisible(tt.total, tt.visibleRows)
			if jobs.Selected != tt.wantSel || jobs.ListScroll != tt.wantScroll {
				t.Fatalf("jobs Selected=%d ListScroll=%d, want %d %d", jobs.Selected, jobs.ListScroll, tt.wantSel, tt.wantScroll)
			}
			msgs := MessagesViewState{Selected: tt.selected, ListScroll: tt.listScroll}
			msgs.EnsureSelectionVisible(tt.total, tt.visibleRows)
			if msgs.Selected != tt.wantSel || msgs.ListScroll != tt.wantScroll {
				t.Fatalf("messages Selected=%d ListScroll=%d, want %d %d", msgs.Selected, msgs.ListScroll, tt.wantSel, tt.wantScroll)
			}
			cmds := CommandsViewState{Selected: tt.selected, ListScroll: tt.listScroll}
			cmds.EnsureSelectionVisible(tt.total, tt.visibleRows)
			if cmds.Selected != tt.wantSel || cmds.ListScroll != tt.wantScroll {
				t.Fatalf("commands Selected=%d ListScroll=%d, want %d %d", cmds.Selected, cmds.ListScroll, tt.wantSel, tt.wantScroll)
			}
			dedup := DedupPane{Selected: tt.selected, ListScroll: tt.listScroll}
			dedup.EnsureSelectionVisible(tt.total, tt.visibleRows)
			if dedup.Selected != tt.wantSel || dedup.ListScroll != tt.wantScroll {
				t.Fatalf("dedup Selected=%d ListScroll=%d, want %d %d", dedup.Selected, dedup.ListScroll, tt.wantSel, tt.wantScroll)
			}
			cmp := CompareViewState{Selected: tt.selected, ListScroll: tt.listScroll}
			cmp.EnsureSelectionVisible(tt.total, tt.visibleRows)
			if cmp.Selected != tt.wantSel || cmp.ListScroll != tt.wantScroll {
				t.Fatalf("compare Selected=%d ListScroll=%d, want %d %d", cmp.Selected, cmp.ListScroll, tt.wantSel, tt.wantScroll)
			}
		})
	}
}

func TestJobsViewStateEnsureSelectionVisible(t *testing.T) {
	state := JobsViewState{Selected: 9, ListScroll: 0}

	state.EnsureSelectionVisible(10, 4)

	if state.Selected != 9 {
		t.Fatalf("Selected = %d, want 9", state.Selected)
	}
	if state.ListScroll != 6 {
		t.Fatalf("ListScroll = %d, want 6", state.ListScroll)
	}
}

func TestJobsViewStateEnsureSelectionVisibleEmpty(t *testing.T) {
	state := JobsViewState{Selected: 3, ListScroll: 2}

	state.EnsureSelectionVisible(0, 4)

	if state.Selected != 0 || state.ListScroll != 0 {
		t.Fatalf("state = %+v, want zero selection and scroll", state)
	}
}

func TestJobEntriesFromJobsOmitsThroughputStripWhenDisabled(t *testing.T) {
	t.Parallel()
	j := &jobs.Job{
		ID:              "1",
		Type:            jobs.TypeCopy,
		Status:          jobs.StatusRunning,
		Sources:         pathloc.PathsForTest("/a"),
		Destination:     pathloc.MustParse("/b"),
		ThroughputStrip: []float64{1, 2, 3},
	}
	got := JobEntriesFromJobs([]*jobs.Job{j}, false, nil)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].ThroughputStrip != nil {
		t.Fatalf("ThroughputStrip = %v, want nil", got[0].ThroughputStrip)
	}
}

func TestJobEntriesFromJobsOneEntryPerJobWithMultipleSources(t *testing.T) {
	t.Parallel()
	j := &jobs.Job{
		ID:          "j1",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/a/1", "/a/2", "/a/3"),
		Destination: pathloc.MustParse("/dst"),
		TotalFiles:  3,
	}
	got := JobEntriesFromJobs([]*jobs.Job{j}, false, nil)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 job row for a multi-source transfer", len(got))
	}
	if len(got[0].Sources) != len(j.Sources) {
		t.Fatalf("Sources = %v", got[0].Sources)
	}
}
