package jobs

import (
	"context"
	"sync"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Task is the work of a TypeTask job: a function a built-in feature runs in the background
// with the job queue's progress, cancel and failure handling, but none of a transfer's
// pre-scan, conflicts, undo, retry or pause.
type Task struct {
	// Title names the task in the jobs view Details pane (e.g. "fileindex scan").
	Title string
	// Lease serializes tasks: one task per lease name runs at a time. It is independent of the
	// transfer lease, so a task runs beside copies and moves.
	Lease string
	// Run does the work. A context.Canceled (or ErrUserCanceled) return ends the job canceled,
	// any other error failed. r may be called from any goroutine.
	Run func(ctx context.Context, r TaskReporter) error
}

// TaskProgress is a task's progress: the same fields a transfer reports. TotalFiles 0 means
// unknown; it never shrinks once reported.
type TaskProgress struct {
	TotalFiles  int
	DoneFiles   int
	DoneBytes   int64
	CurrentPath string
}

// TaskReporter receives a task's progress and warnings. Implementations are safe for
// concurrent use.
type TaskReporter interface {
	// Progress reports the task's progress so far; samples are throttled to the jobs scan
	// progress interval, the last one is always delivered with the terminal event.
	Progress(TaskProgress)
	// Warn adds a non-fatal line to the job's Warnings.
	Warn(msg string)
	// StartSource marks source i (an index into Job.Sources) started and the one started
	// before it finished, so a task working through its sources one at a time clears their
	// file-list marks as it goes (Job.StartSource).
	StartSource(i int)
}

// NewTaskJob builds a queued TypeTask job for task. sources are the paths the task works on:
// they drive path marks and volume contention like a transfer's sources.
func NewTaskJob(task *Task, sources []pathloc.Path) *Job {
	return &Job{
		ID:      NewJobID(),
		Type:    TypeTask,
		Status:  StatusQueued,
		Sources: sources,
		Task:    task,
	}
}

// taskLeaseUnlocked returns the mutex for a task lease name, creating it. Caller must hold s.mu.
func (s *State) taskLeaseUnlocked(name string) *sync.Mutex {
	if s.taskLeases == nil {
		s.taskLeases = make(map[string]*sync.Mutex)
	}
	l := s.taskLeases[name]
	if l == nil {
		l = &sync.Mutex{}
		s.taskLeases[name] = l
	}
	return l
}

// taskReporter maps a task's reports onto job events and Job.Warnings.
type taskReporter struct {
	s   *State
	job *Job
	min time.Duration
	// ctx is the worker context: once it is done nothing drains Events(), and the blocking
	// totals event must not hold up workerShutdown.
	ctx context.Context

	mu       sync.Mutex
	last     TaskProgress
	lastEmit time.Time
	total    int // TotalFiles last sent as EventPlanTotals
}

func (r *taskReporter) Progress(p TaskProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.TotalFiles = max(p.TotalFiles, r.last.TotalFiles)
	r.last = p
	if now := time.Now(); now.Sub(r.lastEmit) >= r.min {
		r.lastEmit = now
		r.emitUnlocked()
	}
}

// emitUnlocked sends the latest sample: totals first when they grew, then progress. Caller
// holds r.mu, which also orders the events of concurrent reporters.
func (r *taskReporter) emitUnlocked() {
	if r.ctx != nil && r.ctx.Err() != nil {
		return
	}
	if r.last.TotalFiles > r.total {
		r.total = r.last.TotalFiles
		r.s.emit(Event{Type: EventPlanTotals, JobID: r.job.ID, Status: StatusRunning, TotalFiles: r.total})
	}
	r.s.emit(Event{
		Type:        EventProgress,
		JobID:       r.job.ID,
		Status:      StatusRunning,
		DoneFiles:   r.last.DoneFiles,
		DoneBytes:   r.last.DoneBytes,
		CurrentPath: r.last.CurrentPath,
	})
}

func (r *taskReporter) Warn(msg string) {
	r.s.mu.Lock()
	r.job.Warnings = append(r.job.Warnings, msg)
	r.s.mu.Unlock()
}

func (r *taskReporter) StartSource(i int) { r.job.StartSource(int32(i)) }

// finish flushes the last sample and returns its counts for the terminal event.
func (r *taskReporter) finish() (doneFiles int, doneBytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.last != (TaskProgress{}) {
		r.emitUnlocked()
	}
	return r.last.DoneFiles, r.last.DoneBytes
}

// runTask runs a TypeTask job's Task and returns its final counts and error.
func (s *State) runTask(ctx context.Context, job *Job) (doneFiles int, doneBytes int64, err error) {
	s.mu.Lock()
	r := &taskReporter{s: s, job: job, min: s.scanConfig.ProgressMinInterval, ctx: s.workerCtx}
	s.mu.Unlock()
	if job.Task != nil && job.Task.Run != nil {
		err = job.Task.Run(ctx, r)
	}
	doneFiles, doneBytes = r.finish()
	return doneFiles, doneBytes, err
}
