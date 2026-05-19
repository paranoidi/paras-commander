package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// throughputChartTickPayload closes one throughput chart column on the main loop.
type throughputChartTickPayload struct{}

func (a *App) runThroughputChartTicker(columnDur time.Duration, stop <-chan struct{}) {
	if columnDur <= 0 {
		return
	}
	ticker := time.NewTicker(columnDur)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !a.config.Jobs.ThroughputChartEnabled || !a.jobState.HasUnfinishedWork() {
				continue
			}
			// Coalesce backlog so a stalled main loop still advances one column per wake.
			for {
				select {
				case <-ticker.C:
					continue
				default:
					goto post
				}
			}
		post:
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(throughputChartTickPayload{}))
		}
	}
}

func (a *App) applyThroughputChartTick() bool {
	if !a.config.Jobs.ThroughputChartEnabled {
		return false
	}
	if !a.jobState.CloseActiveJobThroughputColumn(time.Now()) {
		return false
	}
	if a.model.ViewMode == ui.ViewJobs {
		a.syncJobsList()
		return true
	}
	a.jobsCtrl.SetListStale(true)
	return false
}
