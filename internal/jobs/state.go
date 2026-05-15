package jobs

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State provides a thread-safe view of all tracked jobs for the UI layer.
type State struct {
	mu     sync.Mutex
	queue  *Queue
	active *Job // Job currently holding the transfer lease (running copy/move), or nil.
	// waitingBlocker holds jobs that yielded the lease while awaiting user blocker input (FIFO).
	waitingBlocker []*Job
	finished       []*Job
	cancelRun      context.CancelFunc
	events         chan Event
	// wake unblocks the worker when a job is enqueued while the queue was empty.
	wake chan struct{}

	// transferLease serializes filesystem transfer work across concurrent job goroutines.
	transferLease sync.Mutex

	// blockerWait maps job ID -> channel for one in-flight blocker prompt per job.
	blockerRegMu sync.Mutex
	blockerWait  map[string]chan ConflictDecision

	// pendingDequeued lists jobs removed from the FIFO queue but not yet holding the transfer lease.
	// runWorker may dequeue the next runnable job before an earlier runJob goroutine acquires the lease;
	// keeping every dequeued job here ensures AllJobs() stays complete until each runJob claims s.active.
	pendingDequeued []*Job

	// TransferFunc is called by the worker to copy/move files, allowing tests to inject
	// custom implementations. emit must be used for all job-related UI events (same path as State.emit).
	TransferFunc func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error

	emitMu   sync.RWMutex
	emitHook func(Event)

	throughputChartBin     time.Duration
	throughputChartWindow  time.Duration
	throughputChartEnabled bool
}

// SetThroughputChart configures fixed-bin scrolling for the jobs details throughput strip.
// When enabled is false, progress no longer advances strip state (no chart backend work).
func (s *State) SetThroughputChart(bin, window time.Duration, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.throughputChartBin = bin
	s.throughputChartWindow = window
	s.throughputChartEnabled = enabled
}

func (s *State) advanceThroughputStrip(job *Job, now time.Time, doneBytes int64) {
	if !s.throughputChartEnabled {
		return
	}
	bin := s.throughputChartBin
	if bin <= 0 {
		bin = 400 * time.Millisecond
	}
	win := s.throughputChartWindow
	if win <= 0 {
		win = 45 * time.Second
	}
	AdvanceJobThroughputStrip(job, now, doneBytes, bin, win)
}

// NewState creates a job state manager connected to a fresh queue and worker.
func NewState() *State {
	return &State{
		queue:                  NewQueue(),
		events:                 make(chan Event, 100),
		wake:                   make(chan struct{}, 1),
		blockerWait:            make(map[string]chan ConflictDecision),
		throughputChartEnabled: true,
	}
}

// Queue returns the underlying job queue.
func (s *State) Queue() *Queue {
	return s.queue
}

// Events returns the channel on which the worker publishes events.
func (s *State) Events() <-chan Event {
	return s.events
}

// SetEmitHook registers a callback invoked after each event is successfully queued on Events().
// The hook runs on the worker goroutine; it must be non-blocking and safe with PollEvent driving apps.
func (s *State) SetEmitHook(fn func(Event)) {
	s.emitMu.Lock()
	s.emitHook = fn
	s.emitMu.Unlock()
}

// dequeueJob removes the first runnable job from the queue and appends it to pendingDequeued
// while it waits for the transfer lease. Caller must not be holding s.mu.
func (s *State) dequeueJob() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.queue.DequeueRunnable()
	if job != nil {
		s.pendingDequeued = append(s.pendingDequeued, job)
	}
	return job
}

func (s *State) removePendingDequeuedByID(id string) {
	for i, j := range s.pendingDequeued {
		if j != nil && j.ID == id {
			s.pendingDequeued = append(s.pendingDequeued[:i], s.pendingDequeued[i+1:]...)
			return
		}
	}
}

// AddJob adds a job to the queue and emits an enqueued event.
func (s *State) AddJob(job *Job) {
	s.mu.Lock()
	s.queue.Enqueue(job)
	s.mu.Unlock()

	s.emit(Event{
		Type:   EventEnqueued,
		JobID:  job.ID,
		Status: job.Status,
	})
	s.signalWorker()
}

