package panel

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// RemoteLoadRequest describes a remote directory listing to run off the UI thread.
type RemoteLoadRequest struct {
	Loc           pathloc.Path
	SelectedName  string
	ViewportRows  int
	IndexFallback int
	// Rollback runs on the main thread when listing fails (e.g. revert history index).
	Rollback func()
	// SyncHistoryHead, when true, sets History[0] to Path after a successful load (NavigateToPath).
	SyncHistoryHead bool
}

// RemoteLoadScheduler starts an asynchronous remote listing for req.
// Results are applied via ApplyListing on the main event thread. Return false to list synchronously.
type RemoteLoadScheduler func(req RemoteLoadRequest) bool

// remoteLoadOpts carries per-load rollback/history behavior for remote paths.
type remoteLoadOpts struct {
	rollback        func()
	syncHistoryHead bool
}

// FetchRemoteListing reads a remote directory via fsbackend.
func FetchRemoteListing(ctx context.Context, loc pathloc.Path, showHidden bool) ([]fsbackend.Entry, error) {
	be, err := fsbackend.Default().Backend(loc)
	if err != nil {
		return nil, err
	}
	entries, err := be.List(ctx, loc)
	if err != nil {
		return nil, err
	}
	return fsbackend.FilterHidden(entries, showHidden), nil
}

func (s *State) revertRecordedVisit(attempted string) {
	target := cleanPathString(attempted)
	if target == "" {
		return
	}
	cur := cleanPathString(s.Path.String())
	if len(s.History) == 0 || s.HistoryIndex != 0 {
		return
	}
	if cleanPathString(s.History[0]) != target {
		return
	}
	if cur != "" && cur != target {
		s.History[0] = cur
		return
	}
	if len(s.History) > 1 {
		s.History = s.History[1:]
	}
}
