package ui

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func pathEqualOrUnder(root, p string) bool {
	root = filepath.Clean(root)
	p = filepath.Clean(p)
	if root == "" || root == "." {
		return false
	}
	if p == root {
		return true
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolvedJobDestinationPath matches ops.ResolveDestination for a fixed dest-is-dir flag
// without calling Stat (see jobs.Job.DestIsDir at enqueue time).
func resolvedJobDestinationPath(src, dest string, destIsDir bool) string {
	d := filepath.Clean(dest)
	if destIsDir {
		return filepath.Clean(filepath.Join(d, filepath.Base(src)))
	}
	return d
}

func jobTypeMarkPriority(t string) int {
	switch jobs.Type(t) {
	case jobs.TypeDelete:
		return 3
	case jobs.TypeMove:
		return 2
	case jobs.TypeCopy:
		return 1
	default:
		return 0
	}
}

// longestMatchingRootLen returns the maximum length of a clean path (source or
// resolved destination) that matches absPath as root-of-subtree, or 0 if none.
func longestMatchingRootLen(j JobEntry, absPath string) int {
	p := filepath.Clean(absPath)
	maxLen := 0
	for _, src := range j.Sources {
		if src == "" {
			continue
		}
		cs := filepath.Clean(src)
		if pathEqualOrUnder(cs, p) {
			if n := len(cs); n > maxLen {
				maxLen = n
			}
		}
		if j.Destination == "" {
			continue
		}
		dst := resolvedJobDestinationPath(cs, j.Destination, j.DestIsDir)
		if dst != "" && dst != "." && pathEqualOrUnder(dst, p) {
			if n := len(dst); n > maxLen {
				maxLen = n
			}
		}
	}
	return maxLen
}

// EntryPathJobMarkStatus returns the status of the best non-finished job that
// affects absPath, and whether any such job was found. When several jobs match,
// delete is preferred over move over copy; among jobs of the same type, the
// match with the longest source/destination root wins; further ties keep the
// earliest job in jobList.
func EntryPathJobMarkStatus(absPath string, jobList []JobEntry) (bool, string) {
	if absPath == "" || len(jobList) == 0 {
		return false, ""
	}
	p := filepath.Clean(absPath)
	bestPri := -1
	bestLen := -1
	bestIdx := -1
	var bestStatus string
	for i, j := range jobList {
		if jobs.Status(j.Status).IsFinished() {
			continue
		}
		rootLen := longestMatchingRootLen(j, p)
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
		}
	}
	if bestIdx < 0 {
		return false, ""
	}
	return true, bestStatus
}

// EntryPathMarkedByJobs reports whether absPath is a source or destination tree root
// for any non-finished job in jobList (queued, running, or waiting on conflict).
func EntryPathMarkedByJobs(absPath string, jobList []JobEntry) bool {
	marked, _ := EntryPathJobMarkStatus(absPath, jobList)
	return marked
}
