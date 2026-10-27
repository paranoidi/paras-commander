package ui

import (
	"path"
	"strings"

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

// jobSourceStatus is the job status string shown for source i of job e, from the phase the
// executor reported (JobEntry.SourcePhases): a started source carries the job's status (once
// the job completed, its own outcome), a finished one is failed when it has failed records,
// else completed. An unreached source is queued while the job is active; once the job is over
// it is completed (or failed) when the job completed or it has records, else canceled.
func jobSourceStatus(e JobEntry, i int) string {
	st := jobs.Status(e.Status)
	phase := jobs.SourceQueued
	if i >= 0 && i < len(e.SourcePhases) {
		phase = e.SourcePhases[i]
	}
	var stats oplog.Stats
	if i >= 0 && i < len(e.SourceStats) {
		stats = e.SourceStats[i]
	}
	outcome := string(jobs.StatusCompleted)
	if stats.Failed > 0 {
		outcome = string(jobs.StatusFailed)
	}
	switch {
	case phase == jobs.SourceStarted && st != jobs.StatusCompleted:
		return e.Status
	case phase != jobs.SourceQueued:
		return outcome
	case !st.IsFinished():
		return string(jobs.StatusQueued)
	case st == jobs.StatusCompleted || stats.Done+stats.Skipped+stats.Failed > 0:
		return outcome
	}
	return string(jobs.StatusCanceled)
}

// jobSourceActive reports whether source i is the one job e is working on now: started by the
// executor while the job is not finished. The Queue highlights that sub-row.
func jobSourceActive(e JobEntry, i int) bool {
	return i >= 0 && i < len(e.SourcePhases) && e.SourcePhases[i] == jobs.SourceStarted &&
		!jobs.Status(e.Status).IsFinished()
}

// jobSourceCurrentFile is job e's current path relative to source i, or "" when it is the
// source itself or lies outside it.
func jobSourceCurrentFile(e JobEntry, i int) string {
	if i < 0 || i >= len(e.Sources) {
		return ""
	}
	rel, _ := strings.CutPrefix(e.CurrentPath, strings.TrimSuffix(e.Sources[i], "/")+"/")
	if rel == e.CurrentPath {
		return ""
	}
	return rel
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
