package jobs

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// menuBarStripGroupOrder is the fixed display order for menu-bar job status groups.
var menuBarStripGroupOrder = []string{
	string(jobs.StatusScanning),
	string(jobs.StatusCompleted),
	string(jobs.StatusFailed),
	string(jobs.StatusQueued),
	string(jobs.StatusWaitingDecision),
	string(jobs.StatusPaused),
	string(jobs.StatusRunning),
}

// menuBarJobGroups folds per-job statuses into ordered (status, count) groups, dropping
// canceled jobs entirely and hiding zero-count groups.
func menuBarJobGroups(statuses []string) []ui.MenuBarJobGroup {
	counts := make(map[string]int, len(statuses))
	for _, st := range statuses {
		if st == string(jobs.StatusCanceled) {
			continue
		}
		counts[st]++
	}
	groups := make([]ui.MenuBarJobGroup, 0, len(menuBarStripGroupOrder))
	for _, st := range menuBarStripGroupOrder {
		if c := counts[st]; c > 0 {
			groups = append(groups, ui.MenuBarJobGroup{Status: st, Count: c})
		}
	}
	return groups
}

// MenuBarStripSnapshot builds menu-bar job progress data from current job state.
func (h *Handler) MenuBarStripSnapshot() ui.MenuBarJobsStrip {
	var strip ui.MenuBarJobsStrip
	strip.Groups = menuBarJobGroups(h.state.MenuBarStripStatuses())
	all := h.state.AllJobs()
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
