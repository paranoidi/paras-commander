package scan

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

const (
	batchEntryThreshold = 256
	batchInterval       = 50 * time.Millisecond
)

// WalkOptions configures a directory walk rooted at one path.
type WalkOptions struct {
	Root          string
	IncludeHidden bool
	Gitignore     *gitignore.Cache
	ShouldSkipDir func(string) bool
}

// RootWalk runs one recursive directory index (tests and legacy callers).
type RootWalk = rootWalk

// StartRootWalk begins indexing root. Call Close when finished.
func StartRootWalk(ctx context.Context, root string, opts WalkOptions, walk fswalk.Params) *RootWalk {
	return startRootWalk(ctx, root, opts, walk)
}

type rootWalk struct {
	root    string
	walk    fswalk.Params
	results chan []Entry
	done    chan struct{}
	err     error

	cancel context.CancelFunc
	wg     sync.WaitGroup

	adapt *fswalk.Adaptive

	skippedMu    sync.Mutex
	skippedDirs  []string
	skippedFiles []Entry
}

func startRootWalk(ctx context.Context, root string, opts WalkOptions, walk fswalk.Params) *rootWalk {
	root = filepath.Clean(root)
	ctx, cancel := context.WithCancel(ctx)
	w := &rootWalk{
		root:    root,
		walk:    walk,
		results: make(chan []Entry, 8),
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	w.wg.Add(1)
	go w.run(ctx, opts)
	return w
}

func (w *rootWalk) Results() <-chan []Entry { return w.results }

func (w *rootWalk) Done() <-chan struct{} { return w.done }

func (w *rootWalk) Err() error { return w.err }

func (w *rootWalk) Workers() int {
	if w.adapt == nil {
		return 0
	}
	return w.adapt.Workers()
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

func (w *rootWalk) SkippedHiddenFiles() []Entry {
	w.skippedMu.Lock()
	defer w.skippedMu.Unlock()
	if len(w.skippedFiles) == 0 {
		return nil
	}
	out := make([]Entry, len(w.skippedFiles))
	copy(out, w.skippedFiles)
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
	pending   []Entry
	pendingMu sync.Mutex
	flush     func()
}

func (b *walkBatch) appendEntry(entry Entry, adapt *fswalk.Adaptive) bool {
	adapt.Bump()
	b.pendingMu.Lock()
	b.pending = append(b.pending, entry)
	shouldFlush := len(b.pending) >= batchEntryThreshold
	b.pendingMu.Unlock()
	return shouldFlush
}

func (w *rootWalk) run(ctx context.Context, opts WalkOptions) {
	defer w.wg.Done()
	defer close(w.done)

	adapt := fswalk.NewAdaptive(ctx, w.walk)
	defer adapt.Stop()
	w.adapt = adapt
	sem := adapt.Sem()

	batch := &walkBatch{}
	batch.flush = func() {
		batch.pendingMu.Lock()
		if len(batch.pending) == 0 {
			batch.pendingMu.Unlock()
			return
		}
		out := append([]Entry(nil), batch.pending...)
		batch.pending = batch.pending[:0]
		batch.pendingMu.Unlock()
		select {
		case w.results <- out:
		case <-ctx.Done():
		}
	}

	ticker := time.NewTicker(batchInterval)
	defer ticker.Stop()

	tickDone := make(chan struct{})
	go func() {
		defer close(tickDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				batch.flush()
			}
		}
	}()

	listOpts := localfs.ListOptions{ShowHidden: opts.IncludeHidden}
	if !opts.IncludeHidden {
		matcher, matcherErr := localfs.MatcherForListing(false, opts.Gitignore, w.root)
		if matcherErr != nil {
			w.err = matcherErr
			return
		}
		listOpts.Gitignore = matcher
	}

	var walkWg sync.WaitGroup
	walkWg.Add(1)
	go func() {
		defer walkWg.Done()
		w.walkDirectory(ctx, w.root, opts, listOpts, adapt, sem, &walkWg, batch)
	}()
	walkWg.Wait()

	batch.flush()
	w.cancel()
	<-tickDone
	close(w.results)
	if ctx.Err() != nil && w.err == nil {
		w.err = ctx.Err()
	}
}

func (w *rootWalk) walkDirectory(
	ctx context.Context,
	dir string,
	opts WalkOptions,
	listOpts localfs.ListOptions,
	adapt *fswalk.Adaptive,
	sem *fswalk.DynSem,
	wg *sync.WaitGroup,
	batch *walkBatch,
) {
	if ctx.Err() != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range entries {
		if ctx.Err() != nil {
			return
		}
		path := filepath.Join(dir, d.Name())
		if path == w.root {
			continue
		}

		name := d.Name()
		isDir := d.IsDir()

		if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
			w.recordSkippedHidden(path, d, isDir)
			continue
		}

		if !localfs.EntryVisible(name, filepath.Dir(path), isDir, listOpts) {
			continue
		}

		entry, ok := buildEntry(w.root, path, d, isDir)
		if !ok {
			continue
		}
		if batch.appendEntry(entry, adapt) {
			batch.flush()
		}

		if !entry.IsDir {
			continue
		}
		if opts.ShouldSkipDir != nil && opts.ShouldSkipDir(path) {
			continue
		}

		wg.Add(1)
		go func(subPath string) {
			defer wg.Done()
			if err := sem.Acquire(ctx); err != nil {
				return
			}
			defer sem.Release()
			w.walkDirectory(ctx, subPath, opts, listOpts, adapt, sem, wg, batch)
		}(path)
	}
}

func (w *rootWalk) recordSkippedHidden(path string, d fs.DirEntry, isDir bool) {
	clean := filepath.Clean(path)
	if isDir {
		w.skippedMu.Lock()
		w.skippedDirs = append(w.skippedDirs, clean)
		w.skippedMu.Unlock()
		return
	}
	entry, ok := buildEntry(w.root, path, d, false)
	if !ok {
		return
	}
	w.skippedMu.Lock()
	w.skippedFiles = append(w.skippedFiles, entry)
	w.skippedMu.Unlock()
}
