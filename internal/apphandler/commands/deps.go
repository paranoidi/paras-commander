// Package commands implements the Commands view (run-command list screen), the command
// output dialog, and the run-for-each dialog/batch backend.
package commands

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
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
}

// New creates a Handler.
func New(d Deps) *Handler {
	return &Handler{
		host:         d.Host,
		screen:       d.Screen,
		model:        d.Model,
		keys:         d.Keys,
		keysCommands: d.KeysCommands,
		mu:           d.Mu,
		ctx:          d.Ctx,
		workPools:    d.WorkPools,
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
