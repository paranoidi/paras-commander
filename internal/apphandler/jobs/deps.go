package jobs

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the jobs handler at app construction.
type Deps struct {
	Host     Host
	Screen   tcell.Screen
	Model    *ui.Model
	State    *jobs.State
	Config   config.Config
	Keys     *keymap.Map
	KeysJobs *keymap.Map
}

// Handler owns jobs-view UI orchestration and job event draining.
type Handler struct {
	host     Host
	screen   tcell.Screen
	model    *ui.Model
	state    *jobs.State
	config   config.Config
	keys     *keymap.Map
	keysJobs *keymap.Map

	wakeMu    sync.Mutex
	wakeTimer *time.Timer

	refreshTerminal bool
	refreshProgress bool

	affectVisible             bool
	lastBatchMenuBarStripOnly bool
	listStale                 bool
	listVersion               uint64
	pathMarksVersion          uint64

	// pendingDanglingSources accumulates Sources from completed jobs with
	// PromptDanglingDirs set, stashed in scanBatchFlags and drained (FS-checked,
	// then possibly prompted) in ApplyRefreshes — never on the event-batch path,
	// which must stay free of filesystem I/O.
	pendingDanglingSources []pathloc.Path

	// jobBlockerNextGen invalidates in-flight quick-blocker chain timers.
	jobBlockerNextGen atomic.Uint64
	jobBlockerNext    sched.Debouncer
}
