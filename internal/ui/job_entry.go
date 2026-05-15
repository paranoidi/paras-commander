package ui

import (
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
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
	DoneFiles   int
	TotalFiles  int
	DoneBytes   int64
	TotalBytes  int64
	Error       string
	StartedAt   time.Time
	FinishedAt  time.Time
	// ETABytesPerSec is smoothed throughput from recent progress samples (bytes/s).
	ETABytesPerSec float64
	// ETAFilesPerSec is smoothed completion rate from recent progress samples (files/s).
	ETAFilesPerSec float64
	// DisplaySpeedBPS is slower-smoothed B/s for the Queue Speed column.
	DisplaySpeedBPS float64
	// ThroughputStrip is a snapshot of fixed-clock B/s columns for the details chart (oldest left).
	ThroughputStrip []float64
	// PendingBlocker is set when the job waits on a conflict or disk-space prompt.
	PendingBlocker *jobs.BlockerDetails
}
