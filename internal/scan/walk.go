package scan

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

const batchEntryThreshold = 1024

// WalkOptions configures a directory walk rooted at one path.
type WalkOptions struct {
	Root          string
	IncludeHidden bool
	Gitignore     *gitignore.Cache
	ShouldSkipDir func(string) bool
	// DirsOnly indexes directories only (jump mode); files are neither
	// indexed nor recorded as skipped.
	DirsOnly bool
	// MaxDepth limits the levels read below Root (1 = Root's children only); 0 = unlimited.
	MaxDepth int
	// Walk tunes the adaptive directory-read concurrency within a level; zero runs sequentially.
	Walk fswalk.Params
}

type rootWalk struct {
	root    string
	results chan []Entry
	done    chan struct{}
	err     error

	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Level progress of runBFS: depth of the entries being read (root's
	// children are depth 1), directories read so far and total at that depth.
	depth, levelDone, levelTotal atomic.Int64

	// maxDepth is the live level limit (0 = unlimited); SetMaxDepth lowers it mid-walk.
	maxDepth atomic.Int64

	skippedMu        sync.Mutex
	skippedDirs      []string
	skippedFilePaths []string // RelLine under root for dot-files
}

func startRootWalk(ctx context.Context, root string, opts WalkOptions) *rootWalk {
	root = filepath.Clean(root)
	ctx, cancel := context.WithCancel(ctx)
	w := &rootWalk{
		root:    root,
		results: make(chan []Entry, 16),
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	w.maxDepth.Store(int64(opts.MaxDepth))
	w.wg.Add(1)
	go w.run(ctx, opts)
	return w
}

func (w *rootWalk) Results() <-chan []Entry { return w.results }

func (w *rootWalk) Done() <-chan struct{} { return w.done }

func (w *rootWalk) Err() error { return w.err }

// Progress reports the BFS level being read: its depth and how many of its
// directories are read. After a finished walk it is the last level read.
func (w *rootWalk) Progress() (depth, done, total int) {
	return int(w.depth.Load()), int(w.levelDone.Load()), int(w.levelTotal.Load())
}

// SetMaxDepth changes the level limit of a running walk (0 = unlimited).
func (w *rootWalk) SetMaxDepth(n int) { w.maxDepth.Store(int64(n)) }

// tooDeep reports whether level depth exceeds the current limit.
func (w *rootWalk) tooDeep(depth int64) bool {
	m := w.maxDepth.Load()
	return m > 0 && depth > m
}

func (w *rootWalk) SkippedHiddenDirs() []string {
	w.skippedMu.Lock()
	defer w.skippedMu.Unlock()
	if len(w.skippedDirs) == 0 {
		return nil
	}
	out := make([]string, len(w.skippedDirs))
	copy(out, w.skippedDirs)
	return out
}

func (w *rootWalk) SkippedHiddenFiles() []string {
	w.skippedMu.Lock()
	defer w.skippedMu.Unlock()
	if len(w.skippedFilePaths) == 0 {
		return nil
	}
	out := make([]string, len(w.skippedFilePaths))
	copy(out, w.skippedFilePaths)
	return out
}

func (w *rootWalk) Close() {
	w.cancel()
	w.wg.Wait()
}

// Abort cancels the walk without waiting for goroutines to exit.
func (w *rootWalk) Abort() {
	w.cancel()
}

type walkBatch struct {
	pending []Entry
	flush   func()
}

func (b *walkBatch) appendEntry(entry Entry) bool {
	b.pending = append(b.pending, entry)
	return len(b.pending) >= batchEntryThreshold
}

func (w *rootWalk) run(ctx context.Context, opts WalkOptions) {
	defer w.wg.Done()
	defer close(w.done)

	batch := &walkBatch{}
	batch.flush = func() {
		if len(batch.pending) == 0 {
			return
		}
		out := append([]Entry(nil), batch.pending...)
		batch.pending = batch.pending[:0]
		select {
		case w.results <- out:
		case <-ctx.Done():
		}
	}

	listOpts := localfs.ListOptions{ShowHidden: opts.IncludeHidden}
	if !opts.IncludeHidden {
		matcher, matcherErr := localfs.MatcherForListing(false, opts.Gitignore, w.root)
		if matcherErr != nil {
			w.err = matcherErr
			return
		}
		listOpts.Gitignore = matcher
	}

	adapt := fswalk.NewAdaptive(ctx, opts.Walk)
	defer adapt.Stop()
	w.runBFS(ctx, opts, listOpts, batch, adapt)
	batch.flush()
	close(w.results)
}

// runBFS indexes level by level (all depth-1 entries, then depth-2, ...) so
// shallow results appear first. Hidden and gitignored entries are skipped and
// recorded for the Include hidden replay; ShouldSkipDir dirs are indexed but not
// descended; symlinks are not followed. Directory reads of a level run
// concurrently under adapt; results are consumed in level order.
func (w *rootWalk) runBFS(ctx context.Context, opts WalkOptions, listOpts localfs.ListOptions, batch *walkBatch, adapt *fswalk.Adaptive) {
	level := []string{w.root}
	for len(level) > 0 {
		var next []string
		if w.tooDeep(w.depth.Load() + 1) {
			return
		}
		w.depth.Add(1)
		w.levelDone.Store(0)
		w.levelTotal.Store(int64(len(level)))
		slots, stop := w.readLevel(ctx, level, adapt)
		for i, dir := range level {
			if ctx.Err() != nil || w.tooDeep(w.depth.Load()) {
				stop()
				return
			}
			select {
			case <-slots[i].done:
			case <-ctx.Done():
				stop()
				return
			}
			children, err := slots[i].children, slots[i].err
			slots[i].children = nil
			w.levelDone.Add(1)
			if err != nil {
				continue // unreadable dir: skip
			}
			for _, d := range children {
				isDir := d.IsDir()
				if opts.DirsOnly && !isDir {
					continue
				}
				name := d.Name()
				path := filepath.Join(dir, name)
				if !localfs.EntryVisible(name, dir, isDir, listOpts) {
					w.recordSkippedHidden(path, isDir)
					continue
				}
				entry, ok := buildEntry(w.root, path, d, isDir)
				if !ok {
					continue
				}
				if batch.appendEntry(entry) {
					batch.flush()
				}
				if isDir && (opts.ShouldSkipDir == nil || !opts.ShouldSkipDir(path)) {
					next = append(next, path)
				}
			}
		}
		stop()
		batch.flush() // show each level as soon as it is indexed
		level = next
	}
}

type dirRead struct {
	children []os.DirEntry
	err      error
	done     chan struct{}
}

// readLevel reads dirs concurrently, bounded by the adaptive semaphore. A slot's
// done channel closes once its dir is read. stop cancels the remaining reads and
// waits for every goroutine to exit.
func (w *rootWalk) readLevel(ctx context.Context, dirs []string, adapt *fswalk.Adaptive) ([]dirRead, func()) {
	slots := make([]dirRead, len(dirs))
	for i := range slots {
		slots[i].done = make(chan struct{})
	}
	lctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sem := adapt.Sem()
		for i, dir := range dirs {
			if sem.Acquire(lctx) != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer sem.Release()
				slots[i].children, slots[i].err = os.ReadDir(dir)
				adapt.Bump()
				close(slots[i].done)
			}()
		}
	}()
	return slots, func() { cancel(); wg.Wait() }
}

func (w *rootWalk) recordSkippedHidden(path string, isDir bool) {
	if isDir {
		clean := filepath.Clean(path)
		w.skippedMu.Lock()
		w.skippedDirs = append(w.skippedDirs, clean)
		w.skippedMu.Unlock()
		return
	}
	rel := relLine(w.root, path)
	w.skippedMu.Lock()
	w.skippedFilePaths = append(w.skippedFilePaths, rel)
	w.skippedMu.Unlock()
}
