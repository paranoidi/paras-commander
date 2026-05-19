// Package jobbridge connects the jobs worker to ops planning and execution.
package jobbridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

// CoalesceEventBatch keeps the last EventProgress per job ID so a drained channel
// does not apply hundreds of ETA/strip updates in one PollEvent iteration.
func CoalesceEventBatch(batch []jobs.Event) []jobs.Event {
	if len(batch) <= 1 {
		return batch
	}
	progressSlot := make(map[string]int)
	out := make([]jobs.Event, 0, len(batch))
	for _, ev := range batch {
		if ev.Type != jobs.EventProgress {
			out = append(out, ev)
			continue
		}
		if idx, ok := progressSlot[ev.JobID]; ok {
			out[idx] = ev
			continue
		}
		progressSlot[ev.JobID] = len(out)
		out = append(out, ev)
	}
	return out
}

// EventUpdatesMarks reports whether a job event type should refresh path marks.
func EventUpdatesMarks(t jobs.EventType) bool {
	switch t {
	case jobs.EventEnqueued, jobs.EventScanTotals, jobs.EventStarted, jobs.EventCompleted, jobs.EventFailed, jobs.EventCanceled, jobs.EventJobBlockerRequest:
		return true
	default:
		return false
	}
}

// ScanFunc returns the jobs scan function wired to ops plan building.
func ScanFunc() jobs.ScanFunc {
	return func(ctx context.Context, sources []string, destination string, hooks jobs.ScanWalkHooks) (jobs.ScanResult, error) {
		opts := ops.PlanBuildOptions{
			YieldEveryN: hooks.YieldEveryN,
			Yield:       hooks.Yield,
		}
		if hooks.OnPath != nil {
			opts.OnPath = hooks.OnPath
		}
		plan, totalItems, totalDirs, totalBytes, err := ops.BuildCopyPlanWithTotalsCtx(ctx, sources, destination, opts)
		if err != nil {
			return jobs.ScanResult{}, err
		}
		return jobs.ScanResult{
			Plan:       PlanItemsFromOps(plan),
			TotalFiles: totalItems,
			TotalDirs:  totalDirs,
			TotalBytes: totalBytes,
		}, nil
	}
}

// PlanItemsFromOps converts ops plan items to jobs plan items.
func PlanItemsFromOps(items []ops.PlanItem) []jobs.PlanItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]jobs.PlanItem, len(items))
	for i, p := range items {
		out[i] = jobs.PlanItem{
			Src:       p.Src,
			Dst:       p.Dst,
			IsDir:     p.IsDir,
			IsSymlink: p.IsSymlink,
			FileSize:  p.FileSize,
		}
	}
	return out
}

// PlanItemsToOps converts jobs plan items to ops plan items.
func PlanItemsToOps(items []jobs.PlanItem) []ops.PlanItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]ops.PlanItem, len(items))
	for i, p := range items {
		out[i] = ops.PlanItem{
			Src:       p.Src,
			Dst:       p.Dst,
			IsDir:     p.IsDir,
			IsSymlink: p.IsSymlink,
			FileSize:  p.FileSize,
		}
	}
	return out
}

// ActivityDetailLabel formats the activity line for a progress event.
func ActivityDetailLabel(active *jobs.Job, ev jobs.Event) string {
	if active != nil && active.ID == ev.JobID &&
		ev.CurrentDestPath != "" && active.Destination != "" {
		root := filepath.Clean(active.Destination)
		dst := filepath.Clean(ev.CurrentDestPath)
		rel, err := filepath.Rel(root, dst)
		if err == nil && rel != "." &&
			rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel
		}
	}
	label := filepath.Base(ev.CurrentPath)
	if label == "." || label == "/" {
		return ev.CurrentPath
	}
	return label
}

