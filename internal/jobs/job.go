// Package jobs implements the background job queue, worker, and state management
// for filesystem operations (copy, move) in paras-commander.
package jobs

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Type identifies the kind of filesystem operation a Job represents.
type Type string

const (
	TypeCopy    Type = "copy"
	TypeMove    Type = "move"
	TypeFlatten Type = "flatten"
	TypeDelete  Type = "delete"
	TypeExtract Type = "extract"
)

// Status represents the lifecycle state of a Job.
type Status string

const (
	StatusScanning        Status = "scanning"
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
	Sources     []pathloc.Path
	Destination pathloc.Path
	// DestIsDir is whether Destination was an existing directory at enqueue time (same as ops.ResolveDestination Stat semantics).
	// Used by UI path marks so listing render does not Stat the destination per row.
	DestIsDir     bool
	TotalFiles    int
	TotalDirs     int
	DoneFiles     int
	TotalBytes    int64
	DoneBytes     int64
	CurrentPath   string
	Error         string
	ScanStartedAt time.Time
	StartedAt     time.Time
	FinishedAt    time.Time

	// Plan is the pre-built copy/move plan from pre-scan; nil until scan completes or for delete jobs.
	// Populated only as a synchronous-rebuild fallback (see jobbridge.TransferFunc) when the job
	// never went through PlanCh — the normal streaming pre-scan leaves this nil throughout.
	Plan []ops.PlanItem
	// PlanCh streams PlanItems from the background pre-scan producer (see jobs/scan.go) as they
	// are discovered, instead of jobbridge waiting for a fully-built Plan slice. Set once by
	// startJobScan before the job leaves StatusScanning; nil for delete/extract jobs and for any
	// copy/move/flatten job whose scan hasn't started yet.
	PlanCh <-chan ops.PlanItem
	// PlanErr, paired with PlanCh, retrieves the producer's terminal walk error once PlanCh is
	// observed closed (nil on a clean end). Safe to call from any goroutine at any time — it is
	// backed by the producer's own synchronization, not by job.mu (there is none) — which is why
	// PlanErr is a func rather than a plain field: unlike PlanCh (written once, before the job
	// leaves StatusScanning, then never touched again), the producer may still be running when a
	// consumer wants this value, so a plain field read here would race the scan goroutine.
	PlanErr func() error
	// PlanComplete is true once the PlanCh producer has finished enumerating the whole source
	// tree (success or failure) and written its final totals. Like TotalFiles/TotalDirs/
	// TotalBytes, it is only safe to read while holding jobs.State's lock (e.g. via AllJobs()/
	// Snapshot(), which copy the job under that lock) — the scan producer keeps writing it after
	// the job leaves StatusScanning, so jobbridge (which has no access to that lock) must not
	// read it directly; see jobbridge.TransferFunc's streamed-job dispatch.
	PlanComplete bool
	// PausedAfterScan when true transitions to StatusPaused instead of StatusQueued when pre-scan completes.
	PausedAfterScan bool

	// ETABytesPerSec is an EMA-smoothed recent transfer rate (bytes/s) for ETA display.
	ETABytesPerSec float64
	// ETAFilesPerSec is an EMA-smoothed recent completion rate (files/s) for ETA display.
	ETAFilesPerSec float64
	// DisplaySpeedBPS is a slower EMA for queue throughput column display (bytes/s).
	DisplaySpeedBPS float64
	// ThroughputStrip holds one B/s sample per completed chart column (oldest index 0, newest appended).
	ThroughputStrip []float64
	// ThroughputStripOpenBin is Unix-nano start of the open column (valid when throughputStripOpenSet).
	ThroughputStripOpenBin int64
	// throughputStripOpenSet is true after CloseOneThroughputColumn anchors the open column once.
	throughputStripOpenSet bool
	// ThroughputStripDoneAtOpen is DoneBytes when the current open bin started.
	ThroughputStripDoneAtOpen int64
	// LastProgressSnapshotAt, LastProgressDoneBytes, and LastProgressDoneFiles sample progress for ETA smoothing.
	LastProgressSnapshotAt time.Time
	LastProgressDoneBytes  int64
	LastProgressDoneFiles  int

	// PendingBlocker is non-nil while the job waits for user resolution (conflict or disk space).
	PendingBlocker *BlockerDetails

	// VolumeDevs holds the deduplicated st_dev of each local source and the destination,
	// computed once at enqueue (AddJob). Volume-contention checks compare against these
	// cached IDs so the UI thread never re-stats job paths on a mount the job itself is
	// saturating (stat on a busy CIFS/NFS mount can block for seconds).
	VolumeDevs []uint64

	// FlattenRemoveEmpty enables post-move removal of empty directories under FlattenRoots.
	FlattenRemoveEmpty bool
	// FlattenRoots are the selected directory roots for flatten cleanup (TypeFlatten only).
	FlattenRoots []pathloc.Path

	// DeleteRemoveEmptyDirs enables post-delete removal of directories left empty
	// under the parent directories of Sources (TypeDelete only).
	DeleteRemoveEmptyDirs bool

	// PromptDanglingDirs asks (after this move/delete job completes) whether to remove
	// directories left empty by the operation, via the normal delete confirmation dialog.
	// Set at enqueue from [operations].remove_dangling_directories; see
	// apphandler/jobs.Handler.promptDanglingDirsIfAny.
	PromptDanglingDirs bool

	// Per-job copy/move metadata options (from transfer dialog or config at enqueue).
	PreservePermissions bool
	PreserveTimestamps  bool
	// FlattenIntoDest requests dest/<basename> naming for a copy/move job (transfer-dialog
	// "Flatten into destination" checkbox), independent of TypeFlatten jobs.
	FlattenIntoDest bool
}

// FlatDestNames reports whether the job should resolve every source to dest/<basename>
// instead of preserving relative structure — true for TypeFlatten jobs and for copy/move
// jobs with FlattenIntoDest set. Single source of truth; do not re-derive job.Type == TypeFlatten.
func (j *Job) FlatDestNames() bool {
	return j.Type == TypeFlatten || j.FlattenIntoDest
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

// NeedsPreScan reports whether the job type requires a background plan walk before running.
func (j *Job) NeedsPreScan() bool {
	if j == nil {
		return false
	}
	return j.Type == TypeCopy || j.Type == TypeMove || j.Type == TypeFlatten
}
