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
	// ThroughputSamples is a snapshot of recent instantaneous B/s with timestamps (details chart).
	ThroughputSamples []jobs.ThroughputSample
	// PendingBlocker is set when the job waits on a conflict or disk-space prompt.
	PendingBlocker *jobs.BlockerDetails
}
