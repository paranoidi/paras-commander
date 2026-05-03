// Package jobs implements the background job queue, worker, and state management
// for filesystem operations (copy, move) in paras-commander.
package jobs

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Type identifies the kind of filesystem operation a Job represents.
type Type string

const (
	TypeCopy Type = "copy"
	TypeMove Type = "move"
)

// Status represents the lifecycle state of a Job.
type Status string

const (
	StatusQueued          Status = "queued"
	StatusPaused          Status = "paused"
	StatusRunning         Status = "running"
	StatusWaitingDecision Status = "waiting-decision"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
	StatusCanceled        Status = "canceled"
)

// IsFinished reports whether the job has reached a terminal state.
func (s Status) IsFinished() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCanceled
}

// ThroughputSample is one instantaneous transfer rate reading for the details panel chart.
type ThroughputSample struct {
	At  time.Time
	BPS float64
}

// Job represents a single background filesystem operation.
type Job struct {
	ID          string
	Type        Type
	Status      Status
	Sources     []string
	Destination string
	TotalFiles  int
	DoneFiles   int
	TotalBytes  int64
	DoneBytes   int64
	CurrentPath string
	Error       string
	StartedAt   time.Time
	FinishedAt  time.Time

	// ETABytesPerSec is an EMA-smoothed recent transfer rate (bytes/s) for ETA display.
	ETABytesPerSec float64
	// DisplaySpeedBPS is a slower EMA for queue throughput column display (bytes/s).
	DisplaySpeedBPS float64
	// ThroughputSamples holds recent instantaneous B/s samples with wall time for the details sparkline.
	ThroughputSamples []ThroughputSample
	// LastProgressSnapshotAt and LastProgressDoneBytes sample DoneBytes for ETA smoothing.
	LastProgressSnapshotAt time.Time
	LastProgressDoneBytes int64

	// PendingBlocker is non-nil while the job waits for user resolution (conflict or disk space).
	PendingBlocker *BlockerDetails
}

// jobIDCounter provides unique job IDs within a runtime session.
var jobIDCounter atomic.Uint64

// NewJobID generates a deterministic, unique job ID.
func NewJobID() string {
	n := jobIDCounter.Add(1)
	return fmt.Sprintf("job-%d", n)
}

// FinishedStatuses returns all terminal job statuses.
func FinishedStatuses() []Status {
	return []Status{StatusCompleted, StatusFailed, StatusCanceled}
}
