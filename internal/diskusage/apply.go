package diskusage

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/fschange"
)

// Apply queues a batch of filesystem changes (fschange.Sink). It only appends under a small lock,
// so job workers and the UI goroutine can call it freely; the engine's apply goroutine applies
// everything queued in one pass, in order, and the next pass takes whatever arrived meanwhile.
func (e *Engine) Apply(batch []fschange.Change) {
	if e == nil || len(batch) == 0 {
		return
	}
	e.applyMu.Lock()
	e.pending = append(e.pending, batch)
	e.applyQueued.Add(1)
	e.applyMu.Unlock()
	select {
	case e.applyWake <- struct{}{}:
	default:
	}
}

func (e *Engine) applyLoop() {
	for range e.applyWake {
		e.applyMu.Lock()
		batches := e.pending
		e.pending = nil
		e.applyMu.Unlock()
		if len(batches) == 0 {
			continue
		}
		e.applyBatches(batches)
		e.applyDone.Add(uint64(len(batches)))
	}
}

// applyIdle reports whether every queued batch has been applied (tests).
func (e *Engine) applyIdle() bool {
	return e.applyDone.Load() == e.applyQueued.Load()
}

// applyBatches applies batches in order under one hold of e.mu. Renames and removals collect into
// a sweep group so a burst (a merge move's children) costs one pass over the cache; the group is
// flushed before any change that touches a path in it, so ordering between overlapping changes is
// kept. Creations apply at once: their ancestor deltas are additive. A rename from an uncached
// path (an arrival) applies after the rest of its batch has landed, since the batch's renames are
// simultaneous and the arrival may replace one of their sources.
func (e *Engine) applyBatches(batches [][]fschange.Change) {
	var refresh []string
	e.mu.Lock()
	g := newSweepGroup()
	for _, batch := range batches {
		if g.overlaps(batch) {
			e.flushSweepLocked(g)
			g = newSweepGroup()
		}
		var arrivals []fschange.Change
		for _, c := range batch {
			e.touchWalkRootsLocked(c)
			if _, cached := e.cache[c.From]; c.Kind == fschange.Renamed && !cached {
				arrivals = append(arrivals, c)
			} else if e.applyChangeLocked(g, c) {
				refresh = append(refresh, c.Path)
			}
		}
		if len(arrivals) > 0 && len(arrivals) < len(batch) {
			e.flushSweepLocked(g)
			g = newSweepGroup()
		}
		for _, c := range arrivals {
			if e.arrivedLocked(g, c) {
				refresh = append(refresh, c.Path)
			}
		}
	}
	e.flushSweepLocked(g)
	e.cacheVersion.Add(1)
	e.mu.Unlock()
	e.poke()
	e.queueRefresh(refresh)
}

// touchWalkRootsLocked marks every active walk root that c's paths overlap, so the planner walks
// it again instead of merging sizes read before c. Caller holds e.mu.
func (e *Engine) touchWalkRootsLocked(c fschange.Change) {
	overlaps := func(p, r string) bool { return p != "" && (pathIsOrUnder(p, r) || pathIsOrUnder(r, p)) }
	for r := range e.activeWalkRoots {
		if overlaps(c.Path, r) || overlaps(c.From, r) {
			e.touchedWalkRoots[r] = true
		}
	}
}

// applyChangeLocked applies c, or adds it to g when it needs the sweep. It reports whether c.Path
// now has an unknown size under a scanned directory and needs a refresh scan. Caller holds e.mu.
func (e *Engine) applyChangeLocked(g *sweepGroup, c fschange.Change) (refresh bool) {
	switch c.Kind {
	case fschange.Renamed:
		g.rename(c.From, c.Path, c.IsDir)
	case fschange.Dirty:
		return e.dirtyLocked(g, c.Path, c.IsDir)
	case fschange.Removed:
		g.remove(c.Path, c.IsDir)
	case fschange.Created:
		e.createdLocked(c.Path, c.Bytes, c.IsDir)
	}
	return false
}

