package ui

import (
	"path"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/oplog"
)

// JobsListRow is one Queue row: a job (Source oplog.NoSource) or, under an expanded job, one
// of its sources.
type JobsListRow struct {
	Job    int
	Source int
}

// JobEntryHasSources reports whether a job can expand into source sub-rows (more than one
// source).
func JobEntryHasSources(e JobEntry) bool { return len(e.Sources) > 1 }

// JobExpanded reports whether job e shows its source sub-rows.
func (s JobsViewState) JobExpanded(e JobEntry) bool {
	return JobEntryHasSources(e) && s.Expanded[e.ID]
}

// JobsListRows flattens jobs into Queue rows: each job, followed by its sources when expanded.
// It is the one row model shared by the list render, scrolling, mouse hits and cursor moves.
func (s JobsViewState) JobsListRows(jobList []JobEntry) []JobsListRow {
	rows := make([]JobsListRow, 0, len(jobList))
	for i, e := range jobList {
		rows = append(rows, JobsListRow{Job: i, Source: oplog.NoSource})
		if s.JobExpanded(e) {
			for src := range e.Sources {
				rows = append(rows, JobsListRow{Job: i, Source: src})
			}
		}
	}
	return rows
}

// SelectedSource is the source index of the selected sub-row, or oplog.NoSource on a job row
// (or when the selected job is not expanded).
func (s JobsViewState) SelectedSource(jobList []JobEntry) int {
	if s.SelectedSub <= 0 || s.Selected < 0 || s.Selected >= len(jobList) {
		return oplog.NoSource
	}
	e := jobList[s.Selected]
	if !s.JobExpanded(e) || s.SelectedSub > len(e.Sources) {
		return oplog.NoSource
	}
	return s.SelectedSub - 1
}

// SelectedRow is the index in rows of the selected row: the selected sub-row, else its job's
// row, else the last row when the selection is past the end.
func (s JobsViewState) SelectedRow(rows []JobsListRow) int {
	jobRow := -1
	for i, r := range rows {
		if r.Job != s.Selected {
			continue
		}
		if r.Source == s.SelectedSub-1 {
			return i
		}
		if jobRow < 0 {
			jobRow = i
		}
	}
	if jobRow >= 0 {
		return jobRow
	}
	if n := len(rows); n > 0 && s.Selected > rows[n-1].Job {
		return n - 1
	}
	return 0
}

// SelectRow moves the selection to row r.
func (s *JobsViewState) SelectRow(r JobsListRow) {
	s.Selected, s.SelectedSub = r.Job, r.Source+1
}

// jobSourceStatus is the job status string shown for source i of job e. Sources are processed
// in order, so a source below the newest attributed record's source (or one with records) is
// finished: failed when it has failures, else completed. The newest record's source carries
// the active job's status; the rest are queued, or, once the job finished, canceled when the
// job did not complete.
func jobSourceStatus(e JobEntry, i int) string {
	st := jobs.Status(e.Status)
	if st == jobs.StatusScanning || st == jobs.StatusQueued {
		return string(jobs.StatusQueued)
	}
	var stats oplog.Stats
	if i >= 0 && i < len(e.SourceStats) {
		stats = e.SourceStats[i]
	}
	finished := string(jobs.StatusCompleted)
	if stats.Failed > 0 {
		finished = string(jobs.StatusFailed)
	}
	if !st.IsFinished() && i == max(e.LastSource, 0) {
		return e.Status
	}
	if i < e.LastSource || stats.Done+stats.Skipped+stats.Failed > 0 {
		return finished
	}
	switch st {
	case jobs.StatusCompleted:
		return finished
	case jobs.StatusCanceled, jobs.StatusFailed:
		return string(jobs.StatusCanceled)
	}
	return string(jobs.StatusQueued)
}

// jobSourceLabels are the Queue sub-row labels of sources: each base name, or the whole path
// when another source shares that base name.
func jobSourceLabels(sources []string) []string {
	bases := make([]string, len(sources))
	count := make(map[string]int, len(sources))
	for i, src := range sources {
		bases[i] = path.Base(src)
		count[bases[i]]++
	}
	for i, base := range bases {
		if count[base] > 1 {
			bases[i] = sources[i]
		}
	}
	return bases
}
