// Package jobbridge connects the jobs worker to ops planning and execution.
package jobbridge

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// ScanFunc returns the jobs plan producer wired to ops.BuildPlanStreamCtx. It runs two
// independent walks over the same sources/destination: a delivery walk (raw → relay → items,
// gating FirstItem) that the transfer executor drains via Job.PlanCh, and a separate counting
// walk (countCh → count goroutine) that only tallies running Totals and is never blocked by a
// slow/stalled transfer consumer — a large file mid-copy stalls the relay's handoff to items,
// but the counting walk keeps enumerating and Totals keeps growing regardless. Both walks start
// immediately; ScanFunc returns without blocking for the source tree to be fully enumerated.
//
// The counting walk additionally runs an adaptive contention probe (see scan_throttle.go) unless
// jobsCfg.ScanDisableAdaptiveThrottle is set: it periodically pauses the counting walk and
// measures whether that improves the job's transfer throughput, growing a duty-cycle pause when
// it measurably does and decaying it otherwise. This is pure measured-evidence throttling — no
// disk-type detection — so it self-corrects to zero pause on SSD/NVMe/network storage.
func ScanFunc(jobsCfg config.JobsConfig) jobs.ScanFunc {
	return func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks jobs.ScanWalkHooks) jobs.PlanProducer {
		opts := ops.PlanBuildOptions{
			YieldEveryN:   hooks.YieldEveryN,
			Yield:         hooks.Yield,
			FlatDestNames: hooks.FlatDestNames,
			OnPath:        hooks.OnPath,
		}

		// countOpts is the counting walk's own copy of opts (the delivery walk above keeps the
		// original, untouched) with Yield wrapped by the adaptive throttle when a throughput
		// signal is available and the probe isn't disabled.
		countOpts := opts
		if hooks.ThroughputBPS != nil && !jobsCfg.ScanDisableAdaptiveThrottle {
			countOpts.Yield = newAdaptiveThrottleYield(opts.Yield, hooks.ThroughputBPS)
		}

		// raw is the bounded channel BuildPlanStreamCtx's delivery-walk goroutine sends into; it
		// is the one memory bound for the delivery pipeline (see
		// config.DefaultPlanStreamBufferItems). items is unbuffered: the relay goroutine below
		// forwards one item at a time to whoever eventually reads Job.PlanCh, so backpressure on
		// a slow/absent consumer propagates straight back through raw to the delivery walk
		// itself. This backpressure is exactly why totals must come from the separate counting
		// walk below rather than from this relay.
		raw := make(chan ops.PlanItem, config.DefaultPlanStreamBufferItems)
		items := make(chan ops.PlanItem)
		firstItem := make(chan struct{})
		deliveryDone := make(chan struct{})
		walkErrCh := make(chan error, 1)

		// files/dirs/totalBytes are updated once per discovered item by the counting goroutine
		// below (potentially millions of times for a large tree) and read by Totals() every
		// ~200ms (jobs/scan.go's ticker); atomics keep that off any lock.
		var files, dirs, totalBytes atomic.Int64
		var walkErr error

		go func() {
			walkErrCh <- ops.BuildPlanStreamCtx(ctx, sources, destination, true, opts, raw)
		}()

		go func() {
			defer close(items)
			// finish records the walk's terminal error and signals deliveryDone. walkErr is
			// written here exactly once; every reader (job.PlanErr, Err() below) only consults it
			// after observing items/job.PlanCh close or receiving from deliveryDone/Done, each of
			// which happens-after this write per Go's channel memory model, so no mutex is needed
			// to publish it.
			finish := func() {
				walkErr = <-walkErrCh
				close(deliveryDone)
			}
			firstItemSeen := false
			for {
				select {
				case it, ok := <-raw:
					if !ok {
						finish()
						return
					}
					if !firstItemSeen {
						firstItemSeen = true
						close(firstItem)
					}
					select {
					case items <- it:
					case <-ctx.Done():
						finish()
						return
					}
				case <-ctx.Done():
					finish()
					return
				}
			}
		}()

		// Counting walk: a second, independent BuildPlanStreamCtx call over the same
		// sources/destination, drained by a goroutine that does nothing but tally
		// files/dirs/totalBytes and discard each item. Because nothing downstream of countCh
		// ever blocks, this walk proceeds at full enumeration speed no matter how slowly the
		// delivery side above is being consumed. Its own walk error is intentionally discarded:
		// PlanProducer.Err must reflect only the delivery walk's terminal error, since that is
		// what job.PlanErr feeds to the executor.
		countCh := make(chan ops.PlanItem, config.DefaultPlanStreamBufferItems)
		countDone := make(chan struct{})

		go func() {
			_ = ops.BuildPlanStreamCtx(ctx, sources, destination, true, countOpts, countCh)
		}()

		go func() {
			defer close(countDone)
			for it := range countCh {
				files.Add(1)
				if it.IsDir {
					dirs.Add(1)
				}
				if !it.IsDir && !it.IsSymlink {
					totalBytes.Add(it.FileSize)
				}
			}
		}()

		done := make(chan struct{})
		go func() {
			<-deliveryDone
			<-countDone
			close(done)
		}()

		return jobs.PlanProducer{
			Items:     items,
			FirstItem: firstItem,
			Totals: func() (int, int, int64) {
				return int(files.Load()), int(dirs.Load()), totalBytes.Load()
			},
			Done: done,
			Err: func() error {
				return walkErr
			},
		}
	}
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
		diskWait := diskWaitFromBlocker(waitBlocker)
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

		// runTransfer executes tc and, on success, emits the final DoneFiles/DoneBytes tally
		// through emit (like every other progress update in this function) rather than writing
		// job fields directly: job.Status transitions and ApplyEvent-driven field writes are the
		// app event loop's job, not the worker's — direct writes here would race the event loop
		// reading the same job via AllJobs()/Snapshot().
		runTransfer := func(tc transferExecCtx) error {
			doneFiles, doneBytes, err := executeJobByType(tc)
			if err == nil {
				emit(jobs.Event{
					Type:      jobs.EventProgress,
					JobID:     job.ID,
					Status:    jobs.StatusRunning,
					DoneFiles: doneFiles,
					DoneBytes: doneBytes,
				})
			}
			return mapOpsCanceled(err)
		}

		// job.PlanCh is set once by the background pre-scan producer (jobs/scan.go) before the
		// job ever becomes dequeue-eligible, so reading it here is race-free even though that
		// producer may still be actively streaming. job.TotalFiles/TotalDirs/TotalBytes/
		// PlanComplete are NOT: the producer keeps writing those under jobs.State's lock for as
		// long as it runs, which this package has no access to, so this function must not read
		// them directly. That is why the streamed path below skips the upfront whole-payload
		// EnsureDiskSpace gate unconditionally instead of only when PlanComplete is already
		// true — ops.copyRegularItem's per-file check (gated by disk_space_check_min_file_bytes)
		// is the safety net for streamed jobs, same trade-off mc makes (mc has no upfront check
		// at all); see llm-docs/jobs.md.
		if job.PlanCh != nil && (job.Type == jobs.TypeCopy || job.Type == jobs.TypeMove || job.Type == jobs.TypeFlatten) {
			return runTransfer(transferExecCtx{
				ctx:      ctx,
				job:      job,
				opts:     opts,
				throttle: throttle,
				progress: progress,
				resolver: resolver,
				diskWait: diskWait,
				emit:     emit,
			})
		}

		// Fallback path: job.PlanCh is nil, meaning this job's scan either never ran (delete/
		// extract) or was bypassed (e.g. a copy/move/flatten job injected directly into
		// StatusQueued in tests). opsPlan/totalBytes come from job.Plan — the synchronous-
		// rebuild slice fallback — safe to read unlocked because nothing streams into it.
		opsPlan := job.Plan
		var planErr error
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
				if err := ops.EnsureDiskSpace(diskWait, job.Destination, tb, pathloc.Path{}); err != nil {
					return mapOpsCanceled(err)
				}
			}
		}
		return runTransfer(transferExecCtx{
			ctx:      ctx,
			job:      job,
			opsPlan:  opsPlan,
			planErr:  planErr,
			opts:     opts,
			throttle: throttle,
			progress: progress,
			resolver: resolver,
			diskWait: diskWait,
			emit:     emit,
		})
	}
}

