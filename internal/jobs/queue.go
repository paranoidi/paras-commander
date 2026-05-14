package jobs

import (
	"sync"
	"time"
)

// Queue is a thread-safe FIFO job queue.
type Queue struct {
	mu   sync.Mutex
	jobs []*Job
}

// NewQueue creates an empty job queue.
func NewQueue() *Queue {
	return &Queue{
		jobs: make([]*Job, 0),
	}
}

// Enqueue adds a job to the back of the queue.
func (q *Queue) Enqueue(job *Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
}

// Dequeue removes and returns the front job. Returns nil if empty.
func (q *Queue) Dequeue() *Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return nil
	}
	job := q.jobs[0]
	q.jobs = q.jobs[1:]
	return job
}

// DequeueRunnable removes and returns the first queued (non-paused) job, preserving order among runnable jobs.
// Paused jobs remain in the queue until resumed. Returns nil when there is no StatusQueued job.
func (q *Queue) DequeueRunnable() *Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, job := range q.jobs {
		if job != nil && job.Status == StatusQueued {
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			return job
		}
	}
	return nil
}

// PauseQueuedJob sets a queued job to StatusPaused. Returns false if the job is missing or not StatusQueued.
func (q *Queue) PauseQueuedJob(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range q.jobs {
		if job != nil && job.ID == id && job.Status == StatusQueued {
			job.Status = StatusPaused
			return true
		}
	}
	return false
}

// ResumePausedJob sets a queued paused job to StatusQueued so the worker can run it.
func (q *Queue) ResumePausedJob(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range q.jobs {
		if job != nil && job.ID == id && job.Status == StatusPaused {
			job.Status = StatusQueued
			return true
		}
	}
	return false
}

// Peek returns the front job without removing it. Returns nil if empty.
func (q *Queue) Peek() *Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		return nil
	}
	return q.jobs[0]
}

// CancelQueuedJobByID marks a queued or paused job as canceled without removing it.
// Returns false if the job is missing or already finished.
func (q *Queue) CancelQueuedJobByID(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, job := range q.jobs {
		if job == nil || job.ID != id {
			continue
		}
		if job.Status.IsFinished() {
			return false
		}
		if job.Status != StatusQueued && job.Status != StatusPaused {
			return false
		}
		job.Status = StatusCanceled
		job.FinishedAt = time.Now()
		return true
	}
	return false
}

// RemoveJobByID removes a queued job by ID. The job must be queued (not running).
// Returns true if a job was removed.
func (q *Queue) RemoveJobByID(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, job := range q.jobs {
		if job.ID == id {
			q.jobs = append(q.jobs[:i], q.jobs[i+1:]...)
			return true
		}
	}
	return false
}

// SwapQueued swaps two jobs in the queue by index (0-based). Both indices must be valid.
func (q *Queue) SwapQueued(i, j int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if i < 0 || j < 0 || i >= len(q.jobs) || j >= len(q.jobs) {
		return false
	}
	q.jobs[i], q.jobs[j] = q.jobs[j], q.jobs[i]
	return true
}

// Len returns the number of queued jobs.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// Jobs returns a snapshot of the queued jobs.
func (q *Queue) Jobs() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	snapshot := make([]*Job, len(q.jobs))
	copy(snapshot, q.jobs)
	return snapshot
}

// RemoveFinished removes all finished jobs from the queue.
func (q *Queue) RemoveFinished() {
	q.mu.Lock()
	defer q.mu.Unlock()
	filtered := make([]*Job, 0, len(q.jobs))
	for _, job := range q.jobs {
		if !job.Status.IsFinished() {
			filtered = append(filtered, job)
		}
	}
	q.jobs = filtered
}

// RetainLastN discards finished jobs beyond the last N retained, preserving
// non-finished jobs.
func (q *Queue) RetainLastN(n int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Count finished jobs.
	finished := 0
	for _, job := range q.jobs {
		if job.Status.IsFinished() {
			finished++
		}
	}
	if finished <= n {
		return
	}
	discard := finished - n

	filtered := make([]*Job, 0, len(q.jobs)-discard)
	for _, job := range q.jobs {
		if job.Status.IsFinished() && discard > 0 {
			discard--
			continue
		}
		filtered = append(filtered, job)
	}
	q.jobs = filtered
}

// ClearFinished removes finished jobs and returns them.
func (q *Queue) ClearFinished() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	var finished []*Job
	var remaining []*Job
	for _, job := range q.jobs {
		if job.Status.IsFinished() {
			finished = append(finished, job)
		} else {
			remaining = append(remaining, job)
		}
	}
	q.jobs = remaining
	return finished
}

// AllJobs returns a snapshot of all jobs (queued and any running).
func (q *Queue) AllJobs() []*Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	snapshot := make([]*Job, len(q.jobs))
	copy(snapshot, q.jobs)
	return snapshot
}
