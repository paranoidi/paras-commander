// Package jobbridge connects the jobs worker to ops planning and execution.
package jobbridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
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
	case jobs.EventEnqueued, jobs.EventScanTotals, jobs.EventStarted, jobs.EventCompleted, jobs.EventFailed, jobs.EventCanceled, jobs.EventJobBlockerRequest, jobs.EventJobResumed:
		return true
	default:
		return false
	}
}

// ScanFunc returns the jobs scan function wired to ops plan building.
func ScanFunc() jobs.ScanFunc {
	return func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks jobs.ScanWalkHooks) (jobs.ScanResult, error) {
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
			Src:        p.Src,
			Dst:        p.Dst,
			IsDir:      p.IsDir,
			IsSymlink:  p.IsSymlink,
			FileSize:   p.FileSize,
			Mode:       p.Mode,
			AccessTime: p.AccessTime,
			ModTime:    p.ModTime,
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
			Src:        p.Src,
			Dst:        p.Dst,
			IsDir:      p.IsDir,
			IsSymlink:  p.IsSymlink,
			FileSize:   p.FileSize,
			Mode:       p.Mode,
			AccessTime: p.AccessTime,
			ModTime:    p.ModTime,
		}
	}
	return out
}

// ActivityDetailLabel formats the activity line for a progress event.
func ActivityDetailLabel(active *jobs.Job, ev jobs.Event) string {
	if active != nil && active.ID == ev.JobID &&
		ev.CurrentDestPath != "" && !active.Destination.IsZero() {
		rootLoc, err1 := pathloc.Parse(active.Destination.String())
		dstLoc, err2 := pathloc.Parse(ev.CurrentDestPath)
		if err1 != nil || err2 != nil {
			goto baseLabel
		}
		if rootLoc.IsRemote() || dstLoc.IsRemote() {
			if dstLoc.HasPrefix(rootLoc) && !dstLoc.Equal(rootLoc) {
				rootS := rootLoc.String()
				dstS := dstLoc.String()
				if strings.HasPrefix(dstS, rootS) {
					rel := strings.TrimPrefix(dstS, rootS)
					rel = strings.TrimPrefix(rel, "/")
					if rel != "" && rel != "." {
						return rel
					}
				}
			}
			goto baseLabel
		}
		root, err := rootLoc.FilePath()
		if err != nil {
			goto baseLabel
		}
		dst, err := dstLoc.FilePath()
		if err != nil {
			goto baseLabel
		}
		rel, err := filepath.Rel(root, dst)
		if err == nil && rel != "." &&
			rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return rel
		}
	}
baseLabel:
	label := filepath.Base(ev.CurrentPath)
	if label == "." || label == "/" {
		return ev.CurrentPath
	}
	return label
}

// ActivityFailureLabel formats a terminal failure line for the jobs Activity panel.
func ActivityFailureLabel(ev jobs.Event) string {
	if ev.Type != jobs.EventFailed {
		return ""
	}
	msg := strings.TrimSpace(ev.Error)
	if ev.Err != nil {
		msg = strings.TrimSpace(ev.Err.Error())
	}
	if msg == "" {
		return "Failed"
	}
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = strings.TrimSpace(msg[:i])
	}
	return "Failed: " + msg
}

// TransferFunc builds the job worker transfer function from config.
func TransferFunc(opsCfg config.OperationsConfig, jobsCfg config.JobsConfig) func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
	return func(ctx context.Context, job *jobs.Job, emit func(jobs.Event), waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) error {
		opts := ops.Options{
			PreservePermissions:        job.PreservePermissions,
			PreserveTimestamps:         job.PreserveTimestamps,
			CopyBufferKiB:              opsCfg.CopyBufferKiB,
			SyncAfterEachFile:          opsCfg.SyncAfterEachFile,
			DiskSpaceCheckMinFileBytes: opsCfg.DiskSpaceCheckMinFileBytes,
			CowFileCloning:             opsCfg.CowFileCloning,
		}
		if job.Destination.IsRemote() {
			opts.CowFileCloning = false
		}
		for _, src := range job.Sources {
			if src.IsRemote() {
				opts.CowFileCloning = false
				break
			}
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
		if (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove || job.Type == jobs.TypeFlatten) && len(opsPlan) == 0 {
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
		if planErr == nil && (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove || job.Type == jobs.TypeFlatten) {
			tb := job.TotalBytes
			if tb <= 0 && len(opsPlan) > 0 {
				_, _, tb = ops.SummarizePlan(opsPlan)
			}
			if job.Type == jobs.TypeCopy && tb > 0 {
				if err := ops.EnsureDiskSpace(waitBlocker, job.Destination, tb, pathloc.Path{}); err != nil {
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
		case jobs.TypeMove, jobs.TypeFlatten:
			if len(opsPlan) > 0 {
				doneFiles, doneBytes, err = ops.ExecuteMoveWithPlan(ctx, opsPlan, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			} else {
				doneFiles, doneBytes, err = ops.ExecuteMove(ctx, job.Sources, job.Destination, opts, throttle, progress, resolver, waitBlocker)
			}
			if err == nil && job.Type == jobs.TypeFlatten && job.FlattenRemoveEmpty {
				if cleanErr := ops.RemoveEmptyDirsUnder(ctx, job.FlattenRoots); cleanErr != nil {
					err = cleanErr
				}
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
			doneFiles, doneBytes, err = ops.ExecuteDeletePaths(ctx, pathloc.Strings(job.Sources), deleteProgress)
		case jobs.TypeExtract:
			emit(jobs.Event{
				Type:       jobs.EventPlanTotals,
				JobID:      job.ID,
				Status:     jobs.StatusRunning,
				TotalFiles: len(job.Sources),
				TotalBytes: 0,
			})
			tc := archive.ProbeToolchain()
			plan, _, extractPlanErr := ops.PlanExtract(pathloc.Strings(job.Sources), job.Destination.String(), tc)
			if extractPlanErr != nil {
				err = extractPlanErr
			} else {
				extractProgress := func(path string, df int) {
					emit(jobs.Event{
						Type:        jobs.EventProgress,
						JobID:       job.ID,
						Status:      jobs.StatusRunning,
						DoneFiles:   df,
						DoneBytes:   0,
						CurrentPath: path,
					})
				}
				doneFiles, err = ops.ExecuteExtract(ctx, plan, extractProgress)
				doneBytes = 0
			}
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
