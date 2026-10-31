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
	current int32  // the started source, -1 before any
	version uint32 // bumped on every phase change (SourceVersion)
}

// newSourceProgress tracks n sources; nil for fewer than two (no per-source rows).
func newSourceProgress(n int) *sourceProgress {
	if n < 2 {
		return nil
	}
	return &sourceProgress{phases: make([]SourcePhase, n), current: -1}
}

// StartSource marks source i started and the previously started source finished (unless it
// was deferred back to queued): executors work on one source at a time. It is the
// ops.Options.OnSourceStart of the job.
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
	if p.current >= 0 && p.phases[p.current] == SourceStarted {
		p.phases[p.current] = SourceFinished
	}
	p.phases[i], p.current = SourceStarted, i
	p.version++
}

// DeferSource puts source i back to queued: the executor looked at it but set its work aside
// for later (a move source whose rename fell back to copy). The started-last index is kept,
// so ActiveSource still reports the job has started. It is the ops.Options.OnSourceDeferred of
// the job.
func (j *Job) DeferSource(i int32) {
	p := j.sourcePhases
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if i < 0 || int(i) >= len(p.phases) || p.phases[i] == SourceQueued {
		return
	}
	p.phases[i] = SourceQueued
	p.version++
}

// StartAllSources marks every source started: a task works on all its sources at once and
// finishes them one by one (FinishSource).
func (j *Job) StartAllSources() {
	p := j.sourcePhases
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.phases {
		p.phases[i] = SourceStarted
	}
	p.current = 0
	p.version++
}

// FinishSource marks source i finished.
func (j *Job) FinishSource(i int32) {
	p := j.sourcePhases
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if i < 0 || int(i) >= len(p.phases) || p.phases[i] == SourceFinished {
		return
	}
	p.phases[i] = SourceFinished
	p.version++
}

// SourceVersion changes whenever any source phase changes (0 for a single-source job). It is
// the cheap change check for SourcePhases.
func (j *Job) SourceVersion() uint32 {
	p := j.sourcePhases
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.version
}

// ActiveSource returns the source the executor started last, or -1 before any (always -1
// for a single-source job).
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