func (s *State) signalWorker() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// ActiveJob returns the job currently holding the transfer lease, or nil.
func (s *State) ActiveJob() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil
	}
	cp := *s.active
	return &cp
}

// JobsWaitingDecision returns the count of jobs blocked on user blocker input.
func (s *State) JobsWaitingDecision() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waitingBlocker)
}

// HasUnfinishedWork reports whether any job is running or queued and not in a terminal state.
// Jobs kept only in the finished archive do not count.
func (s *State) HasUnfinishedWork() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && !s.active.Status.IsFinished() {
		return true
	}
	for _, j := range s.waitingBlocker {
		if j != nil && !j.Status.IsFinished() {
			return true
		}
	}
	for _, j := range s.pendingDequeued {
		if j != nil && !j.Status.IsFinished() {
			return true
		}
	}
	for _, j := range s.queue.AllJobs() {
		if !j.Status.IsFinished() {
			return true
		}
	}
	return false
}

// ResumeJob resumes a paused queued job. Returns false if the job was not found or not paused.
func (s *State) ResumeJob(id string) bool {
	if !s.queue.ResumePausedJob(id) {
		return false
	}
	s.signalWorker()
	return true
}

// PauseQueuedJob pauses a queued job still waiting in the FIFO. Returns false if not found or not StatusQueued.
func (s *State) PauseQueuedJob(id string) bool {
	return s.queue.PauseQueuedJob(id)
}

// AllJobs returns active job (if any), jobs waiting for blocker input, queued jobs, then recently finished jobs.
func (s *State) AllJobs() []*Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	var all []*Job
	if s.active != nil {
		all = append(all, s.active)
	}
	for _, j := range s.waitingBlocker {
		if j != nil {
			all = append(all, j)
		}
	}
	for _, j := range s.pendingDequeued {
		if j != nil {
			all = append(all, j)
		}
	}
	all = append(all, s.queue.AllJobs()...)
	if len(s.finished) > 0 {
		all = append(all, s.finished...)
	}
	return all
}

// MenuBarStripStatuses returns job statuses for the menu-bar strip: finished retention
// (left), then in-flight work (active, blocker, pending dequeue), then FIFO queued/paused
// jobs (right)—matching a left-to-right “past → current → waiting” progress metaphor.
func (s *State) MenuBarStripStatuses() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.finished)
	if s.active != nil {
		n++
	}
	for _, j := range s.waitingBlocker {
		if j != nil && !j.Status.IsFinished() {
			n++
		}
	}
	for _, j := range s.pendingDequeued {
		if j != nil && !j.Status.IsFinished() {
			n++
		}
	}
	for _, j := range s.queue.AllJobs() {
		if j != nil && !j.Status.IsFinished() {
			n++
		}
	}
	out := make([]string, 0, n)
	for _, j := range s.finished {
		if j != nil {
			out = append(out, string(j.Status))
		}
	}
	if s.active != nil {
		out = append(out, string(s.active.Status))
	}
	for _, j := range s.waitingBlocker {
		if j != nil && !j.Status.IsFinished() {
			out = append(out, string(j.Status))
		}
	}
	for _, j := range s.pendingDequeued {
		if j != nil && !j.Status.IsFinished() {
			out = append(out, string(j.Status))
		}
	}
	for _, j := range s.queue.AllJobs() {
		if j == nil || j.Status.IsFinished() {
			continue
		}
		out = append(out, string(j.Status))
	}
	return out
}

// HasNonFinishedJob reports whether any known job is not in a terminal state.
func (s *State) HasNonFinishedJob() bool {
	for _, j := range s.AllJobs() {
		if j != nil && !j.Status.IsFinished() {
			return true
		}
	}
	return false
}

