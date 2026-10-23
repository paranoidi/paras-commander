package ui

import (
	"sync/atomic"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
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
			Undo:            j.UndoLabel(),
			FailedItems:     j.FailedItems,
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

// listScrollPolicy is the [ui.scroll] mode and edge margin shared by the Jobs, Messages,
// Compare and Dedup list views; the app plumbs it once via SetListScroll.
type listScrollPolicy struct {
	mode   panel.ScrollMode
	margin int
}

var listScroll atomic.Pointer[listScrollPolicy]

// SetListScroll sets the scroll mode and edge margin the list views honor.
func SetListScroll(mode panel.ScrollMode, margin int) {
	listScroll.Store(&listScrollPolicy{mode: panel.EffectiveScrollMode(mode), margin: margin})
}

// ensureSelectionVisible clamps selected and scrolls per the configured scroll mode.
// visibleRows <= 0 only clamps.
func ensureSelectionVisible(selected *int, listScrollPos *int, total int, visibleRows int) {
	if total == 0 {
		*selected = 0
		*listScrollPos = 0
		return
	}
	*selected = max(0, min(*selected, total-1))
	if visibleRows <= 0 {
		return
	}
	pol := listScroll.Load()
	if pol == nil {
		pol = &listScrollPolicy{mode: panel.ScrollModeEdge}
	}
	switch pol.mode {
	case panel.ScrollModeCenter:
		*listScrollPos = geom.ScrollOffset(*selected, visibleRows, total)
	case panel.ScrollModeEdge:
		*listScrollPos = geom.ScrollOffsetEdge(*selected, *listScrollPos, visibleRows, total, pol.margin)
	default:
		if *selected < *listScrollPos {
			*listScrollPos = *selected
		}
		if *selected >= *listScrollPos+visibleRows {
			*listScrollPos = *selected - visibleRows + 1
		}
		*listScrollPos = max(0, min(*listScrollPos, max(0, total-visibleRows)))
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
