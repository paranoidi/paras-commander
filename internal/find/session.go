package find

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

const (
	batchEntryThreshold = 256
	batchInterval       = 50 * time.Millisecond
)

// Entry is one indexed file or directory under the search root.
type Entry struct {
	Path    string // absolute, clean
	RelLine string // path relative to root (display / fuzzy match)
	IsDir   bool
	Type    localfs.EntryType
	Size    int64 // file byte size from walk; 0 for directories
}

// Options configures a find session walk.
type Options struct {
	ShowHidden    bool
	Gitignore     *gitignore.Cache             // nil disables .gitignore filtering
	ShouldSkipDir diskusage.ShouldIgnoreFolder // skip descending into matching dirs (volume gate, etc.)
}

// Session runs a recursive index in the background.
type Session struct {
	root    string
	results chan []Entry
	done    chan struct{}
	err     error

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start begins indexing root. The caller must call Close when finished.
func Start(ctx context.Context, root string, opts Options) *Session {
	root = filepath.Clean(root)
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{
		root:    root,
		results: make(chan []Entry, 8),
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	s.wg.Add(1)
	go s.run(ctx, opts)
	return s
}

// Results receives incremental batches until the walk completes or is cancelled.
func (s *Session) Results() <-chan []Entry { return s.results }

// Done is closed when the walk goroutine exits.
func (s *Session) Done() <-chan struct{} { return s.done }

// Err returns the walk error, if any, after Done is closed.
func (s *Session) Err() error { return s.err }

// Close cancels the walk and waits for the goroutine.
func (s *Session) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Session) run(ctx context.Context, opts Options) {
	defer s.wg.Done()
	defer close(s.done)

	var pending []Entry
	var pendingMu sync.Mutex
	flush := func() {
		pendingMu.Lock()
		if len(pending) == 0 {
			pendingMu.Unlock()
			return
		}
		batch := append([]Entry(nil), pending...)
		pending = pending[:0]
		pendingMu.Unlock()
		select {
		case s.results <- batch:
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
				flush()
			}
		}
	}()

	listOpts := localfs.ListOptions{ShowHidden: opts.ShowHidden}
	if !opts.ShowHidden {
		matcher, matcherErr := localfs.MatcherForListing(false, opts.Gitignore, s.root)
		if matcherErr != nil {
			s.err = matcherErr
			return
		}
		listOpts.Gitignore = matcher
	}

	walkErr := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if path == s.root {
			return nil
		}

		name := d.Name()
		isDir := d.IsDir()
		if !localfs.EntryVisible(name, filepath.Dir(path), isDir, listOpts) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(s.root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		entryType := localfs.EntryFile
		if d.Type()&fs.ModeSymlink != 0 {
			entryType = localfs.EntrySymlink
			if info, statErr := os.Stat(path); statErr == nil {
				isDir = info.IsDir()
			}
		}
		var size int64
		if isDir {
			entryType = localfs.EntryDirectory
		} else if entryType != localfs.EntrySymlink {
			entryType = localfs.EntryFile
			if info, infoErr := d.Info(); infoErr == nil {
				size = info.Size()
			}
		}

		entry := Entry{
			Path:    filepath.Clean(path),
			RelLine: rel,
			IsDir:   isDir,
			Type:    entryType,
			Size:    size,
		}
		shouldFlush := false
		pendingMu.Lock()
		pending = append(pending, entry)
		if len(pending) >= batchEntryThreshold {
			shouldFlush = true
		}
		pendingMu.Unlock()
		if shouldFlush {
			flush()
		}

		if isDir && opts.ShouldSkipDir != nil && opts.ShouldSkipDir(path) {
			return filepath.SkipDir
		}
		return nil
	})

	flush()
	s.cancel()
	<-tickDone
	close(s.results)
	if walkErr != nil && ctx.Err() == nil {
		s.err = walkErr
	}
}
