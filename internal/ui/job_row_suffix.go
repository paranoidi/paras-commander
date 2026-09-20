package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// jobRowSuffix resolves the <job><queued><operation> suffix icons for absPath, plus the matched
// job's status for Theme.PanelJobMarkStyle. Zero JobSuffix when no unfinished job matches.
func jobRowSuffix(absPath string, jobMarks []JobPathMark, th theme.Theme) (panellist.JobSuffix, string) {
	m, ok := EntryPathJobMark(absPath, jobMarks)
	if !ok {
		return panellist.JobSuffix{}, ""
	}
	s := panellist.JobSuffix{JobIcon: th.IconFilelistJob(), JobWrite: m.Write}
	if jobs.Status(m.Status) == jobs.StatusQueued {
		s.JobQueuedIcon = th.IconFilelistQueued()
	}
	switch jobs.Type(m.Type) {
	case jobs.TypeMove, jobs.TypeFlatten:
		s.JobOpIcon, s.JobOpStyle = th.IconFilelistMove(), th.PanelRowMarkJobMove
	case jobs.TypeDelete:
		s.JobOpIcon, s.JobOpStyle = th.IconFilelistDelete(), th.PanelRowMarkJobDelete
	}
	return s, m.Status
}
