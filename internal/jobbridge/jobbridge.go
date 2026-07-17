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
			YieldEveryN:   hooks.YieldEveryN,
			Yield:         hooks.Yield,
			FlatDestNames: hooks.FlatDestNames,
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
		opts, throttle := buildTransferOptions(job, opsCfg, jobsCfg)
		resolver := newConflictResolver(job, waitBlocker)
		opsPlan := PlanItemsToOps(job.Plan)
		var planErr error
		// totalBytes starts from job.TotalBytes (set during the pre-scan phase, which
		// happens-before this call via the queue/dequeue lock). If we build a fresh plan
		// below we use that value directly instead of reading job.TotalBytes back after
		// emit: the emit is only enqueued, not yet applied by ApplyEvent on the app's
		// event-loop goroutine, so re-reading the field here would race that write.
		totalBytes := job.TotalBytes
		if (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove || job.Type == jobs.TypeFlatten) && len(opsPlan) == 0 {
			var tf int
			opsPlan, tf, _, totalBytes, planErr = ops.BuildCopyPlanWithTotalsCtx(ctx, job.Sources, job.Destination, ops.PlanBuildOptions{FlatDestNames: job.FlatDestNames()})
			if planErr == nil {
				emit(jobs.Event{
					Type:       jobs.EventPlanTotals,
					JobID:      job.ID,
					Status:     jobs.StatusRunning,
					TotalFiles: tf,
					TotalBytes: totalBytes,
				})
			}
		}
		if planErr == nil && (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove || job.Type == jobs.TypeFlatten) {
			tb := totalBytes
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
		doneFiles, doneBytes, err := executeJobByType(transferExecCtx{
			ctx:         ctx,
			job:         job,
			opsPlan:     opsPlan,
			planErr:     planErr,
			opts:        opts,
			throttle:    throttle,
			progress:    progress,
			resolver:    resolver,
			waitBlocker: waitBlocker,
			emit:        emit,
		})
		if err == nil {
			// Final tally goes through emit (like every other progress update in this
			// function) rather than writing job fields directly: job.Status transitions
			// and ApplyEvent-driven field writes are the app event loop's job, not the
			// worker's — direct writes here would race the event loop reading the same
			// job via AllJobs()/Snapshot().
			emit(jobs.Event{
				Type:      jobs.EventProgress,
				JobID:     job.ID,
				Status:    jobs.StatusRunning,
				DoneFiles: doneFiles,
				DoneBytes: doneBytes,
			})
		}
		return err
	}
}

// buildTransferOptions builds the ops.Options and progress-emit throttle for a transfer job.
// CoW cloning / copy_file_range / sparse-file copy / preallocation are local-filesystem-only
// optimizations, so they're stripped when either the destination or any source is remote.
func buildTransferOptions(job *jobs.Job, opsCfg config.OperationsConfig, jobsCfg config.JobsConfig) (ops.Options, ops.ProgressEmitThrottle) {
	opts := ops.Options{
		PreservePermissions:        job.PreservePermissions,
		PreserveTimestamps:         job.PreserveTimestamps,
		CopyBufferKiB:              opsCfg.CopyBufferKiB,
		SyncAfterEachFile:          opsCfg.SyncAfterEachFile,
		DiskSpaceCheckMinFileBytes: opsCfg.DiskSpaceCheckMinFileBytes,
		CowFileCloning:             opsCfg.CowFileCloning,
		CopyFileRange:              opsCfg.CopyFileRange,
		SparseFileCopy:             opsCfg.SparseFileCopy,
		PreallocateDestination:     opsCfg.PreallocateDestination,
		PreallocateMinFileBytes:    opsCfg.PreallocateMinFileBytes,
		SyncAtJobEnd:               opsCfg.SyncAtJobEnd,
		SyncMinFileKiB:             opsCfg.SyncMinFileKiB,
		FlatDestNames:              job.FlatDestNames(),
	}
	if job.Destination.IsRemote() {
		opts.CowFileCloning = false
		opts.CopyFileRange = false
		opts.SparseFileCopy = false
		opts.PreallocateDestination = false
	}
	for _, src := range job.Sources {
		if src.IsRemote() {
			opts.CowFileCloning = false
			opts.CopyFileRange = false
			opts.SparseFileCopy = false
			opts.PreallocateDestination = false
			break
		}
	}
	throttle := ops.ProgressEmitThrottle{
		MinBytes:    int64(jobsCfg.WorkerProgressMinBytes),
		MinInterval: time.Duration(jobsCfg.WorkerProgressMinIntervalMS) * time.Millisecond,
	}
	return opts, throttle
}

// newConflictResolver builds the per-file conflict resolver passed to ops.Execute*: it turns a
// file conflict into a jobs.BlockerRequest, blocks on waitBlocker for the user's decision, and
// translates that decision into ops' (overwrite bool, error) resolver contract.
func newConflictResolver(job *jobs.Job, waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) func(src, dst string, facts ops.FileConflictFacts) (bool, error) {
	return func(src, dst string, facts ops.FileConflictFacts) (bool, error) {
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
			return false, jobs.ErrUserCanceled
		case jobs.DecisionRetry:
			return false, fmt.Errorf("unexpected retry decision for file conflict")
		default:
			return false, nil
		}
	}
}

