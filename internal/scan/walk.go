package scan

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

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
}

// RootWalk runs one recursive directory index (tests and legacy callers).
type RootWalk = rootWalk

// StartRootWalk begins indexing root. Call Close when finished.
func StartRootWalk(ctx context.Context, root string, opts WalkOptions, _ fswalk.Params) *RootWalk {
	return startRootWalk(ctx, root, opts)
}

type rootWalk struct {
	root    string
	results chan []Entry
	done    chan struct{}
	err     error

	cancel context.CancelFunc
	wg     sync.WaitGroup

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
	w.wg.Add(1)
	go w.run(ctx, opts)
	return w
}

func (w *rootWalk) Results() <-chan []Entry { return w.results }

func (w *rootWalk) Done() <-chan struct{} { return w.done }

func (w *rootWalk) Err() error { return w.err }

// Workers reports 1 while a sequential walk is running.
func (w *rootWalk) Workers() int { return 1 }

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

	walkErr := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if path == w.root {
			return nil
		}

		name := d.Name()
		isDir := d.IsDir()

		if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
			w.recordSkippedHidden(path, isDir)
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		if !localfs.EntryVisible(name, filepath.Dir(path), isDir, listOpts) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		entry, ok := buildEntry(w.root, path, d, isDir)
		if !ok {
			return nil
		}
		if batch.appendEntry(entry) {
			batch.flush()
		}

		if entry.IsDir && opts.ShouldSkipDir != nil && opts.ShouldSkipDir(path) {
			return filepath.SkipDir
		}
		return nil
	})

	batch.flush()
	close(w.results)
	if walkErr != nil && ctx.Err() == nil {
		w.err = walkErr
	}
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
