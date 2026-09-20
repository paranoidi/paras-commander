// Package commands implements the Commands view (run-command list screen), the command
// output dialog, and the run-for-each dialog/batch backend.
package commands

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/workpool"
)

// Deps wires the commands handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	// Keys is the global action keymap; KeysCommands is the Commands-view overlay (may be nil).
	Keys         *keymap.Map
	KeysCommands *keymap.Map
	// Mu is the App's shared async-model-mutation lock (guards CommandsList and other model
	// fields written from background goroutines). render() copies the whole App.model under
	// this same lock, so CommandsList mutations here must use the identical mutex App uses
	// elsewhere (internal/apphandler/preview etc.) rather than a Handler-private lock — splitting
	// the lock would let render's whole-struct copy race with CommandsList appends.
	Mu *sync.RWMutex
	// Ctx is the app-lifetime cancellation context, canceled once at quit (internal/app/quit.go).
	// It is shared with the preview subsystem's background subprocess goroutines too.
	Ctx       context.Context
	WorkPools *workpool.Registry
}

// Handler owns the Commands view, command-output dialog, and run-for-each dialog/backend.
type Handler struct {
	host         Host
	screen       tcell.Screen
	model        *ui.Model
	keys         *keymap.Map
	keysCommands *keymap.Map
	mu           *sync.RWMutex
	ctx          context.Context
	workPools    *workpool.Registry

	// batchesInflight counts in-flight command batches (run-for-each, user-menu, file-execute)
	// started via Deps.Ctx-scoped goroutines. HasRunning reports whether any is still running.
	batchesInflight atomic.Int32

	// procsMu guards procs, which maps a Commands-view row index to the handle needed to
	// terminate/kill its running subprocess (commands.terminate/commands.kill).
	procsMu sync.Mutex
	procs   map[int]*procHandle

	// ptyMu guards live PTY session bookkeeping written from batch goroutines (runEntryPTY)
	// and read from the main goroutine (key routing, terminate/kill, grow/shrink, cursor
	// sync) — mirrors the procsMu/procs pattern above.
	ptyMu sync.RWMutex
	// ptySessions maps a Commands-view row index to its live PTY session (foreground or
	// background). Several batches can run PTYs at once; terminate/kill looks up by row.
	ptySessions map[int]*entryPTYSession
	// ptyPanelOwner is the foreground PTY batch that currently holds the terminal-panel
	// lease (0 = none). Ownership is batch-level: it stays set through pool waits and
	// between entries, not only while a process is current.
	ptyPanelOwner int64
	// ptyPanelSess is the owning batch's current entry session, or nil between entries.
	// ActivePTYSession returns this; OwnsTerminalPanel looks at ptyPanelOwner.
	ptyPanelSess *entryPTYSession
	// ptyForeground is a 1-slot semaphore serializing foreground PTY batches so a second
	// batch cannot replace the first's drawer. Empty until New fills the token.
	ptyForeground chan struct{}
	ptyOwnerSeq   atomic.Int64

	// runForEachHistory is the in-memory, session-only (never persisted) list of recently-run
	// run-for-each command lines, most-recent-first, capped at maxRunForEachHistory. Backs the
	// F3 command-history picker on the run-for-each dialog's main screen.
	runForEachHistory []string
}

// entryPTYSession is the live PTY state for one run-for-each entry.
type entryPTYSession struct {
	idx  int
	sub  *subshell.Subshell
	feed *subshell.PanelFeed
}

// New creates a Handler.
func New(d Deps) *Handler {
	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	return &Handler{
		host:          d.Host,
		screen:        d.Screen,
		model:         d.Model,
		keys:          d.Keys,
		keysCommands:  d.KeysCommands,
		mu:            d.Mu,
		ctx:           d.Ctx,
		workPools:     d.WorkPools,
		ptySessions:   make(map[int]*entryPTYSession),
		ptyForeground: sem,
	}
}

// Context returns the app-lifetime cancellation context used for spawned command subprocesses.
func (h *Handler) Context() context.Context { return h.ctx }

// BeginBatch marks one command batch as started; callers must EndBatch when it finishes.
func (h *Handler) BeginBatch() { h.batchesInflight.Add(1) }

// EndBatch marks one command batch as finished.
func (h *Handler) EndBatch() { h.batchesInflight.Add(-1) }

// HasRunning reports whether any command batch is still in flight.
func (h *Handler) HasRunning() bool { return h.batchesInflight.Load() > 0 }

func (h *Handler) acquireForegroundPanel(ctx context.Context, id int64) bool {
	if h.ptyForeground == nil {
		return false
	}
	select {
	case <-h.ptyForeground:
		h.ptyMu.Lock()
		h.ptyPanelOwner = id
		h.ptyPanelSess = nil
		h.ptyMu.Unlock()
		return true
	case <-ctx.Done():
		return false
	}
}

func (h *Handler) releaseForegroundPanel(id int64) {
	h.ptyMu.Lock()
	if h.ptyPanelOwner == id {
		h.ptyPanelOwner = 0
		h.ptyPanelSess = nil
	}
	h.ptyMu.Unlock()
	if h.ptyForeground == nil {
		return
	}
	select {
	case h.ptyForeground <- struct{}{}:
	default:
	}
}

func (h *Handler) registerEntryPTY(owner int64, s *entryPTYSession) {
	h.ptyMu.Lock()
	if h.ptySessions == nil {
		h.ptySessions = make(map[int]*entryPTYSession)
	}
	h.ptySessions[s.idx] = s
	if owner != 0 && h.ptyPanelOwner == owner {
		h.ptyPanelSess = s
	}
	h.ptyMu.Unlock()
}

func (h *Handler) unregisterEntryPTY(idx int) {
	h.ptyMu.Lock()
	delete(h.ptySessions, idx)
	if h.ptyPanelSess != nil && h.ptyPanelSess.idx == idx {
		h.ptyPanelSess = nil
	}
	h.ptyMu.Unlock()
}

func (h *Handler) entryPTYByIndex(idx int) *entryPTYSession {
	h.ptyMu.RLock()
	defer h.ptyMu.RUnlock()
	return h.ptySessions[idx]
}

// OwnsTerminalPanel reports whether a foreground run-for-each PTY batch currently holds the
// terminal-panel lease — used by internal/app's Alt+P entry points to refuse to clobber it.
// True for the whole owning batch (pool wait and gaps between entries included). Background
// PTY sessions never set this.
func (h *Handler) OwnsTerminalPanel() bool {
	h.ptyMu.RLock()
	defer h.ptyMu.RUnlock()
	return h.ptyPanelOwner != 0
}

// ActivePTYSession returns the run-for-each PTY session currently occupying the terminal panel,
// if any — used by internal/app so Ctrl-O (drop-to-shell) and panel grow/shrink operate on the
// actual live session instead of always assuming the persistent Alt+P shell. Nil between
// entries of the owning batch, and never a background PTY.
func (h *Handler) ActivePTYSession() (sub *subshell.Subshell, feed *subshell.PanelFeed, ok bool) {
	h.ptyMu.RLock()
	defer h.ptyMu.RUnlock()
	if h.ptyPanelSess == nil {
		return nil, nil, false
	}
	return h.ptyPanelSess.sub, h.ptyPanelSess.feed, true
}
