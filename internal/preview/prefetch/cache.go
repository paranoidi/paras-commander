package prefetch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func stillKey(path string, mtime, size int64, maxEdge int) string {
	return fmt.Sprintf("still\x00%s\x00%d\x00%d\x00%d", path, mtime, size, maxEdge)
}

func videoKey(path string, mtime, size int64, maxEdge, cols, rows int) string {
	return fmt.Sprintf("video\x00%s\x00%d\x00%d\x00%d\x00%d\x00%d", path, mtime, size, maxEdge, cols, rows)
}

// renderKey identifies a final render-ready payload: decode+resize+protocol-encode output for
// one exact on-screen pixel box, distinct from stillKey's maxEdge-clamped intermediate PNG.
func renderKey(path string, mtime, size int64, proto previewpanel.ImageProtocol, unicodePlaceholder, inTmux bool, maxPxW, maxPxH int) string {
	return fmt.Sprintf("render\x00%s\x00%d\x00%d\x00%d\x00%t\x00%t\x00%d\x00%d",
		path, mtime, size, proto, unicodePlaceholder, inTmux, maxPxW, maxPxH)
}

// packRenderMeta/parseRenderMeta stash the render payload's pixel dimensions in memoryLRU's meta
// string field, since (png []byte, meta string) doesn't have room for a second int pair.
func packRenderMeta(w, h int) string {
	return fmt.Sprintf("%d,%d", w, h)
}

func parseRenderMeta(meta string) (w, h int) {
	before, after, ok := strings.Cut(meta, ",")
	if !ok {
		return 0, 0
	}
	w, _ = strconv.Atoi(before)
	h, _ = strconv.Atoi(after)
	return w, h
}

type flightCall struct {
	done chan struct{}
	png  []byte
	meta string
	err  error

	listenersMu sync.Mutex
	listeners   []func(done, total int)
}

// addListener registers a progress listener that receives every subsequent notify call, even
// from a caller that joined after the flight's work already started.
func (f *flightCall) addListener(fn func(done, total int)) {
	f.listenersMu.Lock()
	f.listeners = append(f.listeners, fn)
	f.listenersMu.Unlock()
}

// notify broadcasts a progress update to every registered listener, including callers that
// joined an already-running flight rather than started it.
func (f *flightCall) notify(done, total int) {
	f.listenersMu.Lock()
	listeners := f.listeners
	f.listenersMu.Unlock()
	for _, fn := range listeners {
		fn(done, total)
	}
}

// Cache implements preview.MediaCache with memory LRU + video disk store + singleflight.
type Cache struct {
	mem    *memoryLRU
	render *memoryLRU // final render-ready payloads, separate budget from mem so per-render
	// writes never evict not-yet-consumed prefetched maxEdge PNGs.
	disk *diskCache

	mu       sync.Mutex
	inflight map[string]int // path -> refcount (for loading mark)
	flights  map[string]*flightCall
	failed   map[string]error // still/video key -> permanent decode error, until path/mtime/size/size changes the key
	onChange func()
}

// NewCache builds a MediaCache. diskDir may be empty to disable disk persistence.
func NewCache(memoryMaxBytes, diskMaxBytes, renderMaxBytes int64, diskDir string) *Cache {
	c := &Cache{
		mem:      newMemoryLRU(memoryMaxBytes),
		render:   newMemoryLRU(renderMaxBytes),
		inflight: make(map[string]int),
		flights:  make(map[string]*flightCall),
		failed:   make(map[string]error),
	}
	if diskDir != "" {
		c.disk = newDiskCache(diskDir, diskMaxBytes)
	}
	return c
}

// SetOnChange registers a callback invoked when the in-flight set changes.
func (c *Cache) SetOnChange(fn func()) {
	c.mu.Lock()
	c.onChange = fn
	c.mu.Unlock()
}

