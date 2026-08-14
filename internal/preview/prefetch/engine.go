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
	Path     string
	Kind     Kind
	Distance int
	Mtime    int64
	Size     int64
}

// Config configures the prefetch engine.
type Config struct {
	Workers        int
	MemoryMaxMB    int
	VideoDiskMaxMB int
	ImageMaxEdgePx int
	VideoMaxEdgePx int
	VideoThumbCols int
	VideoThumbRows int
	DiskDir        string
	OnChange       func() // optional wake for loading-mark redraw
}

// Engine owns the worker pool and shared Cache.
type Engine struct {
	cache  *Cache
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	pending []Item
	cond    *sync.Cond
	wg      sync.WaitGroup
}

// NewEngine starts workers when workers > 0. Caller must Close.
func NewEngine(parent context.Context, cfg Config) *Engine {
	if cfg.Workers < 1 {
		cfg.Workers = config.DefaultPreviewPrefetchWorkers
	}
	if cfg.MemoryMaxMB < 1 {
		cfg.MemoryMaxMB = config.DefaultPreviewPrefetchMemoryMaxMB
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
			diskDir,
		),
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	e.cond = sync.NewCond(&e.mu)
	if cfg.OnChange != nil {
		e.cache.SetOnChange(cfg.OnChange)
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

// Schedule replaces the pending queue with items sorted by Distance (nearest first).
// Already-warm and non-media entries are dropped.
func (e *Engine) Schedule(items []Item) {
	filtered := make([]Item, 0, len(items))
	for _, it := range items {
		if it.Path == "" {
			continue
		}
		switch it.Kind {
		case KindImage:
			if e.cache.HasStill(it.Path, it.Mtime, it.Size, e.cfg.ImageMaxEdgePx) {
				continue
			}
		case KindVideo:
			if e.cache.HasVideo(it.Path, it.Mtime, it.Size, e.cfg.VideoMaxEdgePx, e.cfg.VideoThumbCols, e.cfg.VideoThumbRows) {
				continue
			}
		default:
			continue
		}
		filtered = append(filtered, it)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Distance < filtered[j].Distance
	})
	e.mu.Lock()
	e.pending = filtered
	e.cond.Broadcast()
	e.mu.Unlock()
}

// ScheduleFromListing builds Items from a panel listing around cursorIdx.
func (e *Engine) ScheduleFromListing(entries []localfs.Entry, cursorIdx int) {
	if len(entries) == 0 {
		e.Schedule(nil)
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
		mtime := ent.ModifiedAt.UnixNano()
		size := ent.Size
		// Prefer live stat when ModifiedAt zero (some listings omit it).
		if ent.ModifiedAt.IsZero() || size == 0 {
			if fi, err := os.Stat(path); err == nil {
				mtime = fi.ModTime().UnixNano()
				size = fi.Size()
			}
		}
		d := i - cursorIdx
		if d < 0 {
			d = -d
		}
		items = append(items, Item{
			Path:     path,
			Kind:     kind,
			Distance: d,
			Mtime:    mtime,
			Size:     size,
		})
	}
	e.Schedule(items)
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
		_, _, _ = e.cache.LoadStill(ctx, it.Path, it.Mtime, it.Size, e.cfg.ImageMaxEdgePx, func(c context.Context) ([]byte, string, error) {
			return previewrun.DecodeStillMaxEdgePNG(c, it.Path, e.cfg.ImageMaxEdgePx)
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
		_, _ = e.cache.LoadVideo(ctx, it.Path, it.Mtime, it.Size, maxEdge, e.cfg.VideoThumbCols, e.cfg.VideoThumbRows, func(c context.Context) ([]byte, error) {
			return previewrun.BuildVideoThumbMaxEdgePNG(c, it.Path, previewrun.MediaThumbDuration(work), e.cfg.VideoThumbCols, e.cfg.VideoThumbRows, maxEdge)
		})
	}
}
