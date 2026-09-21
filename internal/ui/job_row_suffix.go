package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// jobRowSuffix resolves the <job><queued><operation> suffix icons for absPath, plus the matched
// job's status for Theme.PanelJobMarkStyle. Zero JobSuffix when no unfinished job matches. The
// job (HDD) icon is only set while the job is actually working on the tree (Status.IsActive);
// a queued or paused job shows just the clock/operation icons.
func jobRowSuffix(absPath string, jobMarks []JobPathMark, th theme.Theme) (panellist.JobSuffix, string) {
	m, ok := EntryPathJobMark(absPath, jobMarks)
	if !ok {
		return panellist.JobSuffix{}, ""
	}
	var s panellist.JobSuffix
	if jobs.Status(m.Status).IsActive() {
		s.JobIcon, s.JobWrite = th.IconFilelistJob(), m.Write
	}
	if jobs.Status(m.Status) == jobs.StatusQueued {
		s.JobQueuedIcon = th.IconFilelistQueued()
	}
	switch jobs.Type(m.Type) {
	case jobs.TypeMove, jobs.TypeFlatten:
		if !m.Write { // source rows only: the glyph says "this will vanish"
			s.JobOpIcon, s.JobOpStyle = th.IconFilelistMove(), th.PanelRowMarkJobMove
		}
	case jobs.TypeDelete:
		s.JobOpIcon, s.JobOpStyle = th.IconFilelistDelete(), th.PanelRowMarkJobDelete
	}
	return s, m.Status
}
