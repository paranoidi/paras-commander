package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/priority"
)

// PlanProducer represents an in-progress background pre-scan walk, started immediately and
// running independently of whoever holds it. Items streams discovered PlanItems as they're
// found — it is meant for exactly one consumer, the eventual transfer executor (via
// Job.PlanCh); runJobScan itself never reads from it, only from FirstItem/Totals/Done/Err
// below, so a job can leave StatusScanning and start transferring the items already produced
// while the walk continues in the background. FirstItem closes as soon as the walk has found
// its first item (or never closes for a source that turns out empty — Done covers that case).
// Totals returns the running file/dir/byte counts so far and is safe to call at any time,
// including after Done closes. Done closes once the walk has finished (success, error, or ctx
// cancellation) and Items has been fully handed off, at which point Err returns the terminal
// walk error (nil on a clean end).
type PlanProducer struct {
	Items     chan ops.PlanItem
	FirstItem <-chan struct{}
	Totals    func() (files, dirs int, bytes int64)
	Done      <-chan struct{}
	Err       func() error
}

// ScanWalkHooks are optional callbacks during a pre-scan walk.
type ScanWalkHooks struct {
	OnPath      func(path string) error
	YieldEveryN int
	Yield       func()
	// FlatDestNames requests dest/<basename> plan naming (flatten jobs); see ops.PlanBuildOptions.
	FlatDestNames bool
}

// ScanFunc starts a background plan walk for sources/destination and returns immediately with a
// PlanProducer streaming into it; wired by the app using internal/ops (see jobbridge.ScanFunc).
type ScanFunc func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) PlanProducer

// ScanConfig tunes background pre-scan walks for queued copy/move jobs.
type ScanConfig struct {
	YieldInterval       time.Duration
	YieldEveryN         int
	NiceIncrement       int
	ProgressMinInterval time.Duration
}

// SetScanConfig configures pre-scan yielding and OS priority lowering.
func (s *State) SetScanConfig(cfg ScanConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanConfig = cfg
}

// SetScanFunc sets the plan producer used for copy/move pre-scan (required for transfer jobs).
func (s *State) SetScanFunc(fn ScanFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanFunc = fn
}

