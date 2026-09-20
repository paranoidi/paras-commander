package theme

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Jobs list icon keys ([icons.jobs.list] in TOML — flattened as jobs.list.<status>).
const (
	IconKeyJobsListScanning  = "jobs.list.scanning"
	IconKeyJobsListQueued    = "jobs.list.queued"
	IconKeyJobsListRunning   = "jobs.list.running"
	IconKeyJobsListPaused    = "jobs.list.paused"
	IconKeyJobsListCanceled  = "jobs.list.canceled"
	IconKeyJobsListFailed    = "jobs.list.failed"
	IconKeyJobsListDecision  = "jobs.list.decision"
	IconKeyJobsListCompleted = "jobs.list.completed"
)

// IconJobsList returns the Nerd Font icon for a job status in the jobs list column.
func (t Theme) IconJobsList(status string) string {
	key := "jobs.list." + status
	if t.Icons != nil {
		if s := strings.TrimSpace(t.Icons[key]); s != "" {
			return s
		}
	}
	switch status {
	case "scanning":
		return "\uf110"
	case "queued":
		return "\u231B"
	case "running":
		return "\uf144"
	case "decision":
		return "\U000f02d7"
	case "paused":
		return "\uf28b"
	case "canceled":
		return "\uf28d"
	case "failed":
		return "\uf06a"
	case "completed":
		return "\uf05d"
	default:
		return " "
	}
}

// IconFilelistQueued returns the file-list row-suffix icon shown while a matched job is still
// queued (the same glyph as the jobs list's queued status icon, jobs.list.queued).
func (t Theme) IconFilelistQueued() rune {
	return t.filelistIconRune(IconKeyJobsListQueued, '\uf017')
}

// JobsIconStyle returns the themed style for the leading icon of a job status in the jobs list.
func (t Theme) JobsIconStyle(status string) tcell.Style {
	switch status {
	case "scanning":
		return t.JobsIconsScanning
	case "queued":
		return t.JobsIconsQueued
	case "running":
		return t.JobsIconsRunning
	case "paused":
		return t.JobsIconsPaused
	case "canceled":
		return t.JobsIconsCanceled
	case "failed":
		return t.JobsIconsFailed
	case "decision":
		return t.JobsIconsDecision
	case "completed":
		return t.JobsIconsCompleted
	default:
		return t.JobsRow
	}
}

// PanelJobMarkStyle returns the file-panel job mark style: write (green) /
// read (yellow), red while the matched job waits on a user decision.
func (t Theme) PanelJobMarkStyle(status string, write bool) tcell.Style {
	if status == "decision" {
		return t.PanelRowMarkJobDecision
	}
	if write {
		return t.PanelRowMarkJob
	}
	return t.PanelRowMarkJobRead
}

// JobsStatusStyle returns the themed style for the status column in the jobs list.
func (t Theme) JobsStatusStyle(status string) tcell.Style {
	switch status {
	case "running", "queued", "scanning", "paused", "decision":
		return t.JobsRunning
	case "completed":
		return t.JobsDone
	case "failed", "canceled":
		return t.JobsFailed
	default:
		return t.JobsRow
	}
}
