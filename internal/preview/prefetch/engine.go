package prefetch

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// Kind classifies a prefetch job.
type Kind int

const (
	KindImage Kind = iota
	KindVideo
)

// Item is one candidate file near the caret.
type Item struct {
	Path   string
	Kind   Kind
	Offset int // signed distance from the caret: negative = before, positive = after
	Mtime  int64
	Size   int64
}

// Config configures the prefetch engine.
type Config struct {
	Workers           int
	MemoryMaxMB       int
	RenderCacheMaxMB  int
	VideoDiskMaxMB    int
	ImageMaxEdgePx    int
	VideoMaxEdgePx    int
	VideoThumbCols    int
	VideoThumbRows    int
	VideoThumbWorkers int
	DiskDir           string
}

// RenderBox is the exact on-screen pixel box the currently active preview surface would render
// an image into — protocol, exact target pixel size, and tmux/unicode-placeholder flags — supplied
// by the app layer so background prefetch can eagerly warm Cache.LoadRender, not just the
// intermediate maxEdge PNG. nil means no box is currently determinable (e.g. no preview surface
// laid out), in which case eager render-warming is skipped for that Schedule call.
type RenderBox struct {
	Proto              previewpanel.ImageProtocol
	UnicodePlaceholder bool
	InTmux             bool
	MaxPxW, MaxPxH     int
}

// Engine owns the worker pool and shared Cache.
type Engine struct {
	cache  *Cache
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.Mutex
	pending   []Item
	renderBox *RenderBox
	cond      *sync.Cond
	wg        sync.WaitGroup
}

// currentRenderBox returns the render box passed to the most recent Schedule call.
func (e *Engine) currentRenderBox() *RenderBox {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.renderBox
}