func (s *State) startJobScan(job *Job) {
	if job == nil || !job.NeedsPreScan() {
		return
	}

	s.scanMu.Lock()
	if s.scanCancel == nil {
		s.scanCancel = make(map[string]context.CancelFunc)
	}
	if _, exists := s.scanCancel[job.ID]; exists {
		s.scanMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.scanCancel[job.ID] = cancel
	s.scanMu.Unlock()

	go s.runJobScan(job, ctx, cancel)
}

func (s *State) runJobScan(job *Job, ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	defer s.clearScanCancel(job.ID)

	s.mu.Lock()
	job.ScanStartedAt = time.Now()
	cfg := s.scanConfig
	scanFn := s.scanFunc
	s.mu.Unlock()

	if scanFn == nil {
		s.finishScanFailed(job, "scan function not configured")
		return
	}

	if s.transferActive() {
		restore := priority.ApplyBackgroundPriority(cfg.NiceIncrement)
		defer restore()
	}

	yieldInterval := cfg.YieldInterval
	if yieldInterval <= 0 {
		yieldInterval = time.Duration(config.DefaultScanYieldIntervalMS) * time.Millisecond
	}
	yieldEvery := cfg.YieldEveryN
	if yieldEvery <= 0 {
		yieldEvery = config.DefaultScanYieldEveryN
	}
	progressMin := cfg.ProgressMinInterval
	if progressMin <= 0 {
		progressMin = time.Duration(config.DefaultScanProgressMinIntervalMS) * time.Millisecond
	}

	var lastProgress time.Time
	hooks := ScanWalkHooks{
		FlatDestNames: job.FlatDestNames(),
		OnPath: func(path string) error {
			now := time.Now()
			if lastProgress.IsZero() || now.Sub(lastProgress) >= progressMin {
				lastProgress = now
				s.emit(Event{
					Type:        EventScanProgress,
					JobID:       job.ID,
					Status:      StatusScanning,
					CurrentPath: path,
				})
			}
			return nil
		},
		YieldEveryN: yieldEvery,
		Yield: func() {
			if !s.transferActive() {
				return
			}
			select {
			case <-ctx.Done():
			case <-time.After(yieldInterval):
			}
		},
	}

	producer := scanFn(ctx, job.Sources, job.Destination, hooks)

	s.mu.Lock()
	job.PlanCh = producer.Items
	job.PlanErr = producer.Err
	s.mu.Unlock()

	flipped := false
	flipToRunnable := func() {
		if flipped {
			return
		}
		flipped = true
		s.mu.Lock()
		if job.PausedAfterScan {
			job.Status = StatusPaused
		} else {
			job.Status = StatusQueued
		}
		s.mu.Unlock()
		s.signalWorker()
	}

	writeTotalsAndEmit := func() {
		files, dirs, bytes := producer.Totals()
		s.mu.Lock()
		job.TotalFiles = files
		job.TotalDirs = dirs
		job.TotalBytes = bytes
		status := job.Status
		s.mu.Unlock()
		s.emit(Event{
			Type:       EventScanTotals,
			JobID:      job.ID,
			Status:     status,
			TotalFiles: files,
			TotalDirs:  dirs,
			TotalBytes: bytes,
		})
	}

	ticker := time.NewTicker(progressMin)
	defer ticker.Stop()

	// firstItemCh is nil'd out after firing once so the select doesn't keep re-selecting an
	// already-closed channel (which would busy-loop instead of blocking on the next tick/Done).
	firstItemCh := producer.FirstItem

waitLoop:
	for {
		select {
		case <-producer.Done:
			break waitLoop
		case <-firstItemCh:
			// As soon as the first item is known to have arrived, let the job start
			// transferring while the walk keeps running in the background.
			flipToRunnable()
			firstItemCh = nil
		case <-ticker.C:
			writeTotalsAndEmit()
		}
	}

	// The channel closes immediately for a tiny/empty source without ever crossing the
	// files>0/dirs>0 check above; flip here too so the job doesn't stay stuck in Scanning.
	// alreadyFlipped is captured before this idempotent call so the error branch below can tell
	// whether the job had already left Scanning (and possibly started transferring) by the time
	// the walk ended.
	alreadyFlipped := flipped
	flipToRunnable()

	// Only fail/cancel the job here if it never left Scanning. Once flipToRunnable has already
	// run once, the transfer worker may be actively executing the job (or have already finished
	// it) using job.PlanCh; the executor independently observes the same walk error via
	// job.PlanErr() when the plan channel closes and owns the job's outcome from that point on.
	// Writing job.Status here too would race that event-driven completion path.
	if err := producer.Err(); err != nil && !alreadyFlipped {
		if errors.Is(err, context.Canceled) {
			s.finishScanCanceled(job)
			return
		}
		s.finishScanFailed(job, err.Error())
		return
	}

	s.mu.Lock()
	job.PlanComplete = true
	s.mu.Unlock()
	writeTotalsAndEmit()
}

func (s *State) transferActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active != nil && s.active.Status == StatusRunning
}

func (s *State) clearScanCancel(jobID string) {
	s.scanMu.Lock()
	delete(s.scanCancel, jobID)
	s.scanMu.Unlock()
}

func (s *State) cancelJobScan(jobID string) bool {
	s.scanMu.Lock()
	cancel, ok := s.scanCancel[jobID]
	s.scanMu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

func (s *State) finishScanFailed(job *Job, msg string) {
	s.mu.Lock()
	job.Status = StatusFailed
	job.Error = msg
	job.FinishedAt = time.Now()
	s.mu.Unlock()
	s.emit(Event{
		Type:   EventFailed,
		JobID:  job.ID,
		Status: StatusFailed,
		Error:  msg,
	})
}

func (s *State) finishScanCanceled(job *Job) {
	s.mu.Lock()
	job.Status = StatusCanceled
	job.FinishedAt = time.Now()
	s.mu.Unlock()
	s.emit(Event{
		Type:   EventCanceled,
		JobID:  job.ID,
		Status: StatusCanceled,
	})
}
