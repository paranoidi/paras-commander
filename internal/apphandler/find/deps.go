package find

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scan"
	"github.com/paranoidi/paras-commander/internal/sched"
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

	scan        *scan.Coordinator
	scanGen     int
	wakePending atomic.Bool

	eventMu       sync.Mutex
	pendingEvents []scan.Event

	rankMu              sync.Mutex
	rankGen             int
	rankTimer           *time.Timer
	lastRankSentAt      time.Time
	throttleWakePending bool
	debouncePending     bool
	pendingRank         *rankResult

	findNavTimer  *time.Timer
	findNavEpoch  uint64
	findNavActive bool

	lastIndexCountRenderAt time.Time

	sizeMu            sync.Mutex
	fileSizeLookupGen uint64
	pendingFileSizes  *findFileSizeUpdate

	// groupCount debounces and runs the group-select dialog's live match count off the main
	// goroutine (find context only — see group_count.go). groupCountMu serializes the
	// counting goroutine's "am I still wanted" check against StopGroupCount's cancel, so a
	// count that's already committed to running is always waited for, and one that isn't
	// never touches the map (avoiding a concurrent read of MarkedPaths while it's mutated).
	// groupCountAcc holds the accumulated count/watermark that ExtendGroupCount advances
	// inline between full StartGroupCount runs, so the preview keeps up with entries still
	// arriving from indexing without a full async recount on every batch.
	groupCount       sched.Debouncer
	groupCountMu     sync.Mutex
	groupCountCancel context.CancelFunc
	groupCountWG     sync.WaitGroup
	groupCountGen    uint64
	groupCountAcc    groupCountAccum
}

type findFileSizeUpdate struct {
	lookupGen uint64
	selGen    uint64
	sizes     map[string]int64
}

// rankResult carries match output applied on the main thread.
type rankResult struct {
	gen              int
	ranked           []int
	fullRanked       []int
	matchRanges      map[int][]search.Range
	rankDisplayLines []string
	entriesLen       int
	onlyDirs         bool
	onlyFiles        bool
}

// RankWakePayload is posted when background match finishes.
type RankWakePayload struct{}

// ThrottleRankWakePayload is posted by the indexing-throttle timer.
type ThrottleRankWakePayload struct{}

// DebounceRankWakePayload is posted by the query-debounce timer.
type DebounceRankWakePayload struct{}

// FindNavIdlePayload is posted when list navigation goes idle.
type FindNavIdlePayload struct{ Epoch uint64 }

const (
	findIndexLargeThreshold  = 500_000
	findIndexMediumThreshold = 50_000
)
