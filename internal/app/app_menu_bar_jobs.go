package app

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) menuBarJobsStripSnapshot() ui.MenuBarJobsStrip {
	var strip ui.MenuBarJobsStrip
	strip.QueueStatuses = a.jobState.MenuBarStripStatuses()
	all := a.jobState.AllJobs()
	var prog *jobs.Job
	for _, j := range all {
		if j == nil {
			continue
		}
		if j.Status == jobs.StatusRunning {
			prog = j
			break
		}
	}
	if prog == nil {
		for _, j := range all {
			if j == nil {
				continue
			}
			if j.Status == jobs.StatusWaitingDecision && jobHasProgressTotals(j) {
				prog = j
				break
			}
		}
	}
	if prog == nil {
		for _, j := range all {
			if j == nil {
				continue
			}
			if j.Status == jobs.StatusPaused && jobHasProgressTotals(j) {
				prog = j
				break
			}
		}
	}
	if prog != nil {
		if f, ok := jobProgressFraction(prog); ok {
			strip.ProgressFrac = f
			strip.HasProgress = true
		}
	}
	return strip
}

func jobHasProgressTotals(j *jobs.Job) bool {
	return j != nil && (j.TotalBytes > 0 || j.TotalFiles > 0)
}

func jobProgressFraction(j *jobs.Job) (float64, bool) {
	if j == nil {
		return 0, false
	}
	if j.TotalBytes > 0 {
		return float64(j.DoneBytes) / float64(j.TotalBytes), true
	}
	if j.TotalFiles > 0 {
		return float64(j.DoneFiles) / float64(j.TotalFiles), true
	}
	return 0, false
}
