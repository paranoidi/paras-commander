package jobs

import (
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Host supplies cross-cutting app services the jobs handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.MessageHost
	host.PanelHost
	host.ShellHost

	RefreshBothPanels()
	RequestBothPanelsVolumeSpaceRefreshAsync()
	SetJobFailedTransientMessage(err error, fallback string)
	DevMode() bool
	// Config is the live configuration, for settings that change at runtime.
	Config() config.Config
	// DiskUsagePainter is the disk-usage cache the panels paint sizes from (nil when disabled);
	// AddTransferJob reads it for the enqueue-time size estimate.
	DiskUsagePainter() ui.DiskUsagePainter
	// PromptDanglingDirDelete opens (or, if another modal is already open, defers via a
	// transient message for) the delete-confirmation dialog for directories a completed
	// move/delete job left empty. dirs is non-empty.
	PromptDanglingDirDelete(dirs []string)
	// AdjustJobsThroughputChartColumnMS changes the graph tickrate by deltaMS at runtime (not persisted).
	AdjustJobsThroughputChartColumnMS(deltaMS int)
	// CopyToClipboard puts text on the system clipboard and toasts done (or a failure warning).
	CopyToClipboard(text, done string)
}
