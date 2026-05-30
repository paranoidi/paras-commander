package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/priority"
)

// ScanResult is returned by ScanFunc after a successful pre-scan walk.
type ScanResult struct {
	Plan       []PlanItem
	TotalFiles int
	TotalDirs  int
	TotalBytes int64
}

// ScanWalkHooks are optional callbacks during a pre-scan walk.
type ScanWalkHooks struct {
	OnPath      func(path string) error
	YieldEveryN int
	Yield       func()
}

// ScanFunc builds a transfer plan and totals; wired by the app using internal/ops.
type ScanFunc func(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, hooks ScanWalkHooks) (ScanResult, error)

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

// SetScanFunc sets the plan builder used for copy/move pre-scan (required for transfer jobs).
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

	var lastProgress time.Time
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

	hooks := ScanWalkHooks{
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

	result, err := scanFn(ctx, job.Sources, job.Destination, hooks)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.finishScanCanceled(job)
			return
		}
		s.finishScanFailed(job, err.Error())
		return
	}

	pausedAfter := false
	s.mu.Lock()
	job.Plan = result.Plan
	job.TotalFiles = result.TotalFiles
	job.TotalDirs = result.TotalDirs
	job.TotalBytes = result.TotalBytes
	pausedAfter = job.PausedAfterScan
	if pausedAfter {
		job.Status = StatusPaused
	} else {
		job.Status = StatusQueued
	}
	nextStatus := job.Status
	s.mu.Unlock()

	s.emit(Event{
		Type:       EventScanTotals,
		JobID:      job.ID,
		Status:     nextStatus,
		TotalFiles: result.TotalFiles,
		TotalDirs:  result.TotalDirs,
		TotalBytes: result.TotalBytes,
	})
	s.signalWorker()
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
