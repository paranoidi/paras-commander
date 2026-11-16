// Package jump owns the Jump to directory dialog: a fuzzy name search over the directories below
// a panel directory whose Enter changes the panel to the highlighted one. Local panels walk the
// disk on the dialog's own scan coordinator; fileindex:// panels query the file index.
package jump

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scan"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Host supplies the app services the jump handler cannot import from internal/app.
type Host interface {
	host.LayoutHost
	host.PanelShowHost
	// Styles is the live theme (the dialog spec's icon widths depend on it).
	Styles() theme.Theme
	GitignoreCache() *gitignore.Cache
	// PinTogglePath pins/unpins path (add-or-remove) and shows a transient "Pinned"/"Unpinned"
	// status message using name.
	PinTogglePath(name, path string, isDir bool)
}

// Deps wires the jump handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	Config config.Config
	Index  *fileindex.Index
	// Keys is the main keymap (panel.jump-dialog closes the dialog, panel.pin-toggle pins);
	// KeysJumpDialog the [dialog.jump] overlay; KeysDialogInput the query editing keys.
	Keys, KeysJumpDialog, KeysDialogInput *keymap.Map
}

// Handler owns the jump dialog (ui.Model.JumpDialog), its disk scan and its index queries.
type Handler struct {
	host   Host
	screen tcell.Screen
	model  *ui.Model
	config config.Config
	index  *fileindex.Index

	keys, keysJump, keysDialogInput *keymap.Map

	// scan walks the disk for JumpDisk; scanGen names its current session (events of another
	// session are dropped).
	scan        *scan.Coordinator
	scanGen     int
	wakePending atomic.Bool
	eventMu     sync.Mutex
	events      []scan.Event

	// rankGen names the current query: a match result or index result of another is stale.
	// rankTimer is the pending debounce / throttle (UI goroutine only); rankSeq tells its wake
	// from one a stopped timer already posted.
	rankGen      int
	rankTimer    *time.Timer
	rankSeq      int
	lastRankSent time.Time
	lastProgress time.Time
}

// WakePayload is posted when scan events are queued for PollScan.
type WakePayload struct{}

// RankDuePayload is posted when a debounced or throttled match / query is due.
type RankDuePayload struct{ Seq int }

// IndexResultPayload carries the outcome of one index query; Gen says which query it answers.
type IndexResultPayload struct {
	Gen       int
	Results   []dialog.JumpResult
	Truncated bool
	Err       error
}

// New builds the handler and its scan coordinator.
func New(d Deps) *Handler {
	h := &Handler{
		host: d.Host, screen: d.Screen, model: d.Model, config: d.Config, index: d.Index,
		keys: d.Keys, keysJump: d.KeysJumpDialog, keysDialogInput: d.KeysDialogInput,
	}
	h.scan = scan.NewCoordinator(h.config.AdaptiveIO.Params(), h.onScanEvent)
	return h
}

// SetConfig replaces the handler's configuration snapshot (configuration reload).
func (h *Handler) SetConfig(cfg config.Config) { h.config = cfg }

func (h *Handler) post(payload any) {
	if h.screen != nil {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(payload))
	}
}
