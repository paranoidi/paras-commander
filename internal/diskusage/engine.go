package diskusage

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/paranoidi/paras-commander/internal/config"
)

type scanJob struct {
	childAbs       []string
	ignore         ShouldIgnoreFolder
	sourcePanel    int // 0=left, 1=right; matches ui.LeftPanel / ui.RightPanel.
	listingVolGate ListingVolumeGate
}

// Engine tracks cached subtree sizes and runs sequential subtree walks keyed by listing children.
type Engine struct {
	mu sync.RWMutex

	cache map[string]int64
	// activeWalkRoots maps an in-flight walk root to the panel that started that subtree scan.
	activeWalkRoots map[string]int

	updates chan struct{}

	// events carries subtree-complete and job-finished notifications for idle-sort UX etc.
	events chan Event

	// gen increments on each dequeued scan session and on Abort; planner compares against the
	// generation it started with to drop stale walk results.
	gen atomic.Uint64

	jobMu   sync.Mutex
	jobCond *sync.Cond
	queue   []scanJob
	// curJobRoots lists top-level roots of the job currently processed by the worker that are
	// not finished yet (walk not completed). Nil when idle between jobs.
	curJobRoots map[string]struct{}
	// curJobSourcePanel is the panel that queued the current job (valid while curJobRoots != nil).
	curJobSourcePanel int

	// runPlannerHook, when non-nil (tests only), replaces runPlanner in the worker.
	runPlannerHook func(sess uint64, childAbs []string, shouldIgnore ShouldIgnoreFolder, sourcePanel int)

	// workerBusy is true while the worker is executing a dequeued scan job (runPlanner or hook).
	workerBusy atomic.Bool

	// walkConcurrency caps concurrent subdirectory walks in WalkFolder (minimum 1).
	walkConcurrency int
}

const cacheMergeChunkSize = 4096

// New returns an engine with the same default walk concurrency as config.Default().
func New() *Engine {
	return NewWithWalkConcurrency(config.DefaultDiskUsageWalkConcurrency)
}

// NewWithWalkConcurrency creates an engine. walkConcurrency below 1 is replaced with config.DefaultDiskUsageWalkConcurrency.
func NewWithWalkConcurrency(walkConcurrency int) *Engine {
	if walkConcurrency < 1 {
		walkConcurrency = config.DefaultDiskUsageWalkConcurrency
	}
	e := &Engine{
		cache:           make(map[string]int64),
		activeWalkRoots: make(map[string]int),
		updates:         make(chan struct{}, 1),
		events:          make(chan Event, 256),
		walkConcurrency: walkConcurrency,
	}
	e.jobCond = sync.NewCond(&e.jobMu)
	go e.workerLoop()
	return e
}

func (e *Engine) workerLoop() {
	for {
		e.jobMu.Lock()
		for len(e.queue) == 0 {
			e.jobCond.Wait()
		}
		job := e.queue[0]
		e.queue = e.queue[1:]
		roots := make(map[string]struct{}, len(job.childAbs))
		for _, raw := range job.childAbs {
			roots[filepath.Clean(raw)] = struct{}{}
		}
		e.curJobRoots = roots
		e.curJobSourcePanel = job.sourcePanel
		e.jobMu.Unlock()

		func() {
			defer func() {
				e.jobMu.Lock()
				e.curJobRoots = nil
				e.curJobSourcePanel = -1
				e.jobMu.Unlock()
			}()
			e.workerBusy.Store(true)
			defer e.workerBusy.Store(false)

			sess := e.gen.Add(1)
			combined := ComposeListingVolumeIgnore(job.ignore, job.listingVolGate)
			if e.runPlannerHook != nil {
				e.runPlannerHook(sess, job.childAbs, combined, job.sourcePanel)
			} else {
				e.runPlanner(sess, job.childAbs, combined, job.sourcePanel)
			}
		}()
	}
}

// Updates notifies the UI that cache or pending scans changed (coalesced).
func (e *Engine) Updates() <-chan struct{} {
	return e.updates
}

// Events returns structured disk-scan notifications (subtree indexed, job finished).
func (e *Engine) Events() <-chan Event {
	return e.events
}

func (e *Engine) poke() {
	select {
	case e.updates <- struct{}{}:
	default:
	}
}