// TransferFunc builds the job worker transfer function from config.
func TransferFunc(opsCfg config.OperationsConfig, jobsCfg config.JobsConfig) func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
	return func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
		opts := ops.Options{
			PreservePermissions:        opsCfg.PreservePermissions,
			PreserveTimestamps:         opsCfg.PreserveTimestamps,
			CopyBufferKiB:              opsCfg.CopyBufferKiB,
			SyncAfterEachFile:          opsCfg.SyncAfterEachFile,
			DiskSpaceCheckMinFileBytes: opsCfg.DiskSpaceCheckMinFileBytes,
			CowFileCloning:             opsCfg.CowFileCloning,
		}
		throttle := ops.ProgressEmitThrottle{
			MinBytes:    int64(jobsCfg.WorkerProgressMinBytes),
			MinInterval: time.Duration(jobsCfg.WorkerProgressMinIntervalMS) * time.Millisecond,
		}
		resolver := func(src, dst string, facts ops.FileConflictFacts) (bool, error) {
			kind := facts.Kind
			if kind == "" {
				kind = "file"
			}
			req := jobs.ConflictRequest{
				JobID:           job.ID,
				Source:          src,
				Destination:     dst,
				ExistingDetails: kind + " exists",
				SourceSize:      ops.FormatConflictSize(facts.SourceSize),
				SourceTime:      ops.FormatConflictTime(facts.SourceMod),
				DestSize:        ops.FormatConflictSize(facts.DestSize),
				DestTime:        ops.FormatConflictTime(facts.DestMod),
			}
			decision := waitBlocker(jobs.BlockerRequest{
				Kind:     jobs.BlockerKindConflict,
				Conflict: &req,
			})
			switch decision {
			case jobs.DecisionOverwrite, jobs.DecisionOverwriteAll:
				return true, nil
			case jobs.DecisionSkip, jobs.DecisionSkipAll:
				return false, nil
			case jobs.DecisionCancel:
				return false, fmt.Errorf("canceled by user")
			case jobs.DecisionRetry:
				return false, fmt.Errorf("unexpected retry decision for file conflict")
			default:
				return false, nil
			}
		}
		opsPlan := PlanItemsToOps(job.Plan)
		var planErr error
		if (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove) && len(opsPlan) == 0 {
			var tf int
			var tb int64
			opsPlan, tf, _, tb, planErr = ops.BuildCopyPlanWithTotalsCtx(ctx, job.Sources, job.Destination, ops.PlanBuildOptions{})
			if planErr == nil {
				emit(jobs.Event{
					Type:       jobs.EventPlanTotals,
					JobID:      job.ID,
					Status:     jobs.StatusRunning,
					TotalFiles: tf,
					TotalBytes: tb,
				})
			}
		}
		if planErr == nil && (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove) {
			tb := job.TotalBytes
			if tb <= 0 && len(opsPlan) > 0 {
				_, _, tb = ops.SummarizePlan(opsPlan)
			}
			if job.Type == jobs.TypeCopy && tb > 0 {
				if err := ops.EnsureDiskSpace(waitBlocker, job.Destination, tb, ""); err != nil {
					return err
				}
			}
		}
		progress := func(sourcePath, destPath string, doneFiles int, doneBytes int64) {
			emit(jobs.Event{
				Type:            jobs.EventProgress,
				JobID:           job.ID,
				Status:          jobs.StatusRunning,
				DoneFiles:       doneFiles,
				DoneBytes:       doneBytes,
				CurrentPath:     sourcePath,
				CurrentDestPath: destPath,
			})
		}
		var doneFiles int
		var doneBytes int64
		var err error
		switch job.Type {
		case jobs.TypeCopy:
			if planErr != nil {
				doneFiles, doneBytes, err = ops.ExecuteCopy(ctx, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			} else {
				doneFiles, doneBytes, err = ops.ExecuteCopyUsingPlan(ctx, opsPlan, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			}
		case jobs.TypeMove:
			if len(opsPlan) > 0 {
				doneFiles, doneBytes, err = ops.ExecuteMoveWithPlan(ctx, opsPlan, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			} else {
				doneFiles, doneBytes, err = ops.ExecuteMove(ctx, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			}
		case jobs.TypeDelete:
			emit(jobs.Event{
				Type:       jobs.EventPlanTotals,
				JobID:      job.ID,
				Status:     jobs.StatusRunning,
				TotalFiles: len(job.Sources),
				TotalBytes: 0,
			})
			deleteProgress := func(path string, df int, db int64) {
				emit(jobs.Event{
					Type:        jobs.EventProgress,
					JobID:       job.ID,
					Status:      jobs.StatusRunning,
					DoneFiles:   df,
					DoneBytes:   db,
					CurrentPath: path,
				})
			}
			doneFiles, doneBytes, err = ops.ExecuteDeletePaths(ctx, job.Sources, deleteProgress)
		default:
			return fmt.Errorf("unknown job type: %s", job.Type)
		}
		if err == nil {
			job.DoneFiles = doneFiles
			job.DoneBytes = doneBytes
		}
		return err
	}
}

// Plural returns singular or plural noun form for n.
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