// Snapshot returns a copy of all jobs (queued + active) for rendering.
func (s *State) Snapshot() []Job {
	all := s.AllJobs()
	result := make([]Job, len(all))
	for i, j := range all {
		result[i] = *j
	}
	return result
}

// ApplyEvent merges a worker event into the state, updating the queue and active job.
// This is called from the app event loop, not from the worker, to keep goroutine
// access safe.
func (s *State) ApplyEvent(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch ev.Type {
	case EventStarted:
		job := s.findJobUnlocked(ev.JobID)
		if job != nil {
			job.Status = StatusRunning
			if job.StartedAt.IsZero() {
				job.StartedAt = time.Now()
			}
			ResetProgressETA(job)
			s.active = job
		}
	case EventPlanTotals:
		job := s.findJobUnlocked(ev.JobID)
		if job != nil {
			job.TotalFiles = ev.TotalFiles
			job.TotalBytes = ev.TotalBytes
		}
	case EventJobBlockerRequest:
		job := s.findJobUnlocked(ev.JobID)
		if job != nil {
			job.Status = StatusWaitingDecision
			if ev.Blocker != nil {
				b := *ev.Blocker
				job.PendingBlocker = &b
			}
		}
	case EventProgress:
		j := s.findJobUnlocked(ev.JobID)
		if j != nil {
			j.DoneFiles = ev.DoneFiles
			j.DoneBytes = ev.DoneBytes
			j.CurrentPath = ev.CurrentPath
			now := time.Now()
			s.advanceThroughputStrip(j, now, j.DoneBytes)
			ApplyProgressETA(j, ev.DoneBytes, ev.DoneFiles, now)
			j.PendingBlocker = nil
		}
	case EventCompleted:
		if s.active != nil && s.active.ID == ev.JobID {
			s.active.Status = StatusCompleted
			s.active.FinishedAt = time.Now()
			s.active.PendingBlocker = nil
			s.active = nil
		}
		if j := s.findJobUnlocked(ev.JobID); j != nil {
			j.Status = StatusCompleted
			j.PendingBlocker = nil
			if j.FinishedAt.IsZero() {
				j.FinishedAt = time.Now()
			}
		}
	case EventFailed:
		if s.active != nil && s.active.ID == ev.JobID {
			s.active.Status = StatusFailed
			s.active.Error = ev.Error
			s.active.FinishedAt = time.Now()
			s.active.PendingBlocker = nil
			s.active = nil
		}
		if j := s.findJobUnlocked(ev.JobID); j != nil {
			j.Status = StatusFailed
			j.PendingBlocker = nil
			if j.Error == "" && ev.Error != "" {
				j.Error = ev.Error
			}
			if j.FinishedAt.IsZero() {
				j.FinishedAt = time.Now()
			}
		}
	case EventCanceled:
		if s.active != nil && s.active.ID == ev.JobID {
			s.active.Status = StatusCanceled
			s.active.FinishedAt = time.Now()
			s.active.PendingBlocker = nil
			s.active = nil
		}
		s.removeWaitingBlockerUnlocked(ev.JobID)
		if job := s.findJobUnlocked(ev.JobID); job != nil {
			job.Status = StatusCanceled
			job.PendingBlocker = nil
			if job.FinishedAt.IsZero() {
				job.FinishedAt = time.Now()
			}
		}
	}
}

// ApplyRetention trims or clears finished-job history according to policy.
func (s *State) ApplyRetention(p RetentionPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !p.ShowFinished {
		s.finished = nil
		return
	}
	if p.KeepFinished > 0 && len(s.finished) > p.KeepFinished {
		s.finished = s.finished[len(s.finished)-p.KeepFinished:]
	}
}

// ClearFinishedArchive removes all finished jobs from the UI history slice.
func (s *State) ClearFinishedArchive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = nil
}

// SetTransferFunc sets the copy/move function used by the worker.
func (s *State) SetTransferFunc(fn func(ctx context.Context, job *Job, emit func(Event), waitBlocker func(BlockerRequest) ConflictDecision) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TransferFunc = fn
}

