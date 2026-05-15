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
	TypeCopy   Type = "copy"
	TypeMove   Type = "move"
	TypeDelete Type = "delete"
)

// Status represents the lifecycle state of a Job.
type Status string

const (
	StatusQueued          Status = "queued"
	StatusPaused          Status = "paused"
	StatusRunning         Status = "running"
	StatusWaitingDecision Status = "decision"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
	StatusCanceled        Status = "canceled"
)

// IsFinished reports whether the job has reached a terminal state.
func (s Status) IsFinished() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCanceled
}

// Job represents a single background filesystem operation.
type Job struct {
	ID          string
	Type        Type
	Status      Status
	Sources     []string
	Destination string
	// DestIsDir is whether Destination was an existing directory at enqueue time (same as ops.ResolveDestination Stat semantics).
	// Used by UI path marks so listing render does not Stat the destination per row.
	DestIsDir   bool
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
	// ETAFilesPerSec is an EMA-smoothed recent completion rate (files/s) for ETA display.
	ETAFilesPerSec float64
	// DisplaySpeedBPS is a slower EMA for queue throughput column display (bytes/s).
	DisplaySpeedBPS float64
	// ThroughputStrip holds one B/s sample per completed wall-clock bin (oldest index 0, newest appended).
	ThroughputStrip []float64
	// ThroughputStripOpenBin is Unix-nano-aligned start of the open bin (valid when throughputStripOpenSet).
	ThroughputStripOpenBin int64
	// throughputStripOpenSet is true after AdvanceJobThroughputStrip anchors the open bin once.
	throughputStripOpenSet bool
	// ThroughputStripDoneAtOpen is DoneBytes when the current open bin started.
	ThroughputStripDoneAtOpen int64
	// LastProgressSnapshotAt, LastProgressDoneBytes, and LastProgressDoneFiles sample progress for ETA smoothing.
	LastProgressSnapshotAt time.Time
	LastProgressDoneBytes  int64
	LastProgressDoneFiles  int

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
