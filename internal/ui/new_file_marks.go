package ui

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TopLevelDestNamesFromJob returns the listing directory and base names of immediate
// children created at the job destination (top-level sources only; plan is not walked).
func TopLevelDestNamesFromJob(j *jobs.Job) (destDir pathloc.Path, names []string, ok bool) {
	if j == nil || j.Status != jobs.StatusCompleted {
		return pathloc.Path{}, nil, false
	}
	switch j.Type {
	case jobs.TypeCopy, jobs.TypeMove, jobs.TypeFlatten:
	default:
		return pathloc.Path{}, nil, false
	}
	destDir = j.Destination
	if !j.DestIsDir {
		destDir = j.Destination.Parent()
	}
	if destDir.IsZero() {
		return pathloc.Path{}, nil, false
	}
	var nameRoot pathloc.Path
	if j.Type != jobs.TypeFlatten {
		nameRoot = ops.TransferNameRoot(j.Sources)
	}
	seen := make(map[string]struct{})
	for _, src := range j.Sources {
		base := j.Destination.Base()
		if j.DestIsDir {
			// Nested batch names create a directory at the destination; mark its first segment.
			base = ops.TransferDestName(src, nameRoot)
			if i := strings.IndexAny(base, `/\`); i >= 0 {
				base = base[:i]
			}
		}
		if base == "" || base == "." {
			continue
		}
		if _, dup := seen[base]; dup {
			continue
		}
		seen[base] = struct{}{}
		names = append(names, base)
	}
	if len(names) == 0 {
		return destDir, nil, false
	}
	return destDir, names, true
}

// ApplyNewFileMarksFromJob adds top-level destination marks from a completed transfer job.
func ApplyNewFileMarksFromJob(s *panel.State, j *jobs.Job) {
	destDir, names, ok := TopLevelDestNamesFromJob(j)
	if !ok {
		return
	}
	s.AddNewFileMarks(destDir, names)
}
