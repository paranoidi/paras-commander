package find

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	findpkg "github.com/paranoidi/paras-commander/internal/find"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the find handler at app construction.
type Deps struct {
	Host           Host
	Screen         tcell.Screen
	Model          *ui.Model
	Config         config.Config
	Keys           *keymap.Map
	KeysFindDialog *keymap.Map
}

// Handler owns find-dialog indexing and navigation.
type Handler struct {
	host           Host
	screen         tcell.Screen
	model          *ui.Model
	config         config.Config
	keys           *keymap.Map
	keysFindDialog *keymap.Map

	sessionMu      sync.Mutex
	walks          map[string]*walk
	batchCh        chan []findpkg.Entry
	indexedPaths   map[string]struct{}
	completedRoots map[string]struct{}
	// wakePending prevents flooding the tcell event queue with WakePayload events.
	// PollUpdates drains ALL available batches in one call, so only one pending
	// WakePayload is ever needed. Without this, a fast SSD can post hundreds of
	// WakePayloads per second, burying key-press events and making typing laggy.
	wakePending atomic.Bool

	rankMu    sync.Mutex
	rankGen   int
	rankTimer *time.Timer
	rankReady chan rankResult
	// lastRankSentAt is the last time a snapshot was actually delivered to the rank worker.
	// Used to implement throttling during indexing walks: fire immediately the first time,
	// then at most once per throttle interval. Protected by rankMu.
	lastRankSentAt time.Time
	// throttleWakePending is set true by an indexing-throttle timer callback just before
	// posting ThrottleRankWakePayload. HandleThrottleRankWake consumes it. This ensures
	// the handler is idempotent when polled from tests without a running event loop.
	throttleWakePending bool
	// debouncePending is set true by a query-debounce timer callback just before posting
	// DebounceRankWakePayload. HandleDebounceRankWake consumes it.
	debouncePending bool

	// findNavTimer is the navigation-idle debounce timer. It is armed whenever the user
	// navigates the result list (Up/Down/PgUp/PgDn) and fires FindNavIdleMS after the last
	// movement. While it is armed, ApplyPendingRank defers applying background rank updates
	// so the view stays stable during fast navigation.
	// Accessed only from the main thread.
	findNavTimer  *time.Timer
	findNavEpoch  uint64
	findNavActive bool

	// rankWorkCh carries the latest pending rank input to the single rank worker goroutine.
	// Capacity 1: the worker drains it; senders drain-then-replace to discard stale inputs.
	rankWorkCh chan rankInput
	// rankSendMu serialises the drain+send pair so concurrent timer callbacks and main-thread
	// sends cannot interleave and lose the newest input.
	rankSendMu sync.Mutex
}

// rankInput is a compact snapshot of the data needed for one rank computation.
// Using separate slices instead of a []FindEntry copy reduces the per-snapshot size
// from ~48 bytes/entry (struct) to ~17 bytes/entry, and string data is shared with
// the live st.Entries slice (no extra string allocations).
type rankInput struct {
	gen      int
	lines    []string // RelLine per entry; string data shared with st.Entries
	isDirs   []bool
	query    string
	onlyDirs bool
	opts     search.Options
}

// rankResult carries the output of a background rank computation.
type rankResult struct {
	gen         int
	ranked      []int
	matchRanges map[int][]search.Range // sparse: nil when query is empty
}

// RankWakePayload is posted to the event loop when an async rank computation finishes.
type RankWakePayload struct{}

// ThrottleRankWakePayload is posted by the indexing-throttle timer to tell the main thread
// to take a fresh snapshot and send it to the rank worker (without cancelling in-flight work).
type ThrottleRankWakePayload struct{}

// DebounceRankWakePayload is posted by the query-debounce timer. Unlike ThrottleRankWakePayload,
// handling it increments rankGen so any in-flight computation for the old query is discarded.
type DebounceRankWakePayload struct{}

// FindNavIdlePayload is posted by the navigation-idle timer after the user stops scrolling the
// result list. The main thread calls HandleFindNavIdle to apply any deferred rank result.
type FindNavIdlePayload struct{ Epoch uint64 }

type walk struct {
	root string
	sess *findpkg.Session
}