func (e *Engine) signalJobFinished(sess uint64) {
	select {
	case e.events <- Event{Kind: EventJobFinished, Generation: sess}:
	default:
	}
}

func (e *Engine) signalSubtreeIndexed(sess uint64, rootAbs string, sourcePanel int) {
	select {
	case e.events <- Event{
		Kind:        EventSubtreeIndexed,
		Generation:  sess,
		RootAbs:     filepath.Clean(rootAbs),
		SourcePanel: sourcePanel,
	}:
	default:
	}
}

// DiskScanBusy reports whether a scan job is queued or subtree walks are still in progress.
// It is independent of the current panel or directory listing.
func (e *Engine) DiskScanBusy() bool {
	if e == nil {
		return false
	}
	if e.workerBusy.Load() {
		return true
	}
	e.jobMu.Lock()
	queued := len(e.queue)
	e.jobMu.Unlock()
	e.mu.RLock()
	activeN := len(e.activeWalkRoots)
	e.mu.RUnlock()
	return queued > 0 || activeN > 0
}

// pathIsOrUnder reports whether child is root or a strict descendant of root (path segments).
func pathIsOrUnder(child, root string) bool {
	r := filepath.Clean(root)
	c := filepath.Clean(child)
	if c == r {
		return true
	}
	return strings.HasPrefix(c, r+string(filepath.Separator))
}

// PendingForPanel reports whether childAbs should show the disk-scan folder tint for the given
// panel (ui.LeftPanel=0, ui.RightPanel=1): queued for that panel, walking for that panel, or
// under such a root. Scans started from another panel do not affect this panel's tint.
func (e *Engine) PendingForPanel(childAbs string, panelID int) bool {
	if e == nil {
		return false
	}
	c := filepath.Clean(childAbs)

	e.mu.RLock()
	for r, pan := range e.activeWalkRoots {
		if pan == panelID && pathIsOrUnder(c, r) {
			e.mu.RUnlock()
			return true
		}
	}
	e.mu.RUnlock()

	e.jobMu.Lock()
	for _, j := range e.queue {
		if j.sourcePanel != panelID {
			continue
		}
		for _, raw := range j.childAbs {
			if pathIsOrUnder(c, filepath.Clean(raw)) {
				e.jobMu.Unlock()
				return true
			}
		}
	}
	if e.curJobRoots != nil && e.curJobSourcePanel == panelID {
		for r := range e.curJobRoots {
			if pathIsOrUnder(c, r) {
				e.jobMu.Unlock()
				return true
			}
		}
	}
	e.jobMu.Unlock()
	return false
}

func (e *Engine) finishCurJobRoot(abs string) {
	if e == nil {
		return
	}
	p := filepath.Clean(abs)
	e.jobMu.Lock()
	if e.curJobRoots != nil {
		delete(e.curJobRoots, p)
	}
	e.jobMu.Unlock()
}

// Size returns cached subtree or file aggregate if present for absPath.
func (e *Engine) Size(absPath string) (int64, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	n, ok := e.cache[filepath.Clean(absPath)]
	return n, ok
}

// ByteSize and PendingForPanel implement ui.DiskUsagePainter for the render layer.
func (e *Engine) ByteSize(absPath string) (int64, bool) {
	return e.Size(absPath)
}

// DiskScanExcluded implements ui.DiskUsagePainter.
func (e *Engine) DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool {
	if e == nil {
		return false
	}
	var gi ShouldIgnoreFolder
	if goduIgnore != nil {
		gi = goduIgnore
	}
	return ScanExcluded(absPath, descendIntoMountPoints, listingDev, listingDevValid, gi)
}

// Abort invalidates in-flight scan work; results from walks already running are dropped if they finish after this bump.
func (e *Engine) Abort() {
	e.gen.Add(1)

	e.jobMu.Lock()
	e.queue = nil
	e.curJobRoots = nil
	e.curJobSourcePanel = -1
	e.jobMu.Unlock()
	e.jobCond.Broadcast()

	e.mu.Lock()
	for p := range e.activeWalkRoots {
		delete(e.activeWalkRoots, p)
	}
	e.mu.Unlock()

	e.poke()
}

