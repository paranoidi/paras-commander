// Package jobs implements the background job queue, worker, and state management
// for filesystem operations (copy, move) in paras-commander.
package jobs

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/oplog"
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
	// TypeRename is a finished rename or mass rename recorded for the jobs view; it never runs.
	TypeRename Type = "rename"
	// TypeUndo reverses the journal of another job (UndoOf).
	TypeUndo Type = "undo"
)

// CanceledCount is how well CanceledItems can count what a canceled job left undone.
type CanceledCount int

const (
	CanceledKnown       CanceledCount = iota
	CanceledUnknown                   // no finished file count: a copy canceled mid-scan, any move
	CanceledUnsupported               // flatten: its plan has no per-item total
)

// CanceledItems is how many items a canceled job of type t left undone, from its Job.TotalFiles,
// TotalDirs, TotalsComplete and ProcessedItems and the rows its operation log recorded. A copy
// counts files only, like its log rows: TotalFiles minus TotalDirs minus processed (rows miss
// the files under a skipped directory); delete, extract and undo count their items at enqueue
// and log one row each. A move never runs the counting walk, so its remainder is unknown.
func CanceledItems(t Type, totalFiles, totalDirs int, totalsComplete bool, processed, recorded int) (n int, count CanceledCount) {
	switch t {
	case TypeCopy:
		if !totalsComplete {
			return 0, CanceledUnknown
		}
		return max(0, totalFiles-totalDirs-processed), CanceledKnown
	case TypeMove:
		return 0, CanceledUnknown
	case TypeDelete, TypeExtract, TypeUndo:
		return max(0, totalFiles-recorded), CanceledKnown
	}
	return 0, CanceledUnsupported
}

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

// IsRetryable reports whether State.RetryJob can re-queue a job in this status.
func (s Status) IsRetryable() bool {
	return s == StatusFailed || s == StatusCanceled
}

// IsActive reports whether a worker is currently touching the job's trees: pre-scanning,
// transferring, or mid-transfer blocked on a user decision. Queued and paused jobs are not active.
func (s Status) IsActive() bool {
	return s == StatusScanning || s == StatusRunning || s == StatusWaitingDecision
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
	DestIsDir  bool
	TotalFiles int
	TotalDirs  int
	DoneFiles  int
	// ProcessedItems is how many non-directory plan items a copy went through (done, skipped or
	// failed), in TotalFiles-TotalDirs units; set when the copy's item loop ends.
	ProcessedItems int
	TotalBytes     int64
	DoneBytes      int64
	// EstimatedBytes is the enqueue-time best guess of the payload size, taken from what the
	// panel already knows (listing sizes and the disk-usage cache; see ui.SourceSizeEstimates).
	// 0 means no estimate. A copy shows it as TotalBytes until its counting walk finishes
	// (runJobScan); a move uses it as its byte total. Never changes after enqueue.
	EstimatedBytes int64
	// SourceSizeHints is the per-source estimate aligned with Sources (-1 = unknown), set only for
	// a move whose every source has an estimate: a rename credits its source's hint as done bytes
	// (ops.Options.SourceSizeHints). Never changes after enqueue.
	SourceSizeHints []int64
	CurrentPath     string
	// VerifyDone and VerifyTotal track a running conflict content verification of CurrentPath
	// (bytes read from both files); VerifyTotal is 0 when none runs. Never part of DoneBytes.
	VerifyDone, VerifyTotal int64
	Error                   string
	// Warnings collects non-fatal per-item issues encountered while building the transfer
	// plan (currently: a symlink that could not be safely dereferenced and was relinked
	// instead — see DereferenceSymlinks). Unlike Error, warnings never fail the job.
	Warnings []string
	// FailedItems lists the source items a copy skipped and carried on without (unsupported file
	// type, unreadable or vanished source file). Written only by State.ApplyEvent
	// (EventItemFailed). A job with failed items ends StatusFailed ("N item(s) failed"); Retry
	// re-runs just these items (see RetryClone).
	FailedItems   []FailedItem
	ScanStartedAt time.Time
	StartedAt     time.Time
	FinishedAt    time.Time

	// Plan is the pre-built copy plan; nil for delete jobs and for streamed copies.
	// Populated as a synchronous-rebuild fallback (see jobbridge.TransferFunc) when the job
	// never went through PlanCh, and by RetryClone with the failed items of the previous run (such
	// a job skips the pre-scan). The normal streaming pre-scan leaves this nil throughout.
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
	// TotalsComplete is true once TotalFiles/TotalDirs/TotalBytes are final: the counting walk has
	// finished, or from the start for a failed-items retry (RetryClone). Unlike PlanComplete it
	// does not wait for the delivery walk, which the transfer
	// executor drains and so only ends near the end of the job. Same locking rule as PlanComplete.
	TotalsComplete bool
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

	// FlattenRemoveEmpty removes directories left empty under the flatten roots (Sources) as the
	// flatten proceeds (TypeFlatten only).
	FlattenRemoveEmpty bool
	// FlattenRecursive flattens every file under the roots instead of only their immediate
	// children (TypeFlatten only).
	FlattenRecursive bool

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
	// DereferenceSymlinks copies through symlinks instead of recreating them. Copy jobs only —
	// AddTransferJob is the single place that decides this, and always forces it false for Move.
	DereferenceSymlinks bool
	// FlattenIntoDest requests dest/<basename> naming for a copy/move job (transfer-dialog
	// "Flatten into destination" checkbox), independent of TypeFlatten jobs.
	FlattenIntoDest bool

	// Undo is the journal of what this copy, move or rename did, oldest first; ExecuteUndo reverses
	// it. A TypeUndo job carries a copy of its target's journal. Written only by State.ApplyEvent
	// (EventUndoJournal) and at enqueue.
	Undo []ops.UndoStep
	// Irreversible is true when the job overwrote or discarded something, so Undo would not
	// restore the previous state. Written only by State.ApplyEvent (EventUndoJournal).
	Irreversible bool
	// Ops is the job's per-item operation log (the jobs view Operations pane). Created by
	// State.AddJob, shared by value copies of the job and kept across a retry; written by the
	// transfer through ops.Options.OnItemDone / OnItemError (jobbridge), closed when the job
	// leaves the state.
	Ops *oplog.Log
	// UndoOf is the ID of the job a TypeUndo job reverses.
	UndoOf string
	// Undone is set on the original job once an undo job has reversed (part of) it.
	Undone bool

	// RemovingSources is true once a move's copy fallback is deleting its transferred sources.
	RemovingSources bool
}

