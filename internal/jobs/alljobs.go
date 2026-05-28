package jobs

// collectAllJobsUnlocked gathers job pointers from every ownership bucket in display order.
// Caller must hold s.mu. The same job ID may appear in more than one bucket during races
// (e.g. active not cleared yet while entering waitingBlocker); use dedupeJobsByID before UI use.
func (s *State) collectAllJobsUnlocked() []*Job {
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

// collectMenuBarStripJobsUnlocked gathers jobs for the menu-bar glyph strip (finished first, then in-flight buckets).
// Caller must hold s.mu.
func (s *State) collectMenuBarStripJobsUnlocked() []*Job {
	var all []*Job
	for _, j := range s.finished {
		if j != nil {
			all = append(all, j)
		}
	}
	if s.active != nil {
		all = append(all, s.active)
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
	for _, j := range s.queue.AllJobs() {
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
