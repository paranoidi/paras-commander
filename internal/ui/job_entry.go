package ui

import (
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/oplog"
)

// JobEntry is a renderable job summary for the jobs view.
type JobEntry struct {
	ID          string
	Type        string
	Status      string
	Sources     []string
	Destination string
	// DestIsDir matches jobs.Job.DestIsDir (destination was a directory when the job was queued).
	DestIsDir   bool
	CurrentPath string
	// CurrentDestPath mirrors jobs.Job.CurrentDestPath.
	CurrentDestPath string
	// VerifyDone and VerifyTotal mirror jobs.Job: a running conflict content verification of
	// CurrentPath (VerifyTotal 0 = none).
	VerifyDone, VerifyTotal int64
	DoneFiles               int
	// ProcessedItems mirrors jobs.Job.ProcessedItems (a copy's files gone through).
	ProcessedItems int
	TotalFiles     int
	TotalDirs      int
	DoneBytes      int64
	TotalBytes     int64
	Error          string
	// Warnings mirrors jobs.Job.Warnings: non-fatal per-item issues (e.g. a symlink relinked
	// instead of dereferenced) that do not fail the job.
	Warnings []string
	// FailedItems mirrors jobs.Job.FailedItems: source items a copy could not copy (retry plan).
	FailedItems []jobs.FailedItem
	// Ops is the job's operation log (the Operations pane reads it through the jobs handler);
	// OpsStats is its counters at list sync (the Details Files line).
	Ops      *oplog.Log
	OpsStats oplog.Stats
	// SourceStats and SourcePhases are the log's per-source counters and the executor's
	// per-source progress at list sync (Queue source sub-rows; nil for a single-source job).
	SourceStats  []oplog.Stats
	SourcePhases []jobs.SourcePhase
	// RemovingSources matches jobs.Job.RemovingSources (a move removing its copied sources).
	RemovingSources bool
	StartedAt       time.Time
	FinishedAt      time.Time
	// Undo is jobs.Job.UndoLabel: whether the job can be undone, and why not.
	Undo string
	// ETABytesPerSec is smoothed throughput from recent progress samples (bytes/s).
	ETABytesPerSec float64
	// ETAFilesPerSec is smoothed completion rate from recent progress samples (files/s).
	ETAFilesPerSec float64
	// DisplaySpeedBPS is slower-smoothed B/s for the Queue Speed column.
	DisplaySpeedBPS float64
	// QueueETA is cumulative ETA including prior unfinished jobs (from jobs.ComputeQueueETAs).
	QueueETA string
	// ThroughputStrip is a snapshot of fixed-clock B/s columns for the details chart (oldest left).
	ThroughputStrip []float64
	// PendingBlocker is set when the job waits on a conflict or disk-space prompt.
	PendingBlocker *jobs.BlockerDetails
	// TotalsComplete matches jobs.Job.TotalsComplete: true once TotalFiles/TotalDirs/TotalBytes
	// are final — the counting walk has finished. False (for a copy/move/flatten job) means
	// those totals are still growing in the background even though the transfer itself may
	// already be running.
	TotalsComplete bool
	// EstimatedBytes matches jobs.Job.EstimatedBytes (enqueue-time size estimate, 0 = none).
	EstimatedBytes int64
}
