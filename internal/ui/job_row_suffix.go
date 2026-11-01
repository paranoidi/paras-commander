package ui

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// jobRowSuffix resolves the <job><queued><operation> suffix icons for absPath, plus the matched
// job's status for Theme.StatusStyle. Zero JobSuffix when no unfinished job matches. The
// job (HDD) icon, the fileindex scanning state icon for a task job, is only set while the job is
// actually working on the tree (Status.IsActive); a queued or paused job shows just the
// clock/operation icons, and so does a source a running multi-source job has not started yet
// (JobRowMark.Pending; its status reads queued). indexStateShown drops a task job's icon: the
// row's fileindex state column already shows scanning (indexRowState).
func jobRowSuffix(absPath string, jobMarks []JobPathMark, th theme.Theme, indexStateShown bool) (panellist.JobSuffix, string) {
	m, ok := EntryPathJobMark(absPath, jobMarks)
	if !ok {
		return panellist.JobSuffix{}, ""
	}
	if m.Pending {
		m.Status = string(jobs.StatusQueued)
	}
	var s panellist.JobSuffix
	if jobs.Status(m.Status).IsActive() {
		s.JobIcon = th.IconFilelistJob()
		if jobs.Type(m.Type) == jobs.TypeTask { // every task job is a fileindex scan or prune
			s.JobIcon = 0
			if !indexStateShown {
				s.JobIcon = th.IconFileIndexState(localfs.IndexStatusScanning)
			}
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