// arrivedLocked applies a rename from an uncached (unscanned) path: a file counts like a
// creation; a directory's size is unknown, so it is handled like a dirty path. It reports whether
// a refresh scan is needed. Caller holds e.mu.
func (e *Engine) arrivedLocked(g *sweepGroup, c fschange.Change) (refresh bool) {
	if c.IsDir {
		return e.dirtyLocked(g, c.Path, true)
	}
	e.createdLocked(c.Path, c.Bytes, false)
	return false
}

// dirtyLocked drops p, whose size is now unknown, and marks it pending refresh when something
// above it is cached. It reports whether a refresh scan must be queued. Caller holds e.mu.
func (e *Engine) dirtyLocked(g *sweepGroup, p string, isDir bool) bool {
	_, scanned := nearestAncestor(p, e.cache)
	g.remove(p, isDir)
	if !scanned || e.refreshPending[p] {
		return false // nothing to correct, or a refresh is already on its way
	}
	e.refreshPending[p] = true
	return true
}

// SetRefreshIgnore sets the ignore rule (the app's disk-usage ignore) and mount-point setting for
// the refresh scans the engine starts on its own for dirty paths.
func (e *Engine) SetRefreshIgnore(ignore ShouldIgnoreFolder, descendIntoMountPoints bool) {
	if e == nil {
		return
	}
	e.jobMu.Lock()
	e.refreshIgnore, e.refreshDescend = ignore, descendIntoMountPoints
	e.jobMu.Unlock()
}

// queueRefresh queues a refresh scan of each newly pending path, one job per path so the volume
// gate measures that path's own device, next in line like StartScanFromListing. Any walk that
// reaches the path first settles it instead (refreshPending).
func (e *Engine) queueRefresh(paths []string) {
	if len(paths) == 0 {
		return
	}
	e.jobMu.Lock()
	defer e.jobMu.Unlock()
	add := make([]scanJob, 0, len(paths))
	for _, p := range paths {
		add = append(add, scanJob{
			childAbs:       []string{p},
			ignore:         e.refreshIgnore,
			sourcePanel:    anyPanel,
			listingVolGate: ListingVolumeGate{Enabled: !e.refreshDescend, FromRoot: true},
		})
	}
	e.queue = append(add, e.queue...)
	e.jobCond.Signal()
}

// createdLocked counts a new path, or a file overwritten in place, into its cached ancestors. A
// walk caches every node, so when a scanned directory holds the path it gets its own entry, and
// so does every directory between them that appeared without a report (mkdir -p); a later removal
// can then subtract it. A cached directory is left alone and a cached file is replaced, so a walk
// that already saw the change is not counted twice. Nothing below an excluded directory is
// counted, as walks skip them. Caller holds e.mu.
func (e *Engine) createdLocked(p string, bytes int64, isDir bool) {
	old, had := e.cache[p]
	if isDir {
		if had {
			return
		}
		bytes = 0
	}
	var files, oldFiles int64
	if !isDir {
		files = 1
		if had {
			oldFiles = e.fileCounts[p]
		}
	}
	scanned := e.growAncestorsLocked(p, ancestorDelta{bytes - old, files - oldFiles}, ancestorDelta{bytes, files})
	if scanned || had { // an overwritten file with no scanned ancestor still keeps its entry current
		e.cache[p], e.fileCounts[p] = bytes, files
	}
}

// growAncestorsLocked moves every cached ancestor of p by d, stopping at an excluded one (walks
// never counted below it) and clamping at zero. A walk caches every node, so the directories
// between p and its nearest cached ancestor are new: they get total as their own entry. It
// reports whether any ancestor is cached. Caller holds e.mu.
func (e *Engine) growAncestorsLocked(p string, d, total ancestorDelta) bool {
	deltas := ancestorDeltas{}
	var missing []string
	for a := p; ; {
		parent := filepath.Dir(a)
		if parent == a || e.excluded[parent] {
			break
		}
		a = parent
		if _, ok := e.cache[a]; ok {
			deltas[a] = d
		} else if len(deltas) == 0 {
			missing = append(missing, a)
		}
	}
	if len(deltas) == 0 {
		return false
	}
	e.applyAncestorDeltasLocked(deltas)
	for _, m := range missing {
		e.cache[m], e.fileCounts[m] = total.bytes, total.files
	}
	return true
}

