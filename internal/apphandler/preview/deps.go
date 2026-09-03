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
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/preview/prefetch"
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
	quickViewDebounce        sched.Debouncer
	quickViewLastFingerprint string
	// quickViewNavSkipReconcile suppresses reconcileQuickViewPreview while file-list nav coalesce
	// is holding a pending preview flush (mirrors syncFollowNavSkipReconcile in internal/app).
	quickViewNavSkipReconcile atomic.Bool
	// quickViewDirNavPath tracks the last-seen cwd per panel (indexed by ui.PrimaryPanel/
	// ui.SecondaryPanel) so HandlePanelDirChanged only reacts to an actual directory change.
	quickViewDirNavPath [2]string

	// carouselPreviewDebounceGen invalidates in-flight carousel side-preview debounce callbacks.
	carouselPreviewDebounceGen atomic.Uint64
	carouselPreviewDebounce    sched.Debouncer
	// carouselPreviewNavSkipSnapshot, when true, reuses cached carousel parent/child snapshots during render.
	carouselPreviewNavSkipSnapshot atomic.Bool
	// carouselSide holds each panel's async side-column dispatch bookkeeping (indexed by
	// ui.PrimaryPanel/ui.SecondaryPanel; see carouselSideSlot). The "does this need a fetch at
	// all" question is answered by the cache-validity check itself (CarouselParentCacheValid/
	// CarouselChildCacheValidFor), which makes the dispatch self-correcting: any change that
	// invalidates the cache (chdir, cursor move, cache clear) re-triggers a fetch without needing
	// its own hook.
	carouselSide [2]carouselSideState

	// previewStyleAtPickerOpen is preview.style when the F3 Chroma style picker opens.
	previewStyleAtPickerOpen string
	// previewStylePickerDebounceGen invalidates in-flight F3 style-picker preview debounce callbacks.
	previewStylePickerDebounceGen atomic.Uint64
	previewStylePickerDebounce    sched.Debouncer

	// prefetch is the optional background image/video warm cache (nil when [preview].prefetch is off).
	prefetch *prefetch.Engine
	// prefetchCfg is the config the running engine was built with, compared against the
	// live-derived one on every ensurePrefetch call so a settings change that moves the cache
	// keys (image protocol, max-edge clamps, video grid shape) restarts the engine instead of
	// leaving it warming keys the foreground preview path never asks for.
	prefetchCfg prefetch.Config
	// prefetchLastCursor / prefetchLastPath / prefetchLastSurfaceActive record the previous
	// SchedulePrefetchFromActivePanel call's position and surface state, so the next call can
	// tell which direction the caret is moving (to bias the prefetch queue that way) and skip
	// rebuilding the queue entirely when nothing has actually changed since last time.
	prefetchLastCursor        int
	prefetchLastPath          pathloc.Path
	prefetchLastSurfaceActive bool
	// prefetchLastEntryCount additionally guards the skip-rebuild check above against a listing
	// change that doesn't move the caret or path (e.g. M-. toggling hidden/gitignored files) —
	// without it, such a change is invisible to the check and prefetch scheduling is silently
	// skipped even though the near-caret window now covers a completely different set of entries.
	prefetchLastEntryCount int
	// prefetchLastEntries / prefetchLastBox cache the most recently scheduled window and render box
	// (written only from SchedulePrefetchFromActivePanel on the main goroutine, read under Mu), so
	// rebuildPrefetchWarmMap can refresh the warm-icon snapshot from syncPrefetchLoadingMarks — which
	// the Cache's OnChange callback can invoke from a background prefetch-worker goroutine — without
	// that path touching Host/ActivePanel or screen state itself.
	prefetchLastEntries []localfs.Entry
	prefetchLastBox     *prefetch.RenderBox
	// prefetchWarmMapDebounce coalesces bursts of Cache.OnChange firings (one per completed
	// decode/render) into a single rebuildPrefetchWarmMap call shortly after the burst settles,
	// instead of doing a full window rescan on every individual completion.
	prefetchWarmMapDebounce sched.Debouncer
}

// New constructs a Handler.
func New(d Deps) *Handler {
	h := &Handler{
		host:            d.Host,
		screen:          d.Screen,
		model:           d.Model,
		keysDialogInput: d.KeysDialogInput,
		mu:              d.Mu,
		ctx:             d.Ctx,
	}
	h.ensurePrefetch()
	return h
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

// QuickViewDirRuleDeclinedPayload signals that every [[preview.commands]] rule matching the
// directory currently open in quick view declined (non-zero exit), or the async run raced past
// a superseded gen. The main goroutine falls back to the built-in directory-overlay listing —
// see Handler.ApplyQuickViewDirRuleDeclined.
type QuickViewDirRuleDeclinedPayload struct{ gen uint64 }
