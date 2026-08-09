package prefetch

import (
	"context"
	"fmt"
	"sync"
)

func stillKey(path string, mtime, size int64, maxEdge int) string {
	return fmt.Sprintf("still\x00%s\x00%d\x00%d\x00%d", path, mtime, size, maxEdge)
}

func videoKey(path string, mtime, size int64, maxEdge, cols, rows int) string {
	return fmt.Sprintf("video\x00%s\x00%d\x00%d\x00%d\x00%d\x00%d", path, mtime, size, maxEdge, cols, rows)
}

type flightCall struct {
	done chan struct{}
	png  []byte
	meta string
	err  error
}

// Cache implements preview.MediaCache with memory LRU + video disk store + singleflight.
type Cache struct {
	mem  *memoryLRU
	disk *diskCache

	mu       sync.Mutex
	inflight map[string]int // path -> refcount (for loading mark)
	flights  map[string]*flightCall
	failed   map[string]error // still/video key -> permanent decode error, until path/mtime/size/size changes the key
	onChange func()
}

// NewCache builds a MediaCache. diskDir may be empty to disable disk persistence.
func NewCache(memoryMaxBytes, diskMaxBytes int64, diskDir string) *Cache {
	c := &Cache{
		mem:      newMemoryLRU(memoryMaxBytes),
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

func (c *Cache) doFlight(key, path string, run func() (png []byte, meta string, err error)) (png []byte, meta string, err error) {
	c.mu.Lock()
	if call, ok := c.flights[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.png, call.meta, call.err
	}
	call := &flightCall{done: make(chan struct{})}
	c.flights[key] = call
	c.inflight[path]++
	c.notifyLocked()
	c.mu.Unlock()

	png, meta, err = run()

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
	return c.doFlight(key, path, func() ([]byte, string, error) {
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

// LoadVideo implements preview.MediaCache.
func (c *Cache) LoadVideo(ctx context.Context, path string, mtime, size int64, maxEdge, cols, rows int, load func(context.Context) (png []byte, err error)) (png []byte, err error) {
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
	png, _, err = c.doFlight(key, path, func() ([]byte, string, error) {
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
		png, err := load(ctx)
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