// diskWaitFromBlocker adapts the jobs blocker callback to ops.DiskWaitFunc.
func diskWaitFromBlocker(waitBlocker func(jobs.BlockerRequest) jobs.ConflictDecision) ops.DiskWaitFunc {
	if waitBlocker == nil {
		return nil
	}
	return func(req ops.DiskSpaceWaitRequest) ops.DiskSpaceWaitDecision {
		decision := waitBlocker(jobs.BlockerRequest{
			Kind: jobs.BlockerKindDiskSpace,
			DiskSpace: &jobs.DiskSpaceBlockerRequest{
				Destination:    req.Destination,
				RequiredBytes:  req.RequiredBytes,
				AvailableBytes: req.AvailableBytes,
				AvailableKnown: req.AvailableKnown,
				NextSource:     req.NextSource,
			},
		})
		if decision == jobs.DecisionCancel {
			return ops.DiskSpaceWaitCancel
		}
		return ops.DiskSpaceWaitRetry
	}
}

func mapOpsCanceled(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ops.ErrUserCanceled) {
		return jobs.ErrUserCanceled
	}
	return err
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
		case jobs.DecisionOverwriteAllSameSize:
			return facts.SourceSize == facts.DestSize, nil
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
	ctx      context.Context
	job      *jobs.Job
	opsPlan  []ops.PlanItem
	planErr  error
	opts     ops.Options
	throttle ops.ProgressEmitThrottle
	progress func(sourcePath, destPath string, doneFiles int, doneBytes int64)
	resolver func(src, dst string, facts ops.FileConflictFacts) (bool, error)
	diskWait ops.DiskWaitFunc
	emit     func(jobs.Event)
}

