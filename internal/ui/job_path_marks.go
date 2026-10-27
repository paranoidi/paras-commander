package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// jobPathIndex answers "is this path at or under one of the job's source / resolved-destination
// roots" in O(path depth) instead of O(len(Sources)).
//
// EntryPathJobMarkStatus runs once per visible row on every frame, and every job progress event
// triggers a repaint: scanning the source list per row made a job built from a large multi-select
// (3000 sources x ~45 rows = 135k filepath.Rel calls per repaint) freeze the UI for the whole
// duration of the job. Matching walks the row path's own ancestors against these sets instead,
// so cost depends on path depth, never on how many files the job moves.
type jobPathIndex struct {
	sources    map[string]int32 // normalized source path -> source index
	sourceDirs []string         // deduplicated parents of sources
	dests      map[string]int32 // normalized per-source destination path -> source index (-1: shared)
}

// newJobPathIndex resolves each source's destination the same way ops.ResolveDestination does
// for a fixed dest-is-dir flag (jobs.Job.DestIsDir, decided by a single Stat at enqueue), so no
// filesystem call happens here or on the render path.
// ponytail: basename-only approximation for in-flight icons; batch-relative names
// (ops.TransferDestName) would need a common-root walk per source.
func newJobPathIndex(m JobPathMark) *jobPathIndex {
	idx := &jobPathIndex{
		sources: make(map[string]int32, len(m.Sources)),
		dests:   make(map[string]int32, len(m.Sources)),
	}
	var destLoc pathloc.Path
	destOK := false
	if m.Destination != "" {
		if loc, err := pathloc.Parse(m.Destination); err == nil {
			destLoc, destOK = loc, true
		}
	}
	seenDir := make(map[string]struct{})
	for i, src := range m.Sources {
		loc, err := pathloc.Parse(src)
		if err != nil {
			continue
		}
		s := loc.String()
		if s == "" || s == "." {
			continue
		}
		if _, dup := idx.sources[s]; !dup {
			idx.sources[s] = int32(i)
		}
		if dir := loc.Parent().String(); dir != "" && dir != "." {
			if _, seen := seenDir[dir]; !seen {
				seenDir[dir] = struct{}{}
				idx.sourceDirs = append(idx.sourceDirs, dir)
			}
		}
		if !destOK {
			continue
		}
		if !m.DestIsDir {
			idx.dests[destLoc.String()] = -1 // every source writes this one root
			continue
		}
		if child, err := destLoc.Join(loc.Base()); err == nil {
			if _, dup := idx.dests[child.String()]; !dup {
				idx.dests[child.String()] = int32(i)
			}
		}
	}
	return idx
}

// walkRoots visits absPath and then each of its ancestors, deepest first, so the first hit is
// always the longest matching root. It stops at the filesystem/scheme root.
func walkRoots(absPath string, hit func(path string) bool) {
	cur, err := pathloc.Parse(absPath)
	if err != nil {
		return
	}
	for {
		s := cur.String()
		if s == "" || s == "." || hit(s) {
			return
		}
		parent := cur.Parent()
		if parent.String() == s {
			return
		}
		cur = parent
	}
}

// rootMatch returns the length of the longest source or destination root containing absPath,
// whether that root is a destination (destinations win when both match at the same depth) and
// whether it is a pending source, or (0, false, false) when nothing matches. m's phases (nil
// marks every root active, JobPathMark.rootPhase) drop done roots and pending destinations; the
// walk then continues to the outer roots. While m is removing its sources every source root is
// active and no destination root matches.
func (idx *jobPathIndex) rootMatch(absPath string, m *JobPathMark) (maxLen int, isDest, pending bool) {
	removing := m.removing()
	walkRoots(absPath, func(s string) bool {
		if src, ok := idx.dests[s]; ok && !removing && m.rootPhase(src) == rootActive {
			maxLen, isDest = len(s), true
			return true
		}
		if src, ok := idx.sources[s]; ok {
			if removing {
				maxLen, isDest, pending = len(s), false, false
				return true
			}
			if p := m.rootPhase(src); p != rootDone {
				maxLen, isDest, pending = len(s), false, p == rootPending
				return true
			}
		}
		return false
	})
	return maxLen, isDest, pending
}

// destMatch reports whether absPath is at or under one of the resolved destination roots whose
// phase in m (nil: any) is active; none is while m is removing its sources.
func (idx *jobPathIndex) destMatch(absPath string, m *JobPathMark) bool {
	if m.removing() {
		return false
	}
	found := false
	walkRoots(absPath, func(s string) bool {
		src, ok := idx.dests[s]
		found = ok && m.rootPhase(src) == rootActive
		return found
	})
	return found
}

func jobTypeMarkPriority(t string) int {
	switch jobs.Type(t) {
	case jobs.TypeDelete:
		return 4
	case jobs.TypeExtract:
		return 3
	case jobs.TypeMove, jobs.TypeFlatten:
		return 2
	case jobs.TypeCopy:
		return 1
	default:
		return 0
	}
}

// longestMatchingRootLen returns the maximum length of a path (source or resolved
// destination) that matches absPath as root-of-subtree, whether that best match is a
// destination root (destination preferred on equal length) and whether it is a pending source
// (JobPathMark.rootPhase), or (0, false, false) if none.
func longestMatchingRootLen(j JobPathMark, absPath string) (maxLen int, isDest, pending bool) {
	return j.index().rootMatch(absPath, &j)
}

// JobRowMark is the best non-finished job matching a row's path, per EntryPathJobMark. Pending
// is set for a source the job has not started yet (a multi-source job working on another one);
// Removing for a source a move is deleting after copying it (JobPathMark.RemovingSources).
type JobRowMark struct {
	Status   string
	Type     string
	Write    bool
	Pending  bool
	Removing bool
}

