package diskusage

// EventKind classifies disk usage notifications for subscribers (e.g. idle-sort debouncing).
type EventKind uint8

const (
	// EventSubtreeIndexed means a single listing-child root walk merged into the cache.
	EventSubtreeIndexed EventKind = iota
	// EventJobFinished means the planner finished the current scan session (toast/spinner).
	EventJobFinished
)

// Event is a structured notification emitted by [Engine] on best-effort buffered channels.
type Event struct {
	Kind       EventKind
	Generation uint64 // planner session id when emitted; stale workers discard before emit
	// RootAbs is set for EventSubtreeIndexed (filepath.Clean walk root).
	RootAbs string
	// SourcePanel is the initiating panel id (same convention as ui.PrimaryPanel / ui.SecondaryPanel).
	SourcePanel int
}