// planSource names where tc's plan comes from, for ops.ExecuteCopyFrom/ExecuteMoveFrom: a
// streaming background producer (job.PlanCh) takes priority when present; otherwise an
// already-built, non-empty tc.opsPlan is used; otherwise the zero PlanSource tells the Execute*
// call to build (and, for a prior failed synchronous build, retry building) the plan itself.
func (tc transferExecCtx) planSource() ops.PlanSource {
	if tc.job.PlanCh != nil {
		return ops.PlanSource{Chan: tc.job.PlanCh, ChanErr: tc.job.PlanErr}
	}
	if tc.planErr == nil && len(tc.opsPlan) > 0 {
		return ops.PlanSource{Slice: tc.opsPlan}
	}
	return ops.PlanSource{}
}

// executeJobByType runs the ops.Execute* call matching tc.job.Type (copy/move/flatten dispatch
// on tc.planSource(); delete/extract build and emit their own PlanTotals since they don't take a
// shared plan).
func executeJobByType(tc transferExecCtx) (doneFiles int, doneBytes int64, err error) {
	job := tc.job
	switch job.Type {
	case jobs.TypeCopy:
		doneFiles, doneBytes, err = ops.ExecuteCopyFrom(tc.ctx, tc.planSource(), job.Sources, job.Destination, tc.opts, tc.throttle, tc.progress, tc.resolver, tc.diskWait)
	case jobs.TypeMove, jobs.TypeFlatten:
		doneFiles, doneBytes, err = ops.ExecuteMoveFrom(tc.ctx, tc.planSource(), job.Sources, job.Destination, tc.opts, tc.throttle, tc.progress, tc.resolver, tc.diskWait)
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