// StartScanFromListing walks each path in childAbs sequentially in two passes (unknown subtree keys first, then refreshes known keys).
//
// Caller supplies absolute filepath.Clean child paths consistent with filesystem listing Paths.
// sourcePanel is the panel that initiated the scan (same values as ui.LeftPanel / ui.RightPanel).
func (e *Engine) StartScanFromListing(childAbs []string, shouldIgnore ShouldIgnoreFolder, sourcePanel int, volGate ListingVolumeGate) {
	if e == nil {
		return
	}
	dup := append([]string(nil), childAbs...)
	e.jobMu.Lock()
	e.queue = append([]scanJob{{childAbs: dup, ignore: shouldIgnore, sourcePanel: sourcePanel, listingVolGate: volGate}}, e.queue...)
	e.jobCond.Signal()
	e.jobMu.Unlock()
}

func (e *Engine) runPlanner(sess uint64, childAbs []string, shouldIgnore ShouldIgnoreFolder, sourcePanel int) {
	if len(childAbs) == 0 {
		if e.gen.Load() == sess {
			e.signalJobFinished(sess)
		}
		return
	}

	type stage struct {
		path string
	}

	passUnknown := []stage{}
	passKnown := []stage{}

	e.mu.RLock()
	for _, raw := range childAbs {
		p := filepath.Clean(raw)
		if _, cached := e.cache[p]; cached {
			passKnown = append(passKnown, stage{path: p})
			continue
		}
		passUnknown = append(passUnknown, stage{path: p})
	}
	e.mu.RUnlock()

	passes := [][]stage{passUnknown, passKnown}

	for _, grp := range passes {
		for _, job := range grp {
			if e.gen.Load() != sess {
				return
			}

			jobPath := filepath.Clean(job.path)

			e.mu.Lock()
			e.activeWalkRoots[jobPath] = sourcePanel
			e.mu.Unlock()
			e.poke()

			fi, err := os.Stat(jobPath)
			if err != nil {
				e.mu.Lock()
				delete(e.activeWalkRoots, jobPath)
				e.mu.Unlock()
				e.finishCurJobRoot(jobPath)
				e.poke()
				continue
			}
			if !fi.IsDir() {
				if e.gen.Load() != sess {
					e.mu.Lock()
					delete(e.activeWalkRoots, jobPath)
					e.mu.Unlock()
					e.finishCurJobRoot(jobPath)
					e.poke()
					continue
				}
				e.mu.Lock()
				e.cache[jobPath] = fi.Size()
				delete(e.activeWalkRoots, jobPath)
				e.mu.Unlock()
				e.finishCurJobRoot(jobPath)
				e.signalSubtreeIndexed(sess, jobPath, sourcePanel)
				e.poke()
				continue
			}

			tree := WalkFolder(jobPath, nil, shouldIgnore, nil, e.walkConcurrency)

			if e.gen.Load() != sess {
				e.mu.Lock()
				delete(e.activeWalkRoots, jobPath)
				e.mu.Unlock()
				e.finishCurJobRoot(jobPath)
				e.poke()
				return
			}

			merged := map[string]int64{}
			FlattenSizes(tree, merged)

			keys := make([]string, 0, len(merged))
			for k := range merged {
				keys = append(keys, k)
			}
			for i := 0; i < len(keys); i += cacheMergeChunkSize {
				if e.gen.Load() != sess {
					e.mu.Lock()
					delete(e.activeWalkRoots, jobPath)
					e.mu.Unlock()
					e.finishCurJobRoot(jobPath)
					e.poke()
					return
				}
				end := min(i+cacheMergeChunkSize, len(keys))
				e.mu.Lock()
				for _, k := range keys[i:end] {
					e.cache[k] = merged[k]
				}
				e.mu.Unlock()
			}
			if e.gen.Load() != sess {
				e.mu.Lock()
				delete(e.activeWalkRoots, jobPath)
				e.mu.Unlock()
				e.finishCurJobRoot(jobPath)
				e.poke()
				return
			}
			e.mu.Lock()
			delete(e.activeWalkRoots, jobPath)
			e.mu.Unlock()

			e.finishCurJobRoot(jobPath)
			e.signalSubtreeIndexed(sess, jobPath, sourcePanel)
			e.poke()
		}
	}
	if e.gen.Load() == sess {
		e.signalJobFinished(sess)
	}
}
