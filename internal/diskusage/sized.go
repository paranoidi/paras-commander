package diskusage

import (
	"context"
	"path/filepath"
	"strings"
)

// SubtreeSizer totals a path under a sizer prefix (a file's own size and 1, or a directory's
// bytes and files) without walking the disk; an error leaves the path uncached.
type SubtreeSizer func(ctx context.Context, path string) (bytes, files int64, err error)

// CleanKey cleans a cache key: a host path with filepath.Clean, a URI (scheme://path) keeping
// its scheme prefix whole (filepath.Clean would collapse the "//").
func CleanKey(p string) string {
	if i := strings.Index(p, "://"); i > 0 && !strings.ContainsRune(p[:i], '/') {
		return p[:i+3] + filepath.Clean(p[i+3:])
	}
	return filepath.Clean(p)
}

// SetSubtreeSizer makes paths starting with prefix (e.g. "fileindex://") take their totals from
// sizer instead of a disk walk. Call it before the first scan.
func (e *Engine) SetSubtreeSizer(prefix string, sizer SubtreeSizer) {
	e.sizerPrefix, e.sizer = prefix, sizer
}

// sized reports whether p takes its total from the subtree sizer.
func (e *Engine) sized(p string) bool {
	return e.sizer != nil && strings.HasPrefix(p, e.sizerPrefix)
}

// ForgetSized drops every cached total under the sizer prefix, after the data behind it changed.
// A sizer query in flight when this runs is not cached.
func (e *Engine) ForgetSized() {
	if e == nil || e.sizer == nil {
		return
	}
	e.sizedEpoch.Add(1)
	e.mu.Lock()
	for k := range e.sizedKeys {
		delete(e.cache, k)
		delete(e.fileCounts, k)
	}
	clear(e.sizedKeys)
	e.cacheVersion.Add(1)
	e.mu.Unlock()
	e.poke()
}

// cacheSized asks the sizer for p's total and caches it; false when the session went stale or
// the cache was forgotten meanwhile, or the sizer failed (p stays uncached).
func (e *Engine) cacheSized(ctx context.Context, sess uint64, p string) bool {
	epoch := e.sizedEpoch.Load()
	bytes, files, err := e.sizer(ctx, p)
	if err != nil || e.gen.Load() != sess {
		return false
	}
	e.mu.Lock()
	// Checked under mu: ForgetSized bumps the epoch before it takes mu, so either this sees the
	// bump or ForgetSized drops what this writes.
	if e.sizedEpoch.Load() != epoch {
		e.mu.Unlock()
		return false
	}
	e.cache[p] = bytes
	e.fileCounts[p] = files
	e.sizedKeys[p] = struct{}{}
	e.cacheVersion.Add(1)
	e.mu.Unlock()
	return true
}
