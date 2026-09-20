package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// JobEntriesFromJobs converts domain jobs to the render DTO used by the jobs view.
// When includeThroughputStrip is false, ThroughputStrip is left nil so progress does not copy strip memory for the UI.
// queueETAs maps job ID to cumulative queue ETA strings from jobs.ComputeQueueETAs.
func JobEntriesFromJobs(jobList []*jobs.Job, includeThroughputStrip bool, queueETAs map[string]string) []JobEntry {
	entries := make([]JobEntry, 0, len(jobList))
	for _, j := range jobList {
		if j == nil {
			continue
		}
		sources := pathloc.Strings(j.Sources)
		var pending *jobs.BlockerDetails
		if j.PendingBlocker != nil {
			b := *j.PendingBlocker
			pending = &b
		}
		var strip []float64
		if includeThroughputStrip && len(j.ThroughputStrip) > 0 {
			strip = append([]float64(nil), j.ThroughputStrip...)
		}
		queueETA := ""
		if queueETAs != nil {
			queueETA = queueETAs[j.ID]
		}
		entries = append(entries, JobEntry{
			ID:              j.ID,
			Type:            string(j.Type),
			Status:          string(j.Status),
			Sources:         sources,
			Destination:     j.Destination.String(),
			DestIsDir:       j.DestIsDir,
			CurrentPath:     j.CurrentPath,
			DoneFiles:       j.DoneFiles,
			TotalFiles:      j.TotalFiles,
			TotalDirs:       j.TotalDirs,
			DoneBytes:       j.DoneBytes,
			TotalBytes:      j.TotalBytes,
			Error:           j.Error,
			Warnings:        j.Warnings,
			StartedAt:       j.StartedAt,
			FinishedAt:      j.FinishedAt,
			ETABytesPerSec:  j.ETABytesPerSec,
			ETAFilesPerSec:  j.ETAFilesPerSec,
			DisplaySpeedBPS: j.DisplaySpeedBPS,
			QueueETA:        queueETA,
			ThroughputStrip: strip,
			PendingBlocker:  pending,
			TotalsComplete:  j.TotalsComplete,
		})
	}
	return entries
}

func ensureSelectionVisible(selected *int, listScroll *int, total int, visibleRows int) {
	if total == 0 {
		*selected = 0
		*listScroll = 0
		return
	}
	if *selected >= total {
		*selected = total - 1
	}
	if *selected < 0 {
		*selected = 0
	}
	if visibleRows <= 0 {
		return
	}
	if *selected < *listScroll {
		*listScroll = *selected
	}
	if *selected >= *listScroll+visibleRows {
		*listScroll = *selected - visibleRows + 1
	}
	maxScroll := max(0, total-visibleRows)
	if *listScroll > maxScroll {
		*listScroll = maxScroll
	}
	if *listScroll < 0 {
		*listScroll = 0
	}
}

// EnsureSelectionVisible clamps the selected job row and scroll offset.
func (s *JobsViewState) EnsureSelectionVisible(total int, visibleRows int) {
	ensureSelectionVisible(&s.Selected, &s.ListScroll, total, visibleRows)
}

// EnsureSelectionVisible clamps the selected compare row and scroll offset.
func (s *CompareViewState) EnsureSelectionVisible(total int, visibleRows int) {
	ensureSelectionVisible(&s.Selected, &s.ListScroll, total, visibleRows)
}
