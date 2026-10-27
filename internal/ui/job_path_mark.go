package ui

import (
	"slices"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// JobPathMark is the minimal job snapshot needed for file-list trailing icons in the browser.
type JobPathMark struct {
	ID          string
	Type        string
	Status      string
	Sources     []string
	Destination string
	DestIsDir   bool
	// SourcePhases is the executor's phase of each source (jobs.Job.SourcePhases; nil for a
	// single-source job) and ActiveSource the source it started last (-1 before any). While the
	// job is active and a source has started they narrow its marks to that source (rootPhase).
	// SetJobPathMarkSources refreshes both in place when the active source moves.
	SourcePhases []jobs.SourcePhase
	ActiveSource int

	// idx is the ancestor lookup index built by the constructors below (see jobPathIndex).
	// Marks built as bare literals leave it nil and fall back to building one per call.
	idx *jobPathIndex
}

// index returns the mark's prebuilt path index, building one on demand for literal-constructed
// marks (tests) so matching has a single implementation.
func (j JobPathMark) index() *jobPathIndex {
	if j.idx != nil {
		return j.idx
	}
	return newJobPathIndex(j)
}

// withIndex attaches the ancestor index so per-row matching never rescans Sources.
func withIndex(m JobPathMark) JobPathMark {
	m.idx = newJobPathIndex(m)
	return m
}

// JobPathMarksFromJobs builds path marks from domain jobs (finished jobs included for stable ordering).
func JobPathMarksFromJobs(jobList []*jobs.Job) []JobPathMark {
	marks := make([]JobPathMark, 0, len(jobList))
	for _, j := range jobList {
		if j == nil {
			continue
		}
		marks = append(marks, withIndex(JobPathMark{
			ID:           j.ID,
			Type:         string(j.Type),
			Status:       string(j.Status),
			Sources:      pathloc.Strings(j.Sources),
			Destination:  j.Destination.String(),
			DestIsDir:    j.DestIsDir,
			SourcePhases: j.SourcePhases(),
			ActiveSource: j.ActiveSource(),
		}))
	}
	return marks
}

// JobPathMarksFromEntries converts render DTOs to path marks (tests and legacy callers).
func JobPathMarksFromEntries(entries []JobEntry) []JobPathMark {
	marks := make([]JobPathMark, 0, len(entries))
	for _, e := range entries {
		marks = append(marks, withIndex(JobPathMark{
			ID:           e.ID,
			Type:         e.Type,
			Status:       e.Status,
			Sources:      append([]string(nil), e.Sources...),
			Destination:  e.Destination,
			DestIsDir:    e.DestIsDir,
			SourcePhases: e.SourcePhases,
			ActiveSource: slices.Index(e.SourcePhases, jobs.SourceStarted),
		}))
	}
	return marks
}

// rootPhase is how the filelist marks one source's roots: rootActive (the job icon, as for a
// job working on every source), rootPending (a source not started yet: the queued icon on its
// source rows, nothing on its destination) or rootDone (no mark).
type rootPhase uint8

const (
	rootActive rootPhase = iota
	rootPending
	rootDone
)

// rootPhase is the phase of source src's roots. A nil mark, roots outside every source
// (src < 0), jobs that are not active, single-source jobs and jobs before their first source
// started mark every root active.
func (j *JobPathMark) rootPhase(src int32) rootPhase {
	if j == nil || src < 0 || j.ActiveSource < 0 || int(src) >= len(j.SourcePhases) || !jobs.Status(j.Status).IsActive() {
		return rootActive
	}
	switch j.SourcePhases[src] {
	case jobs.SourceQueued:
		return rootPending
	case jobs.SourceFinished:
		return rootDone
	}
	return rootActive
}

// SetJobPathMarkSources sets the active source and source phases of job id's mark in place,
// keeping its path index, and reports whether the active source changed. It runs on progress
// events, so a multi-source job's marks follow the executor without rebuilding the index.
func SetJobPathMarkSources(marks []JobPathMark, id string, active int, phases func() []jobs.SourcePhase) bool {
	for i := range marks {
		if marks[i].ID != id {
			continue
		}
		if marks[i].ActiveSource == active {
			return false
		}
		marks[i].ActiveSource, marks[i].SourcePhases = active, phases()
		return true
	}
	return false
}