// journaled reports whether jobs of type t record an undo journal.
func (t Type) journaled() bool {
	return t == TypeCopy || t == TypeMove || t == TypeRename
}

// Undoable reports whether the job's journal can be reversed: a finished copy, move or rename
// that recorded something, overwrote nothing and has not been undone. A canceled or failed copy
// keeps the journal of what it did write. State.EnqueueUndo also rules out a running undo.
func (j *Job) Undoable() bool {
	return j.Type.journaled() && j.Status.IsFinished() && len(j.Undo) > 0 && !j.Irreversible && !j.Undone
}

// UndoLabel is the Details line value for a job that can be undone or explains why not; empty
// for job types that never support undo.
func (j *Job) UndoLabel() string {
	switch {
	case !j.Type.journaled():
		return ""
	case j.Undone:
		return "undone"
	case j.Irreversible:
		return "irreversible (overwrote files)"
	case j.Undoable():
		return "available"
	case len(j.Undo) == 0 && j.Status.IsFinished():
		return "nothing to undo"
	}
	return ""
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
// Move and flatten do not: they rename first and plan only the sources that need a cross-device
// copy fallback (see ops.ExecuteMove). Flatten-into-dest copy is TypeCopy and keeps the pre-scan.
func (j *Job) NeedsPreScan() bool {
	return j != nil && j.Type == TypeCopy && len(j.Plan) == 0
}

// FailedItem is a source item a copy skipped, with the error text shown in the Errors panel.
type FailedItem struct {
	Item  ops.PlanItem
	Error string
}

// RetryClone returns a fresh Job with the same ID and only the enqueue-time spec fields
// copied from j — everything else (progress, ETA, PlanCh, Error, timestamps, PendingBlocker)
// starts zero, mirroring how AddTransferJob/EnqueueDeleteJob/EnqueueExtractJob/AddFlattenJob
// build a job at enqueue time. Status is StatusScanning; State.AddJob turns it into
// StatusQueued for types that do not pre-scan. TotalFiles is preset to len(Sources) for delete/extract, matching
// their enqueue path (copy/move/flatten recompute it during scan). A copy with FailedItems
// instead gets Plan = those items and their totals, so the retry re-runs only them without a scan.
func (j *Job) RetryClone() *Job {
	totalFiles, totalDirs := 0, 0
	if j.Type == TypeDelete || j.Type == TypeExtract {
		totalFiles = len(j.Sources)
	}
	var plan []ops.PlanItem
	planBytes := j.EstimatedBytes
	estimated := j.EstimatedBytes
	if j.Type == TypeCopy && len(j.FailedItems) > 0 {
		estimated = 0
		for _, f := range j.FailedItems {
			plan = append(plan, f.Item)
		}
		totalFiles, totalDirs, planBytes = ops.SummarizePlan(plan)
	}
	clone := &Job{
		Plan:            plan,
		TotalBytes:      planBytes,
		EstimatedBytes:  estimated,
		SourceSizeHints: j.SourceSizeHints,
		ID:              j.ID,
		Type:            j.Type,
		Status:          StatusScanning,
		Sources:         j.Sources,
		Destination:     j.Destination,
		DestIsDir:       ops.DestinationIsDirAtEnqueue(j.Destination),
		TotalFiles:      totalFiles,
		TotalDirs:       totalDirs,
		// A retry plan skips the pre-scan: SummarizePlan's totals are already final.
		TotalsComplete:        len(plan) > 0,
		FlattenRemoveEmpty:    j.FlattenRemoveEmpty,
		FlattenRecursive:      j.FlattenRecursive,
		DeleteRemoveEmptyDirs: j.DeleteRemoveEmptyDirs,
		PromptDanglingDirs:    j.PromptDanglingDirs,
		PreservePermissions:   j.PreservePermissions,
		PreserveTimestamps:    j.PreserveTimestamps,
		DereferenceSymlinks:   j.DereferenceSymlinks,
		FlattenIntoDest:       j.FlattenIntoDest,
		Ops:                   j.Ops,
	}
	return clone
}

// holdsTransferLease reports whether the job must serialize on the single transfer lease.
// Copy/move/flatten/extract always take the lease. A delete takes it only when its source
// roots overlap other (an active transfer's sources or destination). other may be nil.
// ponytail: global lease; becomes a per-device lane key when device queues land
func (j *Job) holdsTransferLease(other *Job) bool {
	if j == nil {
		return false
	}
	if j.Type != TypeDelete {
		return true
	}
	return deleteOverlapsTransfer(j, other)
}

// deleteOverlapsTransfer reports whether del's source roots share a tree with
// transfer's sources or destination.
func deleteOverlapsTransfer(del, transfer *Job) bool {
	if del == nil || transfer == nil {
		return false
	}
	for _, root := range del.Sources {
		for _, src := range transfer.Sources {
			if pathloc.TreesOverlap(root, src) {
				return true
			}
		}
		if !transfer.Destination.IsZero() && pathloc.TreesOverlap(root, transfer.Destination) {
			return true
		}
	}
	return false
}

// RecordOperation appends one item outcome to the job's operation log (the Operations pane). It
// is the single writer of Job.Ops and runs on the job or scan goroutine.
func (j *Job) RecordOperation(item ops.PlanItem, res ops.ItemResult, reason string) {
	if j.Ops == nil {
		return
	}
	r := oplog.Done
	switch res {
	case ops.ItemSkipped:
		r = oplog.Skipped
	case ops.ItemFailed:
		r = oplog.Failed
	}
	j.Ops.Append(oplog.Record{Result: r, Label: OperationLabel(j.Destination, item), Reason: reason, Source: item.Source})
}

// FailItem is the one way a job reports an item it could not process: it records the failed
// row and emits EventItemFailed (FailedItems, retry, the "N item(s) failed" error). Both the
// plan walk (unsupported source types) and the executor (source read errors) call it, with the
// job's status at the time. It runs on the scan or job goroutine.
func (j *Job) FailItem(emit func(Event), status Status, item ops.PlanItem, err error) {
	reason := ops.RootFirstErrorText(err)
	j.RecordOperation(item, ops.ItemFailed, reason)
	emit(Event{
		Type:       EventItemFailed,
		JobID:      j.ID,
		Status:     status,
		FailedItem: &FailedItem{Item: item, Error: reason},
	})
}

// OperationLabel is an Operations row label: the item's destination relative to the job
// destination, else its source path (delete, extract, items outside the destination). A string
// trim, not a path parse, because it runs once per file.
func OperationLabel(dest pathloc.Path, item ops.PlanItem) string {
	if !item.Dst.IsZero() && !dest.IsZero() {
		root := strings.TrimSuffix(dest.String(), "/")
		if rel, ok := strings.CutPrefix(item.Dst.String(), root+"/"); ok && rel != "" {
			return rel
		}
	}
	if !item.Src.IsZero() {
		return item.Src.String()
	}
	return item.Dst.String()
}
