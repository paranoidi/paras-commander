package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// jobRowSuffix resolves the <job><queued><operation> suffix icons for absPath, plus the matched
// job's status for Theme.PanelJobMarkStyle. Zero JobSuffix when no unfinished job matches. The
// job (HDD) icon, the fileindex icon for a task job, is only set while the job is actually working on the tree (Status.IsActive);
// a queued or paused job shows just the clock/operation icons, and so does a source a running
// multi-source job has not started yet (JobRowMark.Pending; its status reads queued).
func jobRowSuffix(absPath string, jobMarks []JobPathMark, th theme.Theme) (panellist.JobSuffix, string) {
	m, ok := EntryPathJobMark(absPath, jobMarks)
	if !ok {
		return panellist.JobSuffix{}, ""
	}
	if m.Pending {
		m.Status = string(jobs.StatusQueued)
	}
	var s panellist.JobSuffix
	if jobs.Status(m.Status).IsActive() {
		s.JobIcon, s.JobWrite = th.IconFilelistJob(), m.Write
		if ic := []rune(th.IconFileIndex()); jobs.Type(m.Type) == jobs.TypeTask && len(ic) > 0 {
			s.JobIcon = ic[0] // every task job is a fileindex scan or prune
		}
	}
	if jobs.Status(m.Status) == jobs.StatusQueued {
		s.JobQueuedIcon = th.IconFilelistQueued()
	}
	switch jobs.Type(m.Type) {
	case jobs.TypeMove, jobs.TypeFlatten:
		switch {
		case m.Removing: // the copy is done and the source is being deleted now
			s.JobOpIcon, s.JobOpStyle = th.IconFilelistMoveRemoving(), th.PanelRowMarkJobRemoving
		case !m.Write: // source rows only: the glyph says "this will vanish"
			s.JobOpIcon, s.JobOpStyle = th.IconFilelistMove(), th.PanelRowMarkJobMove
		}
	case jobs.TypeDelete:
		s.JobOpIcon, s.JobOpStyle = th.IconFilelistDelete(), th.PanelRowMarkJobDelete
	}
	return s, m.Status
}
