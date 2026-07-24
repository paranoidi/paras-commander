// Package preview owns the file-preview cluster: inactive-column quick view, the fullscreen
// (F3) preview, carousel side/child preview, "/" incremental search in the fullscreen preview,
// and the Chroma style picker. All async preview subprocess runs and their debounce/coalesce
// bookkeeping live here; view/dialog state itself stays in the shared ui.Model.
package preview

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Deps wires the preview handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	// Keys is the [dialog.input] keymap overlay used for the style-picker query field and any
	// other scrolling-query editing owned by this handler.
	KeysDialogInput *keymap.Map
	// Mu is the App's shared async-model-mutation lock (guards FilePreview/CarouselFilePreview/
	// FullscreenFilePreview and other model fields written from background goroutines). render()
	// copies the whole App.model under this same lock, so mutations here must use the identical
	// mutex App uses elsewhere rather than a Handler-private lock.
	Mu *sync.RWMutex
	// Ctx is the app-lifetime cancellation context, canceled once at quit (internal/app/quit.go).
	// It is shared with the commands subsystem's background subprocess goroutines too.
	Ctx context.Context
}

// previewTarget identifies which of the three preview panes a piece of state applies to.
type previewTarget int

const (
	previewTargetInactive previewTarget = iota
	previewTargetFullscreen
	previewTargetCarousel
)

// Handler owns quick view, fullscreen (F3) preview, and carousel preview state and dispatch.
type Handler struct {
	host            Host
	screen          tcell.Screen
	model           *ui.Model
	keysDialogInput *keymap.Map
	mu              *sync.RWMutex
	ctx             context.Context

	// filePreviewRunGen invalidates in-flight preview subprocess completions (skip stale RenderWake).
	filePreviewRunGen atomic.Uint64
	// filePreviewHold keeps the last completed inactive-column preview for stale-while-revalidate draws.
	filePreviewHold ui.FilePreviewState
	// fullscreenFilePreviewHold keeps the last completed F3 preview body while the next file loads.
	fullscreenFilePreviewHold ui.FilePreviewState
	// carouselFilePreviewHold keeps the last completed carousel child preview body while loading.
	carouselFilePreviewHold ui.FilePreviewState
	// carouselFilePreviewRunGen invalidates in-flight carousel child-column preview subprocess completions.
	carouselFilePreviewRunGen atomic.Uint64
	// carouselFilePreviewLastFingerprint tracks the last carousel file preview highlight for debouncing.
	carouselFilePreviewLastFingerprint string
	// previewLastWidth records the TextWidth each preview target's content was last requested at
	// (indexed by previewTarget), so a terminal resize can detect a width change and re-run the
	// preview (markdown word-wrap/table layout is baked into emitted cells at request time).
	previewLastWidth [3]int

	// quickViewDebounceGen invalidates in-flight quick view preview debounce callbacks.
	quickViewDebounceGen     atomic.Uint64
	quickViewDebounce        sched.ManagedTimer
	quickViewLastFingerprint string
	// quickViewNavSkipReconcile suppresses reconcileQuickViewPreview while file-list nav coalesce
	// is holding a pending preview flush (mirrors syncFollowNavSkipReconcile in internal/app).
	quickViewNavSkipReconcile atomic.Bool

	// carouselPreviewDebounceGen invalidates in-flight carousel side-preview debounce callbacks.
	carouselPreviewDebounceGen atomic.Uint64
	carouselPreviewDebounce    sched.ManagedTimer
	// carouselPreviewNavSkipSnapshot, when true, reuses cached carousel parent/child snapshots during render.
	carouselPreviewNavSkipSnapshot atomic.Bool

	// previewStyleAtPickerOpen is preview.style when the F3 Chroma style picker opens.
	previewStyleAtPickerOpen string
	// previewStylePickerDebounceGen invalidates in-flight F3 style-picker preview debounce callbacks.
	previewStylePickerDebounceGen atomic.Uint64
	previewStylePickerDebounce    sched.ManagedTimer
}

// New constructs a Handler.
func New(d Deps) *Handler {
	return &Handler{
		host:            d.Host,
		screen:          d.Screen,
		model:           d.Model,
		keysDialogInput: d.KeysDialogInput,
		mu:              d.Mu,
		ctx:             d.Ctx,
	}
}

// RenderWakePayload wakes PollEvent purely to trigger a repaint after a background goroutine
// (file preview subprocess) mutates model state under Deps.Mu in place. It carries no data on
// purpose: callers that need to hand data back to the main goroutine post a more specific payload.
type RenderWakePayload struct{}

// postRenderWake posts a RenderWakePayload.
func (h *Handler) postRenderWake() {
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(RenderWakePayload{}))
}

// QuickViewFlushPayload reloads the inactive-column quick view preview after file-list debounce.
type QuickViewFlushPayload struct{ gen uint64 }

// CarouselPreviewFlushPayload reloads the carousel child side preview after file-list debounce.
type CarouselPreviewFlushPayload struct{ gen uint64 }

// StylePickerFlushPayload re-runs F3 preview highlighting after style-picker debounce.
type StylePickerFlushPayload struct{ gen uint64 }