// NewEngine starts workers when workers > 0. Caller must Close. onChange, if non-nil, is the
// wake for loading-mark redraw; it is a parameter rather than a Config field so Config stays
// comparable — the app layer compares the live-derived config against the running engine's to
// notice a settings change that moves the cache keys (see apphandler/preview.ensurePrefetch).
func NewEngine(parent context.Context, cfg Config, onChange func()) *Engine {
	if cfg.Workers < 1 {
		cfg.Workers = config.DefaultPreviewPrefetchWorkers
	}
	if cfg.MemoryMaxMB < 1 {
		cfg.MemoryMaxMB = config.DefaultPreviewPrefetchMemoryMaxMB
	}
	if cfg.RenderCacheMaxMB < 1 {
		cfg.RenderCacheMaxMB = config.DefaultPreviewRenderCacheMaxMB
	}
	if cfg.VideoDiskMaxMB < 1 {
		cfg.VideoDiskMaxMB = config.DefaultPreviewVideoThumbCacheMaxMB
	}
	if cfg.VideoThumbCols < 1 {
		cfg.VideoThumbCols = config.DefaultPreviewVideoThumbCols
	}
	if cfg.VideoThumbRows < 1 {
		cfg.VideoThumbRows = config.DefaultPreviewVideoThumbRows
	}
	if cfg.VideoThumbWorkers < 1 {
		cfg.VideoThumbWorkers = config.DefaultPreviewVideoThumbWorkers
	}
	diskDir := cfg.DiskDir
	if diskDir == "" {
		if d, err := DefaultDir(); err == nil {
			diskDir = d
		}
	}
	ctx, cancel := context.WithCancel(parent)
	e := &Engine{
		cache: NewCache(
			int64(cfg.MemoryMaxMB)*1024*1024,
			int64(cfg.VideoDiskMaxMB)*1024*1024,
			int64(cfg.RenderCacheMaxMB)*1024*1024,
			diskDir,
		),
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	e.cond = sync.NewCond(&e.mu)
	if onChange != nil {
		e.cache.SetOnChange(onChange)
	}
	for i := 0; i < cfg.Workers; i++ {
		e.wg.Add(1)
		go e.worker()
	}
	return e
}

// Cache returns the shared MediaCache for display paths.
func (e *Engine) Cache() *Cache { return e.cache }

// Close stops workers.
func (e *Engine) Close() {
	e.cancel()
	e.mu.Lock()
	e.pending = nil
	e.cond.Broadcast()
	e.mu.Unlock()
	e.wg.Wait()
}

// Schedule replaces the pending queue with items sorted by a five-tier priority ladder: the
// caret's exact entry, then its immediate ±prefetchNearRadius neighbors, then the two exact
// PgUp/PgDn landing offsets (±pageSize), then the rest of the caret's ±window band (biased
// toward dir — positive = caret moving toward later entries, negative = earlier, 0 = no bias),
// then the vicinity of each landing offset's own ±window band. Within each tier items are
// nearest-first. Non-media entries are dropped, and so are already-warm ones — for images
// "warm" means both the still-PNG tier (HasStill) and the render-payload tier for the current
// box (HasRender) are warm, since a still-warm item can still need its render payload rebuilt
// for a new/changed box. Items currently in flight (Cache.InFlight) are also skipped, since a
// worker is already warming them and re-queuing would just let a different idle worker block
// inside doFlight's singleflight wait instead of doing other work.
//
// Tier 5 (landing vicinity) is a hard gate, not just a sort tail: whenever this call also has any
// tier ≤4 (near-cursor/landing-center) item outstanding, every tier-5 item is dropped from the
// queue entirely rather than merely sorted last. Workers are not preemptible once dispatched
// (runJob), so without this gate a worker can pick up a low-value tier-5 job from an older
// Schedule call and sit on it while a fresh call's near-cursor work waits — this is what caused
// the worker-starvation regression during ordinary one-at-a-time cursor scrolling. Once the caret
// settles and a later Schedule call has no tier ≤4 work left, tier-5 items are scheduled normally
// to backfill the landing vicinity.
func (e *Engine) Schedule(items []Item, dir, pageSize, window int, box *RenderBox) {
	type scored struct {
		it    Item
		group int
	}
	scoredItems := make([]scored, 0, len(items))
	hasNearWork := false
	for _, it := range items {
		if it.Path == "" {
			continue
		}
		if e.cache.InFlight(it.Path) {
			// Some worker is already warming this path right now (LoadStill or LoadRender — either
			// way runJob handles both stages for an image in one dispatch). Re-queuing it here would
			// let a different, idle worker dequeue it and block inside LoadRender's doFlight
			// singleflight wait instead of doing other work — this is what caused a near-cursor
			// throughput stall during rapid single-step scrolling. It'll be re-evaluated fresh (and
			// re-included if still needed) on the next Schedule call once the in-flight work finishes.
			continue
		}
		switch it.Kind {
		case KindImage:
			if e.isImageWarm(it.Path, it.Mtime, it.Size, box) {
				continue
			}
		case KindVideo:
			if e.cache.HasVideo(it.Path, it.Mtime, it.Size, e.cfg.VideoMaxEdgePx, e.cfg.VideoThumbCols, e.cfg.VideoThumbRows) {
				continue
			}
		default:
			continue
		}
		g := priorityGroup(it.Offset, dir, pageSize, window)
		if g <= 4 {
			hasNearWork = true
		}
		scoredItems = append(scoredItems, scored{it, g})
	}
	sort.SliceStable(scoredItems, func(i, j int) bool {
		if scoredItems[i].group != scoredItems[j].group {
			return scoredItems[i].group < scoredItems[j].group
		}
		return abs(scoredItems[i].it.Offset) < abs(scoredItems[j].it.Offset)
	})
	filtered := make([]Item, 0, len(scoredItems))
	for _, s := range scoredItems {
		if s.group == 5 && hasNearWork {
			// Landing vicinity waits for the caret's own window to fully warm first — see the
			// doc comment above and priorityGroup's tier ladder. Dropped here (not just sorted
			// last) so a worker never gets tied up on this low-value work while a fresh Schedule
			// call still has near-cursor work outstanding; it'll be picked up on a later call
			// once hasNearWork goes false (the caret has settled and the window is warm).
			continue
		}
		filtered = append(filtered, s.it)
	}
	e.mu.Lock()
	e.pending = filtered
	e.renderBox = box
	e.cond.Broadcast()
	e.mu.Unlock()
}

// isImageWarm reports whether path's prefetch cache is warm for an image: both the intermediate
// decode tier (HasStill) and, when box is known, the render-payload tier for that exact box
// (HasRender). Shared by Schedule's dedup filter and IsEntryWarm so the two-tier warmness rule
// can't drift between them.
func (e *Engine) isImageWarm(path string, mtime, size int64, box *RenderBox) bool {
	if !e.cache.HasStill(path, mtime, size, e.cfg.ImageMaxEdgePx) {
		return false
	}
	return box == nil || e.cache.HasRender(path, mtime, size, box.Proto, box.UnicodePlaceholder, box.InTmux, box.MaxPxW, box.MaxPxH)
}

// prefetchNearRadius is the "immediate" cursor band highest non-exact
// priority — a small tuning constant for prefetch ordering, not a resource
// budget, so (like shrinkSixelForTmux's 0.75 retry factor) it stays a local
// const rather than a config field.
const prefetchNearRadius = 2

// priorityGroup buckets an item into Schedule's five-tier ladder: exact caret position, the
// immediate ±prefetchNearRadius neighbors, the two exact PgUp/PgDn landing offsets, the rest of
// the caret's ±window band (biased toward dir), then the vicinity of each landing offset's own
// ±window band.
func priorityGroup(offset, dir, pageSize, window int) int {
	switch {
	case offset == 0:
		return 0
	case abs(offset) <= prefetchNearRadius:
		return 1
	case pageSize > 0 && (offset == pageSize || offset == -pageSize):
		return 2
	case abs(offset) <= window && (dir == 0 || (dir > 0) == (offset > 0)):
		return 3
	case abs(offset) <= window:
		return 4
	default:
		return 5 // landing-band vicinity; only reachable when pageSize > 0,
		// since withinPrefetchRange already excludes everything else
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// withinPrefetchRange reports whether offset falls inside the caret's ±window band or the
// vicinity (±window) of either PgUp/PgDn landing offset (±pageSize). pageSize <= 0 means no
// landing band is predicted, so only the caret's own window applies.
func withinPrefetchRange(offset, window, pageSize int) bool {
	if abs(offset) <= window {
		return true
	}
	return pageSize > 0 && (abs(offset-pageSize) <= window || abs(offset+pageSize) <= window)
}

// ScheduleFromListing builds Items from a panel listing around cursorIdx, biased toward dir.
// Entries outside the caret's ±window band and outside the vicinity of either PgUp/PgDn landing
// offset (±pageSize) are dropped, even if the caller didn't pre-slice entries to that range — the
// range invariant is enforced here so no caller can reintroduce whole-directory prefetch by
// passing an unbounded listing.
func (e *Engine) ScheduleFromListing(entries []localfs.Entry, cursorIdx, window, dir, pageSize int, box *RenderBox) {
	if len(entries) == 0 {
		e.Schedule(nil, dir, pageSize, window, box)
		return
	}
	if cursorIdx < 0 {
		cursorIdx = 0
	}
	if cursorIdx >= len(entries) {
		cursorIdx = len(entries) - 1
	}
	items := make([]Item, 0, len(entries))
	for i, ent := range entries {
		offset := i - cursorIdx
		if !withinPrefetchRange(offset, window, pageSize) {
			continue
		}
		if ent.Type == localfs.EntryDirectory {
			continue
		}
		path := ent.Path
		var kind Kind
		switch {
		case localfs.IsImagePath(path):
			kind = KindImage
		case localfs.IsVideoPath(path):
			kind = KindVideo
		default:
			continue
		}
		mtime, size := resolveMtimeSize(ent)
		items = append(items, Item{
			Path:   path,
			Kind:   kind,
			Offset: offset,
			Mtime:  mtime,
			Size:   size,
		})
	}
	e.Schedule(items, dir, pageSize, window, box)
}

// resolveMtimeSize returns ent's mtime/size, falling back to a live stat when the listing didn't
// populate them (some listings omit ModifiedAt/Size).
func resolveMtimeSize(ent localfs.Entry) (mtime, size int64) {
	mtime = ent.ModifiedAt.UnixNano()
	size = ent.Size
	if ent.ModifiedAt.IsZero() || size == 0 {
		if fi, err := os.Stat(ent.Path); err == nil {
			mtime = fi.ModTime().UnixNano()
			size = fi.Size()
		}
	}
	return mtime, size
}

// IsEntryWarm reports whether ent's prefetch cache is currently warm — both the intermediate
// decode tier and, for images, the final render-payload tier for the given box (mirrors the same
// two-tier check Schedule uses to decide whether an image item still needs work). Used only for
// the debug preloaded/not-preloaded icon tint (internal/ui/panel_icon_strip.go), not scheduling
// itself. box == nil means no render box is currently known, so only the decode tier is checked.
func (e *Engine) IsEntryWarm(ent localfs.Entry, box *RenderBox) bool {
	mtime, size := resolveMtimeSize(ent)
	switch {
	case localfs.IsImagePath(ent.Path):
		return e.isImageWarm(ent.Path, mtime, size, box)
	case localfs.IsVideoPath(ent.Path):
		return e.cache.HasVideo(ent.Path, mtime, size, e.cfg.VideoMaxEdgePx, e.cfg.VideoThumbCols, e.cfg.VideoThumbRows)
	default:
		return false
	}
}

func (e *Engine) takeNext() (Item, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for len(e.pending) == 0 {
		select {
		case <-e.ctx.Done():
			return Item{}, false
		default:
		}
		e.cond.Wait()
		if e.ctx.Err() != nil {
			return Item{}, false
		}
	}
	it := e.pending[0]
	e.pending = e.pending[1:]
	return it, true
}

func (e *Engine) worker() {
	defer e.wg.Done()
	for {
		it, ok := e.takeNext()
		if !ok {
			return
		}
		e.runJob(it)
	}
}

func (e *Engine) runJob(it Item) {
	ctx := e.ctx
	switch it.Kind {
	case KindImage:
		pngBytes, _, err := e.cache.LoadStill(ctx, it.Path, it.Mtime, it.Size, e.cfg.ImageMaxEdgePx, func(c context.Context) ([]byte, string, error) {
			return previewrun.DecodeStillMaxEdgePNG(c, it.Path, e.cfg.ImageMaxEdgePx)
		})
		if err != nil {
			return
		}
		box := e.currentRenderBox()
		if box == nil {
			return
		}
		_, _, _, _ = e.cache.LoadRender(ctx, it.Path, it.Mtime, it.Size, box.Proto, box.UnicodePlaceholder, box.InTmux, box.MaxPxW, box.MaxPxH,
			func(c context.Context) ([]byte, int, int, error) {
				return previewrun.EncodeRenderPayload(pngBytes, box.MaxPxW, box.MaxPxH, box.Proto, box.UnicodePlaceholder, box.InTmux)
			})
	case KindVideo:
		maxEdge := e.cfg.VideoMaxEdgePx
		// Duration probe via meta path; skip if no video duration.
		metaRes, work := previewrun.RunMediaMeta(previewrun.Request{
			Path:          it.Path,
			Preview:       config.PreviewConfig{Images: true, VideoThumbCols: e.cfg.VideoThumbCols, VideoThumbRows: e.cfg.VideoThumbRows, ImageMaxEdgePx: maxEdge},
			ImageMaxPxW:   maxEdge,
			ImageMaxPxH:   maxEdge,
			ImageCellPxH:  20,
			ImageProtocol: previewpanel.ImageProtocolSixel,
		})
		_ = metaRes
		if work == nil {
			// No probeable duration (unreadable/corrupt video): record the failure so
			// HasVideo reports warm and Schedule stops re-queuing it every reconcile.
			key := videoKey(it.Path, it.Mtime, it.Size, maxEdge, e.cfg.VideoThumbCols, e.cfg.VideoThumbRows)
			e.cache.markFailed(ctx, key, fmt.Errorf("no video duration"))
			return
		}
		_, _ = e.cache.LoadVideo(ctx, it.Path, it.Mtime, it.Size, maxEdge, e.cfg.VideoThumbCols, e.cfg.VideoThumbRows, nil, func(c context.Context, notify func(done, total int)) ([]byte, error) {
			return previewrun.BuildVideoThumbMaxEdgePNG(c, it.Path, previewrun.MediaThumbDuration(work), e.cfg.VideoThumbCols, e.cfg.VideoThumbRows, maxEdge, e.cfg.VideoThumbWorkers, notify)
		})
	}
}
