package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func sourcesJobs() []JobEntry {
	return []JobEntry{
		{ID: "harbor", Type: "copy", Status: string(jobs.StatusRunning), Sources: []string{"/meadow/lantern", "/meadow/willow.txt", "/meadow/copper"}},
		{ID: "quarry", Type: "delete", Status: string(jobs.StatusQueued), Sources: []string{"/meadow/thistle"}},
	}
}

func TestJobsListRowsAndSelection(t *testing.T) {
	list := sourcesJobs()
	st := JobsViewState{}
	if rows := st.JobsListRows(list); len(rows) != 2 {
		t.Fatalf("collapsed rows = %v", rows)
	}
	st.Expanded = map[string]bool{"harbor": true, "quarry": true} // quarry has one source: no sub-rows
	rows := st.JobsListRows(list)
	if fmt.Sprint(rows) != "[{0 -1} {0 0} {0 1} {0 2} {1 -1}]" {
		t.Fatalf("expanded rows = %v", rows)
	}
	st.SelectRow(rows[2])
	if st.SelectedRow(rows) != 2 || st.SelectedSource(list) != 1 {
		t.Fatalf("row %d source %d, want 2 and 1", st.SelectedRow(rows), st.SelectedSource(list))
	}
	st.Expanded = nil // collapsed: the cursor falls back to its job row and filters nothing
	if st.SelectedRow(st.JobsListRows(list)) != 0 || st.SelectedSource(list) != oplog.NoSource {
		t.Fatal("a collapsed job's sub-row selection must resolve to the job row")
	}
	st.EnsureSelectionVisible(list, 10)
	if st.SelectedSub != 0 {
		t.Fatalf("SelectedSub = %d after clamping a collapsed job", st.SelectedSub)
	}
}

func TestJobSourceStatus(t *testing.T) {
	stats := []oplog.Stats{{Done: 4}, {Done: 1, Failed: 1}, {}}
	tests := []struct {
		status string
		last   int
		want   []jobs.Status
	}{
		{string(jobs.StatusQueued), oplog.NoSource, []jobs.Status{jobs.StatusQueued, jobs.StatusQueued, jobs.StatusQueued}},
		{string(jobs.StatusRunning), oplog.NoSource, []jobs.Status{jobs.StatusRunning, jobs.StatusFailed, jobs.StatusQueued}},
		{string(jobs.StatusRunning), 1, []jobs.Status{jobs.StatusCompleted, jobs.StatusRunning, jobs.StatusQueued}},
		{string(jobs.StatusPaused), 2, []jobs.Status{jobs.StatusCompleted, jobs.StatusFailed, jobs.StatusPaused}},
		{string(jobs.StatusCompleted), 1, []jobs.Status{jobs.StatusCompleted, jobs.StatusFailed, jobs.StatusCompleted}},
		{string(jobs.StatusCanceled), 1, []jobs.Status{jobs.StatusCompleted, jobs.StatusFailed, jobs.StatusCanceled}},
	}
	for _, tt := range tests {
		e := JobEntry{Status: tt.status, Sources: make([]string, 3), SourceStats: stats, LastSource: tt.last}
		for i, want := range tt.want {
			if got := jobSourceStatus(e, i); got != string(want) {
				t.Errorf("%s last=%d source %d = %s, want %s", tt.status, tt.last, i, got, want)
			}
		}
	}
}

func TestJobSourceLabelDisambiguatesSharedBaseNames(t *testing.T) {
	sources := []string{"/meadow/lantern", "/harbor/lantern", "/harbor/copper"}
	labels := jobSourceLabels(sources)
	if labels[0] != "/meadow/lantern" || labels[1] != "/harbor/lantern" {
		t.Fatalf("shared base name = %q", labels[:2])
	}
	if labels[2] != "copper" {
		t.Fatalf("unique base name = %q", labels[2])
	}
}

func TestDrawJobsListPanelExpandedSources(t *testing.T) {
	screen := newOpsScreen(t)
	screen.SetSize(90, 10)
	rect := Rect{X: 0, Y: 0, Width: 90, Height: 9}
	list := sourcesJobs()
	list[0].SourceStats = []oplog.Stats{{Done: 3}, {Done: 1, Failed: 2}, {}}
	list[0].LastSource = 1
	styles := theme.Default()
	st := JobsViewState{Expanded: map[string]bool{"harbor": true}}
	drawJobsListPanel(screen, rect, st, list, styles, time.Now(), false, 0)

	jobRow := screenRowText(screen, rect, 2)
	if !strings.Contains(jobRow, "copy "+styles.IconJobsListSources(true)) {
		t.Fatalf("job row = %q, want the expanded icon after the type", jobRow)
	}
	wants := []struct{ connector, status, label, counts string }{
		{styles.IconTreeBranch(), styles.IconJobsList("completed"), "lantern", "3 done"},
		{styles.IconTreeBranch(), styles.IconJobsList("running"), "willow.txt", "1 done, 2 failed"},
		{styles.IconTreeEnd(), styles.IconJobsList("queued"), "copper", "0 done"},
	}
	for i, w := range wants {
		row := screenRowText(screen, rect, 3+i)
		for _, part := range []string{w.connector, w.status + " " + w.label, w.counts} {
			if !strings.Contains(row, part) {
				t.Fatalf("source row %d = %q, want %q", i, row, part)
			}
		}
	}
	if next := screenRowText(screen, rect, 6); !strings.Contains(next, "delete") || strings.Contains(next, styles.IconJobsListSources(false)) {
		t.Fatalf("single-source job row = %q, want no expander", next)
	}
}
