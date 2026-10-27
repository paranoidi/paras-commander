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

// Jobs list expander icon keys ([icons.jobs.list]): appended to the Type of a job with more
// than one source, collapsed or expanded into its source sub-rows.
const (
	IconKeyJobsListSourcesCollapsed = "jobs.list.sources_collapsed"
	IconKeyJobsListSourcesExpanded  = "jobs.list.sources_expanded"
)

// IconJobsListSources returns the expander icon of a multi-source job row.
func (t Theme) IconJobsListSources(expanded bool) string {
	key, fallback := IconKeyJobsListSourcesCollapsed, "\uf196"
	if expanded {
		key, fallback = IconKeyJobsListSourcesExpanded, "\uf147"
	}
	if t.Icons != nil {
		if s := strings.TrimSpace(t.Icons[key]); s != "" {
			return s
		}
	}
	return fallback
}

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

// Operations pane result keys ([icons.jobs.operations] and [jobs.operations] keys).
const (
	JobsOperationSuccess = "success"
	JobsOperationFailed  = "failed"
	JobsOperationSkipped = "skipped"
	// JobsOperationSourceDeleted is an icon-only key: a skipped row whose conflict resolution
	// deleted the source. Its style is the skipped one.
	JobsOperationSourceDeleted = "source_deleted"
)

// IconJobsOperation returns the result icon of an Operations pane row for a result key
// (JobsOperationSuccess, ...), from [icons.jobs.operations], falling back to the jobs list's
// completed / failed icon or a built-in skipped / source deleted glyph.
func (t Theme) IconJobsOperation(result string) string {
	if t.Icons != nil {
		if s := strings.TrimSpace(t.Icons["jobs.operations."+result]); s != "" {
			return s
		}
	}
	switch result {
	case JobsOperationFailed:
		return t.IconJobsList("failed")
	case JobsOperationSkipped:
		return "\uf056"
	case JobsOperationSourceDeleted:
		return "\uf05c"
	}
	return t.IconJobsList("completed")
}

// JobsOperationStyle returns the style of a result key (Operations icons, Details counts).
func (t Theme) JobsOperationStyle(result string) tcell.Style {
	switch result {
	case JobsOperationFailed:
		return t.JobsOperationFailed
	case JobsOperationSkipped:
		return t.JobsOperationSkipped
	}
	return t.JobsOperationSuccess
}

// JobsOperationOn returns a result key's style foreground merged onto surface's background.
func (t Theme) JobsOperationOn(result string, surface tcell.Style) tcell.Style {
	return mergeForegroundOnSurface(t.JobsOperationStyle(result), surface)
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

// JobsProgressStyles returns progress-bar fill/track/label styles for the jobs list.
// Paused and canceled jobs use the inactive palette (grey fill instead of green).
func (t Theme) JobsProgressStyles(status string) (fill, track, labelOnFill, labelOnTrack tcell.Style) {
	switch status {
	case "paused", "canceled":
		return t.JobsProgressInactiveFill, t.JobsProgressInactiveTrack,
			t.JobsProgressInactiveLabelOnFill, t.JobsProgressInactiveLabelOnTrack
	default:
		return t.JobsProgressFill, t.JobsProgressTrack,
			t.JobsProgressLabelOnFill, t.JobsProgressLabelOnTrack
	}
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