// sweepGroup is a set of renames and removals applied together. Paths of one group never overlap
// except within a single batch, whose renames are simultaneous (swap semantics).
type sweepGroup struct {
	renames map[string]string // from -> to
	targets map[string]string // to -> from
	removed map[string]bool
	hasDir  bool
	// paths holds every path in the group; ancestors every strict ancestor of one, so overlap
	// checks are O(depth) per change in both directions.
	paths     map[string]struct{}
	ancestors map[string]struct{}
}

func newSweepGroup() *sweepGroup {
	return &sweepGroup{
		renames:   map[string]string{},
		targets:   map[string]string{},
		removed:   map[string]bool{},
		paths:     map[string]struct{}{},
		ancestors: map[string]struct{}{},
	}
}

func (g *sweepGroup) empty() bool { return len(g.renames)+len(g.removed) == 0 }

func (g *sweepGroup) rename(from, to string, isDir bool) {
	g.renames[from] = to
	g.targets[to] = from
	g.hasDir = g.hasDir || isDir
	g.track(from)
	g.track(to)
}

func (g *sweepGroup) remove(p string, isDir bool) {
	g.removed[p] = true
	g.hasDir = g.hasDir || isDir
	g.track(p)
}

func (g *sweepGroup) track(p string) {
	g.paths[p] = struct{}{}
	for a := filepath.Dir(p); ; a = filepath.Dir(a) {
		if _, ok := g.ancestors[a]; ok {
			return
		}
		g.ancestors[a] = struct{}{}
		if filepath.Dir(a) == a {
			return
		}
	}
}

// overlaps reports whether any path of batch is in g, under a path in g, or above one.
func (g *sweepGroup) overlaps(batch []fschange.Change) bool {
	if g.empty() {
		return false
	}
	hit := func(p string) bool {
		if p == "" {
			return false
		}
		if _, ok := g.ancestors[p]; ok {
			return true
		}
		_, ok := nearestAncestor(p, g.paths)
		return ok
	}
	for _, c := range batch {
		if hit(c.Path) || hit(c.From) {
			return true
		}
	}
	return false
}

// flushSweepLocked applies g: cached ancestor totals first (from the state before the group),
// then the keys. Only a group holding a directory sweeps the whole cache; files have no
// descendants, so their keys move directly. Caller holds e.mu.
func (e *Engine) flushSweepLocked(g *sweepGroup) {
	if g.empty() {
		return
	}
	d := ancestorDeltas{}
	for from, to := range g.renames {
		if nestedInOther(from, g.renames) {
			continue // carried inside the outer rename's size
		}
		e.addAncestorDeltasLocked(d, from, from, -1)
		e.addAncestorDeltasLocked(d, from, to, +1)
		if _, moving := g.renames[to]; !moving {
			e.addAncestorDeltasLocked(d, to, to, -1) // the tree this move replaced
		}
	}
	for r := range g.removed {
		if !nestedInOther(r, g.removed) { // a nested root's size is inside the outer root's
			e.addAncestorDeltasLocked(d, r, r, -1)
		}
	}
	e.applyAncestorDeltasLocked(d)
	if g.hasDir {
		e.sweeps.Add(1)
		rebase := func(k string) (string, bool) {
			if p, ok := nearestAncestor(k, g.renames); ok {
				return g.renames[p] + k[len(p):], true
			}
			return "", false
		}
		drop := func(k string) bool {
			if _, ok := nearestAncestor(k, g.targets); ok {
				return true
			}
			_, ok := nearestAncestor(k, g.removed)
			return ok
		}
		rebaseKeys(e.cache, rebase, drop)
		rebaseKeys(e.fileCounts, rebase, drop)
		rebaseKeys(e.excluded, rebase, drop)
		return
	}
	moveKeys(e.cache, g)
	moveKeys(e.fileCounts, g)
	moveKeys(e.excluded, g)
}

// moveKeys is rebaseKeys for a group of files only: each key is its own whole subtree.
func moveKeys[V any](m map[string]V, g *sweepGroup) {
	moved := map[string]V{}
	for from, to := range g.renames {
		if v, ok := m[from]; ok {
			moved[to] = v
		}
	}
	for from, to := range g.renames {
		delete(m, from)
		delete(m, to)
	}
	for r := range g.removed {
		delete(m, r)
	}
	for k, v := range moved {
		m[k] = v
	}
}
