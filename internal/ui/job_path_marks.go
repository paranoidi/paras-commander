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

// EntryPathJobMarkStatus returns the job status of the first non-finished job that
// affects absPath, and a bool indicating whether any such job was found.
func EntryPathJobMarkStatus(absPath string, jobList []JobEntry) (bool, string) {
	if absPath == "" || len(jobList) == 0 {
		return false, ""
	}
	p := filepath.Clean(absPath)
	for _, j := range jobList {
		if jobs.Status(j.Status).IsFinished() {
			continue
		}
		for _, src := range j.Sources {
			if src == "" {
				continue
			}
			cs := filepath.Clean(src)
			if pathEqualOrUnder(cs, p) {
				return true, j.Status
			}
			if j.Destination == "" {
				continue
			}
			dst := resolvedJobDestinationPath(cs, j.Destination, j.DestIsDir)
			if dst != "" && dst != "." && pathEqualOrUnder(dst, p) {
				return true, j.Status
			}
		}
	}
	return false, ""
}

// EntryPathMarkedByJobs reports whether absPath is a source or destination tree root
// for any non-finished job in jobList (queued, running, or waiting on conflict).
func EntryPathMarkedByJobs(absPath string, jobList []JobEntry) bool {
	marked, _ := EntryPathJobMarkStatus(absPath, jobList)
	return marked
}
