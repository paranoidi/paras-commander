package ui

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

// JobQueuedMarkRune is appended after entry names when the path is affected by an unfinished copy/move job.
const JobQueuedMarkRune = '\uf04d'

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

// EntryPathMarkedByJobs reports whether absPath is a source or destination tree root
// for any non-finished job in jobList (queued, running, or waiting on conflict).
func EntryPathMarkedByJobs(absPath string, jobList []JobEntry) bool {
	if absPath == "" || len(jobList) == 0 {
		return false
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
				return true
			}
			if j.Destination == "" {
				continue
			}
			dst := filepath.Clean(ops.ResolveDestination(cs, j.Destination))
			if dst != "" && dst != "." && pathEqualOrUnder(dst, p) {
				return true
			}
		}
	}
	return false
}
