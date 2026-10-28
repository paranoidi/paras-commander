package jobbridge

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

// moveCount is a move's background count of the sources its copy phase copies
// (ops.MoveCopyPhase.Count). It starts when the rename phase hands over and runs beside the copy,
// emitting EventPlanTotals every interval and once more, marked complete, when the walk ends.
type moveCount struct {
	cancel context.CancelFunc
	done   chan struct{} // closed once the final totals are emitted
}

// executeMove runs ops.ExecuteMove with the background count wired in. Before the rename phase
// the totals are one item per source and the enqueue-time estimate; the count replaces them with
// the exact copy-phase totals. Source removal waits for the count so the totals are final before
// anything is deleted (and the walk never races a removal).
func executeMove(tc transferExecCtx) (int, int64, error) {
	job := tc.job
	tc.emit(jobs.Event{
		Type:       jobs.EventPlanTotals,
		JobID:      job.ID,
		Status:     jobs.StatusRunning,
		TotalFiles: len(job.Sources),
		TotalBytes: job.EstimatedBytes, // an enqueue-time field, so reading it here is race-free
	})
	var mc *moveCount
	opts := tc.opts
	opts.OnCopyPhase = func(phase ops.MoveCopyPhase) {
		mc = startMoveCount(tc.ctx, job, phase, tc.countInterval, tc.emit)
	}
	onRemove := opts.OnRemoveSources
	opts.OnRemoveSources = func(removing bool) {
		if removing && mc != nil {
			select {
			case <-mc.done:
			case <-tc.ctx.Done():
			}
		}
		if onRemove != nil {
			onRemove(removing)
		}
	}
	doneFiles, doneBytes, err := ops.ExecuteMove(tc.ctx, job.Sources, job.Destination, opts, tc.throttle, tc.progress, tc.resolver, tc.diskWait)
	if mc != nil {
		mc.cancel()
		<-mc.done // no totals event may follow the job's terminal one
	}
	return doneFiles, doneBytes, err
}

// startMoveCount starts the count of phase. Without anything to copy the rename phase's units are
// already final and are emitted complete right away. A canceled walk (the move ended first) emits
// nothing more.
func startMoveCount(ctx context.Context, job *jobs.Job, phase ops.MoveCopyPhase, interval time.Duration, emit func(jobs.Event)) *moveCount {
	ctx, cancel := context.WithCancel(ctx)
	mc := &moveCount{cancel: cancel, done: make(chan struct{})}
	emitTotals := func(files, dirs int, bytes int64, complete bool) {
		files += phase.Items
		bytes += phase.Bytes
		if !complete {
			// Until the walk is done its tally is a lower bound: keep the estimate when larger.
			bytes = max(bytes, job.EstimatedBytes)
		}
		emit(jobs.Event{
			Type:           jobs.EventPlanTotals,
			JobID:          job.ID,
			Status:         jobs.StatusRunning,
			TotalFiles:     files,
			TotalDirs:      dirs,
			TotalBytes:     bytes,
			TotalsComplete: complete,
		})
	}
	if phase.Count == nil {
		emitTotals(0, 0, 0, true)
		close(mc.done)
		return mc
	}
	if interval <= 0 {
		interval = time.Duration(config.DefaultScanProgressMinIntervalMS) * time.Millisecond
	}
	var files, dirs, bytes atomic.Int64
	walkDone := make(chan error, 1)
	go func() {
		walkDone <- phase.Count(ctx, ops.PlanBuildOptions{
			// Skip what the copy will report as failed instead of ending the count early.
			OnItemError: func(ops.PlanItem, error) {},
		}, func(it ops.PlanItem) {
			files.Add(1)
			if it.IsDir {
				dirs.Add(1)
			}
			if !it.IsDir && !it.IsSymlink {
				bytes.Add(it.FileSize)
			}
		})
	}()
	go func() {
		defer close(mc.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-walkDone:
				if ctx.Err() == nil {
					emitTotals(int(files.Load()), int(dirs.Load()), bytes.Load(), true)
				}
				return
			case <-ticker.C:
				emitTotals(int(files.Load()), int(dirs.Load()), bytes.Load(), false)
			}
		}
	}()
	return mc
}
