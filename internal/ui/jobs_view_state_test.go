package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

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