// SubmitBlockerDecision delivers the user's choice for a job blocked on conflict or disk space.
func (s *State) SubmitBlockerDecision(jobID string, d ConflictDecision) {
	s.blockerRegMu.Lock()
	ch := s.blockerWait[jobID]
	s.blockerRegMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- d:
	default:
	}
}

// SubmitConflictDecision is an alias for SubmitBlockerDecision (conflict outcomes use the same channel).
func (s *State) SubmitConflictDecision(jobID string, d ConflictDecision) {
	s.SubmitBlockerDecision(jobID, d)
}

// CancelJob requests cancellation of a queued or running job. Returns true if the job was found.
func (s *State) CancelJob(id string) bool {
	s.mu.Lock()
	if s.active != nil && s.active.ID == id {
		if s.cancelRun != nil {
			cancel := s.cancelRun
			s.mu.Unlock()
			cancel()
			return true
		}
		s.mu.Unlock()
		return true
	}
	for i, pj := range s.pendingDequeued {
		if pj != nil && pj.ID == id {
			job := pj
			s.pendingDequeued = append(s.pendingDequeued[:i], s.pendingDequeued[i+1:]...)
			job.Status = StatusCanceled
			job.FinishedAt = time.Now()
			s.mu.Unlock()
			s.emit(Event{
				Type:   EventCanceled,
				JobID:  id,
				Status: StatusCanceled,
			})
			return true
		}
	}
	if s.removeWaitingBlockerUnlocked(id) {
		s.mu.Unlock()
		s.SubmitBlockerDecision(id, DecisionCancel)
		return true
	}
	canceled := s.queue.CancelQueuedJobByID(id)
	s.mu.Unlock()
	if canceled {
		s.emit(Event{
			Type:   EventCanceled,
			JobID:  id,
			Status: StatusCanceled,
		})
	}
	return canceled
}

// StartWorker launches the background worker goroutine. It returns immediately.
// The worker dequeues jobs and runs each on its own goroutine; only one transfer holds the lease at a time.
func (s *State) StartWorker(stop <-chan struct{}) {
	go s.runWorker(stop)
}

func (s *State) runWorker(stop <-chan struct{}) {
	for {
		job := s.dequeueJob()
		if job == nil {
			select {
			case <-stop:
				s.workerShutdown()
				return
			case <-s.wake:
			}
			continue
		}
		go s.runJob(job, stop)
	}
}