func (c *Cache) notifyLocked() {
	fn := c.onChange
	if fn == nil {
		return
	}
	// Unlock before calling — UI wake must not run under cache mu.
	c.mu.Unlock()
	fn()
	c.mu.Lock()
}

// InFlight reports whether path is currently being loaded.
func (c *Cache) InFlight(path string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inflight[path] > 0
}

// SnapshotInFlight returns absolute paths currently in flight.
func (c *Cache) SnapshotInFlight() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.inflight))
	for p := range c.inflight {
		out = append(out, p)
	}
	return out
}

// HasStill reports a warm memory hit, or a previously recorded permanent decode failure,
// without marking in-flight.
func (c *Cache) HasStill(path string, mtime, size int64, maxEdge int) bool {
	key := stillKey(path, mtime, size, maxEdge)
	return c.mem.has(key) || c.isFailed(key)
}

// HasVideo reports a warm memory/disk hit, or a previously recorded permanent decode failure,
// without marking in-flight.
func (c *Cache) HasVideo(path string, mtime, size int64, maxEdge, cols, rows int) bool {
	key := videoKey(path, mtime, size, maxEdge, cols, rows)
	if c.mem.has(key) || c.isFailed(key) {
		return true
	}
	if c.disk == nil {
		return false
	}
	dk := videoDiskKey(path, mtime, size, maxEdge, cols, rows)
	_, ok := c.disk.get(dk)
	return ok
}

// HasRender reports a warm memory hit for the exact render-payload box, without marking in-flight.
// Unlike HasStill/HasVideo, a permanent LoadRender failure isn't memoized (LoadRender doesn't call
// markFailed — see its doc comment), so a render that keeps failing will keep getting retried by
// Schedule's dedup check below; that's an accepted tradeoff since the retried work (resize+encode
// of an already-decoded image) is cheap, unlike a full decode retry.
func (c *Cache) HasRender(path string, mtime, size int64, proto previewpanel.ImageProtocol, unicodePlaceholder, inTmux bool, maxPxW, maxPxH int) bool {
	key := renderKey(path, mtime, size, proto, unicodePlaceholder, inTmux, maxPxW, maxPxH)
	return c.render.has(key)
}

// isFailed reports whether key is a previously recorded permanent decode failure.
func (c *Cache) isFailed(key string) bool {
	_, ok := c.failedErr(key)
	return ok
}

// failedErr returns the previously recorded permanent decode failure for key, if any.
func (c *Cache) failedErr(key string) (error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	err, ok := c.failed[key]
	return err, ok
}

// markFailed records a permanent decode failure for key so HasStill/HasVideo treat it as
// warm and callers (e.g. the prefetch engine's Schedule) stop re-queuing it every reconcile.
// Context cancellation isn't recorded — the load may simply not have finished yet.
func (c *Cache) markFailed(ctx context.Context, key string, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	c.mu.Lock()
	c.failed[key] = err
	c.mu.Unlock()
}

func (c *Cache) doFlight(key, path string, onProgress func(done, total int), run func(notify func(done, total int)) (png []byte, meta string, err error)) (png []byte, meta string, err error) {
	c.mu.Lock()
	if call, ok := c.flights[key]; ok {
		if onProgress != nil {
			call.addListener(onProgress)
		}
		c.mu.Unlock()
		<-call.done
		return call.png, call.meta, call.err
	}
	call := &flightCall{done: make(chan struct{})}
	if onProgress != nil {
		call.addListener(onProgress)
	}
	c.flights[key] = call
	c.inflight[path]++
	c.notifyLocked()
	c.mu.Unlock()

	png, meta, err = run(call.notify)

	c.mu.Lock()
	call.png, call.meta, call.err = png, meta, err
	delete(c.flights, key)
	if n := c.inflight[path]; n <= 1 {
		delete(c.inflight, path)
	} else {
		c.inflight[path] = n - 1
	}
	c.notifyLocked()
	close(call.done)
	c.mu.Unlock()
	return png, meta, err
}

