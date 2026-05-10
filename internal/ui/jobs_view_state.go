package ui

import "github.com/paranoidi/paras-commander/internal/jobs"

// JobEntriesFromJobs converts domain jobs to the render DTO used by the jobs view.
func JobEntriesFromJobs(jobList []*jobs.Job) []JobEntry {
	entries := make([]JobEntry, 0, len(jobList))
	for _, j := range jobList {
		sources := append([]string(nil), j.Sources...)
		var pending *jobs.BlockerDetails
		if j.PendingBlocker != nil {
			b := *j.PendingBlocker
			pending = &b
		}
		entries = append(entries, JobEntry{
			ID:                j.ID,
			Type:              string(j.Type),
			Status:            string(j.Status),
			Sources:           sources,
			Destination:       j.Destination,
			CurrentPath:       j.CurrentPath,
			DoneFiles:         j.DoneFiles,
			TotalFiles:        j.TotalFiles,
			DoneBytes:         j.DoneBytes,
			TotalBytes:        j.TotalBytes,
			Error:             j.Error,
			StartedAt:         j.StartedAt,
			FinishedAt:        j.FinishedAt,
			ETABytesPerSec:    j.ETABytesPerSec,
			DisplaySpeedBPS:   j.DisplaySpeedBPS,
			ThroughputSamples: append([]jobs.ThroughputSample(nil), j.ThroughputSamples...),
			PendingBlocker: pending,
		})
	}
	return entries
}

// EnsureSelectionVisible clamps the selected job row and scroll offset.
func (s *JobsViewState) EnsureSelectionVisible(total int, visibleRows int) {
	if total == 0 {
		s.Selected = 0
		s.ListScroll = 0
		return
	}
	if s.Selected >= total {
		s.Selected = total - 1
	}
	if s.Selected < 0 {
		s.Selected = 0
	}
	if visibleRows <= 0 {
		return
	}
	if s.Selected < s.ListScroll {
		s.ListScroll = s.Selected
	}
	if s.Selected >= s.ListScroll+visibleRows {
		s.ListScroll = s.Selected - visibleRows + 1
	}
	maxScroll := max(0, total-visibleRows)
	if s.ListScroll > maxScroll {
		s.ListScroll = maxScroll
	}
	if s.ListScroll < 0 {
		s.ListScroll = 0
	}
}
