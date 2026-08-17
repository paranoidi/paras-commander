package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// throughputChartTickPayload samples the active job's throughput on the main loop.
type throughputChartTickPayload struct{}

// throughputTickDivisor makes the ticker fire several times per chart column. Ticking at exactly
// one column duration would drift in and out of phase with the column grid, so some columns closed
// two ticks late and others not at all — the chart advanced 0, 1 or 2 columns per tick instead of
// scrolling at a steady rate. Oversampling costs a cheap no-op tick and pins each column to its
// wall-clock boundary.
const throughputTickDivisor = 4

// throughputTickMinInterval floors the oversampled tick so the smallest configured column
// (throughput_chart_column_ms clamps at 80) cannot spin the main loop.
const throughputTickMinInterval = 40 * time.Millisecond

func throughputTickInterval(columnDur time.Duration) time.Duration {
	return min(max(columnDur/throughputTickDivisor, throughputTickMinInterval), columnDur)
}

// runThroughputChartTicker drives transfer-speed sampling. It runs regardless of
// [jobs].throughput_chart_enabled because the same clock feeds DisplaySpeedBPS (the Speed column),
// not just the chart strip.
func (a *App) runThroughputChartTicker(columnDur time.Duration, stop <-chan struct{}) {
	if columnDur <= 0 {
		return
	}
	ticker := time.NewTicker(throughputTickInterval(columnDur))
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if !a.jobState.HasUnfinishedWork() {
				continue
			}
			// Coalesce backlog: a stalled main loop catches up inside SampleThroughputColumns.
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
	if !a.jobState.SampleActiveJobThroughput(time.Now()) {
		return false
	}
	if a.model.ViewMode == ui.ViewJobs {
		a.jobsCtrl.SyncJobsList()
		return true
	}
	a.jobsCtrl.SetListStale(true)
	return false
}
