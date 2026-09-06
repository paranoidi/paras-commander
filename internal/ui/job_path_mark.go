package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// JobPathMark is the minimal job snapshot needed for file-list trailing glyphs in the browser.
type JobPathMark struct {
	ID          string
	Type        string
	Status      string
	Sources     []string
	Destination string
	DestIsDir   bool

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
			ID:          j.ID,
			Type:        string(j.Type),
			Status:      string(j.Status),
			Sources:     pathloc.Strings(j.Sources),
			Destination: j.Destination.String(),
			DestIsDir:   j.DestIsDir,
		}))
	}
	return marks
}

// JobPathMarksFromEntries converts render DTOs to path marks (tests and legacy callers).
func JobPathMarksFromEntries(entries []JobEntry) []JobPathMark {
	marks := make([]JobPathMark, 0, len(entries))
	for _, e := range entries {
		marks = append(marks, withIndex(JobPathMark{
			ID:          e.ID,
			Type:        e.Type,
			Status:      e.Status,
			Sources:     append([]string(nil), e.Sources...),
			Destination: e.Destination,
			DestIsDir:   e.DestIsDir,
		}))
	}
	return marks
}
