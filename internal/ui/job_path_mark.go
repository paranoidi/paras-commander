package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
)

// JobPathMark is the minimal job snapshot needed for file-list trailing glyphs in the browser.
type JobPathMark struct {
	ID          string
	Type        string
	Status      string
	Sources     []string
	Destination string
	DestIsDir   bool
}

// JobPathMarksFromJobs builds path marks from domain jobs (finished jobs included for stable ordering).
func JobPathMarksFromJobs(jobList []*jobs.Job) []JobPathMark {
	marks := make([]JobPathMark, 0, len(jobList))
	for _, j := range jobList {
		if j == nil {
			continue
		}
		marks = append(marks, JobPathMark{
			ID:          j.ID,
			Type:        string(j.Type),
			Status:      string(j.Status),
			Sources:     append([]string(nil), j.Sources...),
			Destination: j.Destination,
			DestIsDir:   j.DestIsDir,
		})
	}
	return marks
}

// JobPathMarksFromEntries converts render DTOs to path marks (tests and legacy callers).
func JobPathMarksFromEntries(entries []JobEntry) []JobPathMark {
	marks := make([]JobPathMark, 0, len(entries))
	for _, e := range entries {
		marks = append(marks, JobPathMark{
			ID:          e.ID,
			Type:        e.Type,
			Status:      e.Status,
			Sources:     append([]string(nil), e.Sources...),
			Destination: e.Destination,
			DestIsDir:   e.DestIsDir,
		})
	}
	return marks
}
