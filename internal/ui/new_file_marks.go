package ui

import (
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
	seen := make(map[string]struct{})
	for _, src := range j.Sources {
		dst := ops.ResolveDestination(src, j.Destination)
		if !dst.Parent().Equal(destDir) {
			continue
		}
		base := dst.Base()
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
