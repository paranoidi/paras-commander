package jobs

// EventType categorises worker-to-app events.
type EventType string

const (
	EventEnqueued          EventType = "enqueued"
	EventStarted           EventType = "started"
	EventScanProgress      EventType = "scan-progress"
	EventScanTotals        EventType = "scan-totals"
	EventPlanTotals        EventType = "plan-totals"
	EventProgress          EventType = "progress"
	EventJobBlockerRequest EventType = "job-blocker-request"
	EventCompleted         EventType = "completed"
	EventFailed            EventType = "failed"
	EventCanceled          EventType = "canceled"
)

// Event is a structured message emitted by the worker to report job state changes.
type Event struct {
	Type      EventType
	JobID     string
	Status    Status
	DoneFiles int
	DoneBytes int64
	// TotalFiles, TotalDirs, TotalBytes are set for EventScanTotals and EventPlanTotals.
	TotalFiles int
	TotalDirs  int
	TotalBytes int64
	// CurrentPath is set for EventProgress (source path being processed).
	CurrentPath string
	// CurrentDestPath is set for EventProgress (destination path for the same item).
	CurrentDestPath string
	Error           string
	// Err is set for EventFailed when the worker has the original error (for compact banners).
	Err error
	// Blocker is set when Type == EventJobBlockerRequest (file conflict or disk space).
	Blocker *BlockerDetails
}

// ConflictEvent carries information about a file conflict.
type ConflictEvent struct {
	Source          string
	Destination     string
	ExistingDetails string // e.g. "file exists", "symlink exists"
	// Optional display lines (MC-style "New" / "Existing" columns).
	SourceSize string
	SourceTime string
	DestSize   string
	DestTime   string
}