func (s *State) runJob(job *Job, stop <-chan struct{}) {
	var policy ConflictPolicy
	waitBlocker := func(req BlockerRequest) ConflictDecision {
		if req.Kind == BlockerKindConflict && req.Conflict != nil {
			if policy.Decision() != "" {
				return policy.Decision()
			}
		}

		details := BlockerDetailsFromRequest(req)
		jobSnap := *details
		emitSnap := *details
		ch := make(chan ConflictDecision, 1)
		s.blockerRegMu.Lock()
		s.blockerWait[job.ID] = ch
		s.blockerRegMu.Unlock()

		s.mu.Lock()
		if s.active == job {
			s.active = nil
		}
		job.Status = StatusWaitingDecision
		job.PendingBlocker = &jobSnap
		s.appendWaitingBlockerUnlocked(job)
		s.mu.Unlock()

		s.emit(Event{
			Type:    EventJobBlockerRequest,
			JobID:   job.ID,
			Status:  StatusWaitingDecision,
			Blocker: &emitSnap,
		})

		s.transferLease.Unlock()
		var d ConflictDecision
		select {
		case d = <-ch:
		case <-stop:
			d = DecisionCancel
		}
		s.transferLease.Lock()

		s.blockerRegMu.Lock()
		delete(s.blockerWait, job.ID)
		s.blockerRegMu.Unlock()

		s.mu.Lock()
		s.removeWaitingBlockerUnlocked(job.ID)
		job.PendingBlocker = nil
		s.active = job
		job.Status = StatusRunning
		s.mu.Unlock()

		if req.Kind == BlockerKindConflict {
			_, _, _, policy = ApplyDecision(policy, d)
		}
		return d
	}

	s.transferLease.Lock()
	defer s.transferLease.Unlock()

	jobCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	job.Status = StatusRunning
	s.removePendingDequeuedByID(job.ID)
	s.active = job
	s.cancelRun = cancel
	s.mu.Unlock()

	s.emit(Event{
		Type:   EventStarted,
		JobID:  job.ID,
		Status: StatusRunning,
	})

	var transferErr error
	if s.TransferFunc != nil {
		transferErr = s.TransferFunc(jobCtx, job, s.emit, waitBlocker)
	}

	cancel()

	switch {
	case errors.Is(transferErr, context.Canceled):
		job.Status = StatusCanceled
	case transferErr != nil:
		job.Status = StatusFailed
		job.Error = transferErr.Error()
	default:
		job.Status = StatusCompleted
	}
	job.FinishedAt = time.Now()

	s.mu.Lock()
	s.cancelRun = nil
	if s.active != nil && s.active.ID == job.ID {
		s.finished = append(s.finished, job)
	}
	s.active = nil
	job.PendingBlocker = nil
	s.mu.Unlock()

	switch {
	case errors.Is(transferErr, context.Canceled):
		s.emit(Event{
			Type:   EventCanceled,
			JobID:  job.ID,
			Status: StatusCanceled,
		})
	case transferErr != nil:
		s.emit(Event{
			Type:   EventFailed,
			JobID:  job.ID,
			Status: StatusFailed,
			Error:  transferErr.Error(),
			Err:    transferErr,
		})
	default:
		s.emit(Event{
			Type:   EventCompleted,
			JobID:  job.ID,
			Status: StatusCompleted,
		})
	}
}

func (s *State) workerShutdown() {
	s.mu.Lock()
	cancel := s.cancelRun
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.mu.Lock()
	for _, j := range s.waitingBlocker {
		if j != nil {
			j.Status = StatusCanceled
		}
	}
	s.waitingBlocker = nil
	for _, j := range s.pendingDequeued {
		if j != nil && !j.Status.IsFinished() {
			j.Status = StatusCanceled
		}
	}
	s.pendingDequeued = nil
	if s.active != nil && !s.active.Status.IsFinished() {
		s.active.Status = StatusCanceled
	}
	s.mu.Unlock()
}

// QueueTestEvent enqueues ev on the events channel (for tests).
func (s *State) QueueTestEvent(ev Event) {
	s.emit(ev)
}

func (s *State) emit(ev Event) {
	sent := false
	select {
	case s.events <- ev:
		sent = true
	default:
		// Channel full; drop event to avoid blocking. This should not happen
		// with proper event consumption in the app event loop.
	}
	if !sent {
		return
	}
	s.emitMu.RLock()
	hook := s.emitHook
	s.emitMu.RUnlock()
	if hook != nil {
		hook(ev)
	}
}

func (s *State) findJobUnlocked(id string) *Job {
	for _, job := range s.queue.AllJobs() {
		if job.ID == id {
			return job
		}
	}
	for _, j := range s.pendingDequeued {
		if j != nil && j.ID == id {
			return j
		}
	}
	if s.active != nil && s.active.ID == id {
		return s.active
	}
	for _, j := range s.waitingBlocker {
		if j != nil && j.ID == id {
			return j
		}
	}
	for _, j := range s.finished {
		if j != nil && j.ID == id {
			return j
		}
	}
	return nil
}

func (s *State) appendWaitingBlockerUnlocked(job *Job) {
	for _, j := range s.waitingBlocker {
		if j != nil && j.ID == job.ID {
			return
		}
	}
	s.waitingBlocker = append(s.waitingBlocker, job)
}

func (s *State) removeWaitingBlockerUnlocked(id string) bool {
	for i, j := range s.waitingBlocker {
		if j != nil && j.ID == id {
			s.waitingBlocker = append(s.waitingBlocker[:i], s.waitingBlocker[i+1:]...)
			return true
		}
	}
	return false
}
