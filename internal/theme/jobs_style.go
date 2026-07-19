package theme

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Jobs list symbol keys ([symbols.jobs.list] in TOML — flattened as jobs.list.<status>).
const (
	SymbolKeyJobsListScanning  = "jobs.list.scanning"
	SymbolKeyJobsListQueued    = "jobs.list.queued"
	SymbolKeyJobsListRunning   = "jobs.list.running"
	SymbolKeyJobsListPaused    = "jobs.list.paused"
	SymbolKeyJobsListCanceled  = "jobs.list.canceled"
	SymbolKeyJobsListFailed    = "jobs.list.failed"
	SymbolKeyJobsListDecision  = "jobs.list.decision"
	SymbolKeyJobsListCompleted = "jobs.list.completed"
)

// SymbolJobsList returns the Nerd Font glyph for a job status in the jobs list column.
func (t Theme) SymbolJobsList(status string) string {
	key := "jobs.list." + status
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
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

// JobsIconStyle returns the themed style for the leading icon of a job status in the jobs list.
func (t Theme) JobsIconStyle(status string) tcell.Style {
	switch status {
	case "scanning":
		return t.JobsIconsScanning
	case "queued":
		return t.JobsIconsQueued
	case "running":
		return t.JobsIconsOngoing
	case "paused":
		return t.JobsIconsPaused
	case "canceled":
		return t.JobsIconsStopped
	case "failed":
		return t.JobsIconsError
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
		return t.PanelRowIndicatorJobDecision
	}
	if write {
		return t.PanelRowIndicatorJob
	}
	return t.PanelRowIndicatorJobRead
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
