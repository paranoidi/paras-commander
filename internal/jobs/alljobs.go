package jobs

// collectAllJobsUnlocked gathers job pointers from every ownership bucket in display order: in-flight buckets and the queue in run order, then finished jobs newest first.
// Caller must hold s.mu and s.queue.mu. The same job ID may appear in more than one bucket
// during transitions (e.g. active not cleared yet while entering waitingBlocker); use
// dedupeJobsByID before UI use.
func (s *State) collectAllJobsUnlocked() []*Job {
	var all []*Job
	for _, j := range s.active {
		if j != nil {
			all = append(all, j)
		}
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
	all = append(all, s.queue.allJobsUnlocked()...)
	for i := len(s.finished) - 1; i >= 0; i-- { // most recently finished first
		all = append(all, s.finished[i])
	}
	return all
}

// collectMenuBarStripJobsUnlocked gathers jobs for the menu-bar icon strip (finished first, then in-flight buckets).
// Caller must hold s.mu and s.queue.mu.
func (s *State) collectMenuBarStripJobsUnlocked() []*Job {
	var all []*Job
	for _, j := range s.finished {
		if j != nil {
			all = append(all, j)
		}
	}
	for _, j := range s.active {
		if j != nil {
			all = append(all, j)
		}
	}
	for _, j := range s.waitingBlocker {
		if j != nil && !j.Status.IsFinished() {
			all = append(all, j)
		}
	}
	for _, j := range s.pendingDequeued {
		if j != nil && !j.Status.IsFinished() {
			all = append(all, j)
		}
	}
	for _, j := range s.queue.allJobsUnlocked() {
		if j != nil && !j.Status.IsFinished() {
			all = append(all, j)
		}
	}
	return all
}

// dedupeJobsByID keeps the first occurrence of each job ID (bucket order in collectAllJobsUnlocked).
func dedupeJobsByID(jobs []*Job) []*Job {
	if len(jobs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(jobs))
	out := make([]*Job, 0, len(jobs))
	for _, j := range jobs {
		if j == nil {
			continue
		}
		if _, ok := seen[j.ID]; ok {
			continue
		}
		seen[j.ID] = struct{}{}
		out = append(out, j)
	}
	return out
}
