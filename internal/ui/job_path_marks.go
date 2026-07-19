package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// resolvedJobDestinationPath matches ops.ResolveDestination for a fixed dest-is-dir flag
// without calling Stat (see jobs.Job.DestIsDir at enqueue time).
// ponytail: basename-only approximation for in-flight glyphs; batch-relative names
// (ops.TransferDestName) would cost a common-root walk per row paint.
func resolvedJobDestinationPath(src, dest string, destIsDir bool) string {
	srcLoc, err1 := pathloc.Parse(src)
	destLoc, err2 := pathloc.Parse(dest)
	if err1 != nil || err2 != nil {
		return dest
	}
	return ops.ResolveDestination(srcLoc, destLoc).String()
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

// longestMatchingRootLen returns the maximum length of a clean path (source or
// resolved destination) that matches absPath as root-of-subtree, and whether that
// best match is a destination root (destination preferred on equal length), or
// (0, false) if none.
func longestMatchingRootLen(j JobPathMark, absPath string) (maxLen int, isDest bool) {
	for _, src := range j.Sources {
		if src == "" {
			continue
		}
		if pathloc.EqualOrUnderStrings(src, absPath) {
			if n := len(src); n > maxLen {
				maxLen = n
				isDest = false
			}
		}
		if j.Destination == "" {
			continue
		}
		dst := resolvedJobDestinationPath(src, j.Destination, j.DestIsDir)
		if dst != "" && dst != "." && pathloc.EqualOrUnderStrings(dst, absPath) {
			if n := len(dst); n >= maxLen {
				maxLen = n
				isDest = true
			}
		}
	}
	return maxLen, isDest
}

// EntryPathJobMarkStatus returns the status of the best non-finished job that
// affects absPath, whether any such job was found, and whether the matched role is
// a write (destination, or any delete-type job — deletes mutate their sources and
// have no destination). When several jobs match, delete is preferred over move over
// copy; among jobs of the same type, the match with the longest source/destination
// root wins; further ties keep the earliest job in jobList.
func EntryPathJobMarkStatus(absPath string, jobMarks []JobPathMark) (bool, string, bool) {
	if absPath == "" || len(jobMarks) == 0 {
		return false, "", false
	}
	p := absPath
	bestPri := -1
	bestLen := -1
	bestIdx := -1
	var bestStatus string
	var bestWrite bool
	for i, j := range jobMarks {
		if jobs.Status(j.Status).IsFinished() {
			continue
		}
		rootLen, isDest := longestMatchingRootLen(j, p)
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
			bestStatus = j.Status
			bestWrite = isDest || jobs.Type(j.Type) == jobs.TypeDelete
		}
	}
	if bestIdx < 0 {
		return false, "", false
	}
	return true, bestStatus, bestWrite
}

// EntryPathMarkedByJobs reports whether absPath is a source or destination tree root
// for any non-finished job in jobMarks (queued, running, or waiting on conflict).
func EntryPathMarkedByJobs(absPath string, jobMarks []JobPathMark) bool {
	marked, _, _ := EntryPathJobMarkStatus(absPath, jobMarks)
	return marked
}

// PanelTouchedByJobs reports whether panelPath overlaps any non-finished job source or destination tree.
func PanelTouchedByJobs(panelPath string, jobMarks []JobPathMark) bool {
	if panelPath == "" || len(jobMarks) == 0 {
		return false
	}
	p := panelPath
	for _, j := range jobMarks {
		if jobs.Status(j.Status).IsFinished() {
			continue
		}
		for _, src := range j.Sources {
			if src == "" {
				continue
			}
			if pathloc.TreesOverlapStrings(p, src) {
				return true
			}
			if j.Destination == "" {
				continue
			}
			dst := resolvedJobDestinationPath(src, j.Destination, j.DestIsDir)
			if dst != "" && dst != "." {
				if pathloc.TreesOverlapStrings(p, dst) {
					return true
				}
			}
		}
	}
	return false
}

// PanelInsideJobWriteTree reports whether panelPath is the destination dir of, or
// at-or-under a resolved destination root of, any non-finished job with a
// non-empty Destination; decision status wins over any other matched status.
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
		if !hit {
			for _, src := range j.Sources {
				if src == "" {
					continue
				}
				dst := resolvedJobDestinationPath(src, j.Destination, j.DestIsDir)
				if dst != "" && dst != "." && pathloc.EqualOrUnderStrings(dst, panelPath) {
					hit = true
					break
				}
			}
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
