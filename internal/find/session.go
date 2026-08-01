package find

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/fswalk"
	"github.com/paranoidi/paras-commander/internal/scan"
)

// Session runs a recursive index in the background.
type Session struct {
	w *scan.RootWalk
}

// Start begins indexing root. The caller must call Close when finished.
func Start(ctx context.Context, root string, opts Options, walk fswalk.Params) *Session {
	wopts := scan.WalkOptions{
		Root:          root,
		IncludeHidden: opts.IncludeHidden,
		Gitignore:     opts.Gitignore,
		ShouldSkipDir: opts.ShouldSkipDir,
	}
	return &Session{w: scan.StartRootWalk(ctx, root, wopts, walk)}
}

// Results receives incremental batches until the walk completes or is cancelled.
func (s *Session) Results() <-chan []Entry { return s.w.Results() }

// Done is closed when the walk goroutine exits.
func (s *Session) Done() <-chan struct{} { return s.w.Done() }

// Err returns the walk error, if any, after Done is closed.
func (s *Session) Err() error { return s.w.Err() }

// Workers returns the current adaptive walk concurrency limit.
func (s *Session) Workers() int { return s.w.Workers() }

// SkippedHiddenDirs returns absolute paths of dot-directories skipped when IncludeHidden was false.
func (s *Session) SkippedHiddenDirs() []string { return s.w.SkippedHiddenDirs() }

// SkippedHiddenFiles returns absolute paths of dot-files skipped when IncludeHidden was false.
func (s *Session) SkippedHiddenFiles() []string { return s.w.SkippedHiddenFiles() }

// Close cancels the walk and waits for the goroutine.
func (s *Session) Close() { s.w.Close() }
