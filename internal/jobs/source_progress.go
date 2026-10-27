package jobs

import "sync"

// SourcePhase is how far a job's executor got with one of its sources.
type SourcePhase uint8

const (
	SourceQueued SourcePhase = iota
	SourceStarted
	SourceFinished
)

// sourceProgress is the phase of each source of a multi-source job, advanced by the executor
// (ops.Options.OnSourceStart) on the job goroutine and read by the UI at list sync.
type sourceProgress struct {
	mu      sync.Mutex
	phases  []SourcePhase
	current int32 // the started source, -1 before any
}

// newSourceProgress tracks n sources; nil for fewer than two (no per-source rows).
func newSourceProgress(n int) *sourceProgress {
	if n < 2 {
		return nil
	}
	return &sourceProgress{phases: make([]SourcePhase, n), current: -1}
}

// StartSource marks source i started and the previously started source finished: executors
// work on one source at a time. It is the ops.Options.OnSourceStart of the job.
func (j *Job) StartSource(i int32) {
	p := j.sourcePhases
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if i < 0 || int(i) >= len(p.phases) || i == p.current {
		return
	}
	if p.current >= 0 {
		p.phases[p.current] = SourceFinished
	}
	p.phases[i], p.current = SourceStarted, i
}

// ActiveSource returns the source the executor started last, or -1 before any (always -1
// for a single-source job). It is the cheap change check for SourcePhases.
func (j *Job) ActiveSource() int {
	p := j.sourcePhases
	if p == nil {
		return -1
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return int(p.current)
}

// SourcePhases returns a copy of each source's phase (nil for a single-source job).
func (j *Job) SourcePhases() []SourcePhase {
	p := j.sourcePhases
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]SourcePhase(nil), p.phases...)
}