// EntryPathJobMark returns the best non-finished job that affects absPath (whether any such
// job was found, and its status/type/write role). When several jobs match, delete is preferred
// over move over copy; among jobs of the same type, the match with the longest
// source/destination root wins; further ties keep the earliest job in jobMarks. Write is true
// when the matched role is a write (destination, or any delete-type job — deletes mutate their
// sources and have no destination).
func EntryPathJobMark(absPath string, jobMarks []JobPathMark) (JobRowMark, bool) {
	if absPath == "" || len(jobMarks) == 0 {
		return JobRowMark{}, false
	}
	p := absPath
	bestPri := -1
	bestLen := -1
	bestIdx := -1
	var best JobRowMark
	for i, j := range jobMarks {
		if jobs.Status(j.Status).IsFinished() {
			continue
		}
		rootLen, isDest, pending := longestMatchingRootLen(j, p)
		if rootLen == 0 {
			continue
		}
		pri := jobTypeMarkPriority(j.Type)
		if bestIdx < 0 ||
			pri > bestPri ||
			(pri == bestPri && rootLen > bestLen) {
			bestIdx = i
			bestPri = pri
			bestLen = rootLen
			best = JobRowMark{
				Status:   j.Status,
				Type:     j.Type,
				Write:    isDest || jobs.Type(j.Type) == jobs.TypeDelete,
				Pending:  pending,
				Removing: !isDest && j.removing(),
			}
		}
	}
	if bestIdx < 0 {
		return JobRowMark{}, false
	}
	return best, true
}

// EntryPathJobMarkStatus returns the status of the best non-finished job that
// affects absPath, whether any such job was found, and whether the matched role is
// a write (destination, or any delete-type job — deletes mutate their sources and
// have no destination). See EntryPathJobMark.
func EntryPathJobMarkStatus(absPath string, jobMarks []JobPathMark) (bool, string, bool) {
	m, ok := EntryPathJobMark(absPath, jobMarks)
	return ok, m.Status, m.Write
}

// EntryPathMarkedByJobs reports whether absPath is a source or destination tree root
// for any non-finished job in jobMarks (queued, running, or waiting on conflict).
func EntryPathMarkedByJobs(absPath string, jobMarks []JobPathMark) bool {
	marked, _, _ := EntryPathJobMarkStatus(absPath, jobMarks)
	return marked
}

// PanelTouchedByJobs reports whether panelPath overlaps any non-finished job source or destination tree.
//
// Overlap in the "panel is inside a job root" direction is the index's ancestor walk. The other
// direction — a source or destination living under panelPath — needs no per-source scan either:
// a source is under panelPath exactly when its parent directory is at or under panelPath, and
// every resolved destination is j.Destination itself or a direct child of it.
func PanelTouchedByJobs(panelPath string, jobMarks []JobPathMark) bool {
	if panelPath == "" || len(jobMarks) == 0 {
		return false
	}
	for _, j := range jobMarks {
		if jobs.Status(j.Status).IsFinished() || len(j.Sources) == 0 {
			continue
		}
		idx := j.index()
		if n, _, _ := idx.rootMatch(panelPath, nil); n > 0 {
			return true
		}
		for _, dir := range idx.sourceDirs {
			if pathloc.EqualOrUnderStrings(panelPath, dir) {
				return true
			}
		}
		if j.Destination != "" && pathloc.EqualOrUnderStrings(panelPath, j.Destination) {
			return true
		}
	}
	return false
}

// PanelInsideJobWriteTree reports whether panelPath is the destination dir of, or
// at-or-under a resolved destination root of, any non-finished job with a
// non-empty Destination; decision status wins over any other matched status. Of a multi-source
// job's resolved roots only the active source's counts (JobPathMark.rootPhase); the destination
// dir itself always does, since every source is written into it.
func PanelInsideJobWriteTree(panelPath string, jobMarks []JobPathMark) (bool, string) {
	if panelPath == "" || len(jobMarks) == 0 {
		return false, ""
	}
	panelLoc, err := pathloc.Parse(panelPath)
	if err != nil {
		return false, ""
	}
	marked := false
	var status string
	for _, j := range jobMarks {
		if jobs.Status(j.Status).IsFinished() || j.Destination == "" {
			continue
		}
		hit := false
		if j.DestIsDir {
			if destLoc, err := pathloc.Parse(j.Destination); err == nil && destLoc.String() == panelLoc.String() {
				hit = true
			}
		}
		// Every per-source resolved destination lives at-or-under j.Destination, so panelPath can
		// only match one if it is itself at-or-under j.Destination — skip the index walk otherwise.
		if !hit && pathloc.EqualOrUnderStrings(j.Destination, panelPath) {
			hit = j.index().destMatch(panelPath, &j)
		}
		if !hit {
			continue
		}
		if j.Status == string(jobs.StatusWaitingDecision) {
			return true, j.Status
		}
		if !marked {
			marked = true
			status = j.Status
		}
	}
	return marked, status
}

// EntryPathJobMarkStatusFromEntries is a test helper that maps JobEntry slices to path marks.
func EntryPathJobMarkStatusFromEntries(absPath string, jobList []JobEntry) (bool, string, bool) {
	return EntryPathJobMarkStatus(absPath, JobPathMarksFromEntries(jobList))
}

// EntryPathMarkedByJobsFromEntries is a test helper that maps JobEntry slices to path marks.
func EntryPathMarkedByJobsFromEntries(absPath string, jobList []JobEntry) bool {
	return EntryPathMarkedByJobs(absPath, JobPathMarksFromEntries(jobList))
}