// LoadStill implements preview.MediaCache.
func (c *Cache) LoadStill(ctx context.Context, path string, mtime, size int64, maxEdge int, load func(context.Context) (png []byte, meta string, err error)) (png []byte, meta string, err error) {
	key := stillKey(path, mtime, size, maxEdge)
	if png, meta, ok := c.mem.get(key); ok {
		return png, meta, nil
	}
	if ferr, ok := c.failedErr(key); ok {
		return nil, "", ferr
	}
	return c.doFlight(key, path, nil, func(func(done, total int)) ([]byte, string, error) {
		if png, meta, ok := c.mem.get(key); ok {
			return png, meta, nil
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		default:
		}
		png, meta, err := load(ctx)
		if err != nil {
			c.markFailed(ctx, key, err)
			return nil, meta, err
		}
		c.mem.put(key, png, meta)
		return png, meta, nil
	})
}

// LoadRender implements preview.MediaCache. It caches the final protocol-encoded payload for one
// exact on-screen pixel box, so re-rendering an already-seen box (e.g. landing back on a file at
// unchanged panel geometry) skips decode/resize/encode entirely.
func (c *Cache) LoadRender(ctx context.Context, path string, mtime, size int64,
	proto previewpanel.ImageProtocol, unicodePlaceholder, inTmux bool, maxPxW, maxPxH int,
	load func(context.Context) (payload []byte, w, h int, err error),
) (payload []byte, w, h int, err error) {
	key := renderKey(path, mtime, size, proto, unicodePlaceholder, inTmux, maxPxW, maxPxH)
	if payload, meta, ok := c.render.get(key); ok {
		w, h := parseRenderMeta(meta)
		return payload, w, h, nil
	}
	payload, meta, err := c.doFlight(key, path, nil, func(func(done, total int)) ([]byte, string, error) {
		if payload, meta, ok := c.render.get(key); ok {
			return payload, meta, nil
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		default:
		}
		payload, w, h, err := load(ctx)
		if err != nil {
			return nil, "", err
		}
		meta := packRenderMeta(w, h)
		c.render.put(key, payload, meta)
		return payload, meta, nil
	})
	if err != nil {
		return nil, 0, 0, err
	}
	w, h = parseRenderMeta(meta)
	return payload, w, h, nil
}

// LoadVideo implements preview.MediaCache. onProgress, if non-nil, is called with each frame
// completed by load — even when this call joins a flight already started by another caller
// (e.g. the background prefetch engine), since progress is broadcast to every joiner.
func (c *Cache) LoadVideo(ctx context.Context, path string, mtime, size int64, maxEdge, cols, rows int, onProgress func(done, total int), load func(context.Context, func(done, total int)) (png []byte, err error)) (png []byte, err error) {
	key := videoKey(path, mtime, size, maxEdge, cols, rows)
	if png, _, ok := c.mem.get(key); ok {
		return png, nil
	}
	dk := videoDiskKey(path, mtime, size, maxEdge, cols, rows)
	if c.disk != nil {
		if b, ok := c.disk.get(dk); ok {
			c.mem.put(key, b, "")
			return b, nil
		}
	}
	if ferr, ok := c.failedErr(key); ok {
		return nil, ferr
	}
	png, _, err = c.doFlight(key, path, onProgress, func(notify func(done, total int)) ([]byte, string, error) {
		if png, _, ok := c.mem.get(key); ok {
			return png, "", nil
		}
		if c.disk != nil {
			if b, ok := c.disk.get(dk); ok {
				c.mem.put(key, b, "")
				return b, "", nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		default:
		}
		png, err := load(ctx, notify)
		if err != nil {
			c.markFailed(ctx, key, err)
			return nil, "", err
		}
		c.mem.put(key, png, "")
		if c.disk != nil {
			_ = c.disk.put(dk, png)
		}
		return png, "", nil
	})
	return png, err
}