// transferExecCtx bundles the parameters executeJobByType needs to run the ops.Execute* call
// for job.Type; it exists purely to keep that function's signature manageable.
type transferExecCtx struct {
	ctx         context.Context
	job         *jobs.Job
	opsPlan     []ops.PlanItem
	planErr     error
	opts        ops.Options
	throttle    ops.ProgressEmitThrottle
	progress    func(sourcePath, destPath string, doneFiles int, doneBytes int64)
	resolver    func(src, dst string, facts ops.FileConflictFacts) (bool, error)
	waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision
	emit        func(jobs.Event)
}

// executeJobByType runs the ops.Execute* call matching tc.job.Type (copy/move/flatten use the
// pre-built plan when available; delete/extract build and emit their own PlanTotals since they
// don't take a shared plan).
func executeJobByType(tc transferExecCtx) (doneFiles int, doneBytes int64, err error) {
	job := tc.job
	switch job.Type {
	case jobs.TypeCopy:
		if tc.planErr != nil {
			doneFiles, doneBytes, err = ops.ExecuteCopy(tc.ctx, job.Sources, job.Destination, tc.opts, tc.throttle, tc.progress, tc.resolver, tc.waitBlocker)
		} else {
			doneFiles, doneBytes, err = ops.ExecuteCopyUsingPlan(tc.ctx, tc.opsPlan, job.Sources, job.Destination, tc.opts, tc.throttle, tc.progress, tc.resolver, tc.waitBlocker)
		}
	case jobs.TypeMove, jobs.TypeFlatten:
		if len(tc.opsPlan) > 0 {
			doneFiles, doneBytes, err = ops.ExecuteMoveWithPlan(tc.ctx, tc.opsPlan, job.Sources, job.Destination, tc.opts, tc.throttle, tc.progress, tc.resolver, tc.waitBlocker)
		} else {
			doneFiles, doneBytes, err = ops.ExecuteMove(tc.ctx, job.Sources, job.Destination, tc.opts, tc.throttle, tc.progress, tc.resolver, tc.waitBlocker)
		}
		if err == nil && job.Type == jobs.TypeFlatten && job.FlattenRemoveEmpty {
			if cleanErr := ops.RemoveEmptyDirsUnder(tc.ctx, job.FlattenRoots); cleanErr != nil {
				err = cleanErr
			}
		}
	case jobs.TypeDelete:
		tc.emit(jobs.Event{
			Type:       jobs.EventPlanTotals,
			JobID:      job.ID,
			Status:     jobs.StatusRunning,
			TotalFiles: len(job.Sources),
			TotalBytes: 0,
		})
		deleteProgress := func(path string, df int, db int64) {
			tc.emit(jobs.Event{
				Type:        jobs.EventProgress,
				JobID:       job.ID,
				Status:      jobs.StatusRunning,
				DoneFiles:   df,
				DoneBytes:   db,
				CurrentPath: path,
			})
		}
		doneFiles, doneBytes, err = ops.ExecuteDeletePaths(tc.ctx, pathloc.Strings(job.Sources), deleteProgress)
		if err == nil && job.DeleteRemoveEmptyDirs {
			if cleanErr := ops.RemoveEmptyDirsUnder(tc.ctx, uniqueParents(job.Sources)); cleanErr != nil {
				err = cleanErr
			}
		}
	case jobs.TypeExtract:
		tc.emit(jobs.Event{
			Type:       jobs.EventPlanTotals,
			JobID:      job.ID,
			Status:     jobs.StatusRunning,
			TotalFiles: len(job.Sources),
			TotalBytes: 0,
		})
		toolchain := archive.ProbeToolchain()
		plan, _, extractPlanErr := ops.PlanExtract(pathloc.Strings(job.Sources), job.Destination.String(), toolchain)
		if extractPlanErr != nil {
			err = extractPlanErr
		} else {
			extractProgress := func(path string, df int) {
				tc.emit(jobs.Event{
					Type:        jobs.EventProgress,
					JobID:       job.ID,
					Status:      jobs.StatusRunning,
					DoneFiles:   df,
					DoneBytes:   0,
					CurrentPath: path,
				})
			}
			doneFiles, err = ops.ExecuteExtract(tc.ctx, plan, extractProgress)
			doneBytes = 0
		}
	default:
		return 0, 0, fmt.Errorf("unknown job type: %s", job.Type)
	}
	return doneFiles, doneBytes, err
}

// Plural returns singular or plural noun form for n.
func Plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// uniqueParents returns the distinct parent directories of paths, in first-seen order.
func uniqueParents(paths []pathloc.Path) []pathloc.Path {
	seen := make(map[string]bool, len(paths))
	out := make([]pathloc.Path, 0, len(paths))
	for _, p := range paths {
		parent := p.Parent()
		key := parent.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, parent)
	}
	return out
}
