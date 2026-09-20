package panel

import (
	"context"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/fsbackend/file"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ListingRefreshSnapshot captures panel listing options for an off-thread directory read.
type ListingRefreshSnapshot struct {
	Loc                     pathloc.Path
	ShowHidden              bool
	Gitignore               *gitignore.Cache
	ListTimeout             time.Duration
	ClimbToExistingAncestor bool
}

// ListingRefreshSnapshot builds a snapshot for loc using the panel's current visibility options.
func (s *State) ListingRefreshSnapshot(loc pathloc.Path, listTimeout time.Duration) ListingRefreshSnapshot {
	return ListingRefreshSnapshot{
		Loc:                     loc,
		ShowHidden:              s.ShowHidden,
		Gitignore:               s.Gitignore,
		ListTimeout:             listTimeout,
		ClimbToExistingAncestor: s.climbToExistingAncestor,
	}
}

// FetchListing reads a directory listing using snap (safe to call from a worker goroutine).
// When ClimbToExistingAncestor is set and snap.Loc is missing, it retries each parent until a
// listing succeeds. The requested loc is listed first so a still-existing cwd pays no extra Stat.
func FetchListing(ctx context.Context, snap ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
	if snap.ListTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, snap.ListTimeout)
		defer cancel()
	}
	loc := snap.Loc
	for {
		entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err := fetchListingAt(ctx, snap, loc)
		if err == nil || !snap.ClimbToExistingAncestor {
			return entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err
		}
		parent := loc.Parent()
		if parent.Equal(loc) {
			return entries, listingLoc, gitignoreActive, dotfilesHiddenActive, err
		}
		loc = parent
	}
}

func fetchListingAt(ctx context.Context, snap ListingRefreshSnapshot, loc pathloc.Path) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
	if loc.IsRemote() {
		be, berr := fsbackend.Default().Backend(loc)
		if berr != nil {
			return nil, pathloc.Path{}, false, false, berr
		}
		entries, err := be.List(ctx, loc)
		if err != nil {
			return nil, pathloc.Path{}, false, false, err
		}
		dotfilesHiddenActive := !snap.ShowHidden && fsbackend.HasDotfileNames(entries)
		return fsbackend.FilterHidden(entries, snap.ShowHidden), loc, false, dotfilesHiddenActive, nil
	}
	host, ferr := loc.FilePath()
	if ferr != nil {
		return nil, pathloc.Path{}, false, false, ferr
	}
	gitMatcher, gerr := localfs.MatcherForListing(snap.ShowHidden, snap.Gitignore, host)
	if gerr != nil {
		return nil, pathloc.Path{}, false, false, gerr
	}
	be := file.New()
	entries, err := be.ListWithOptions(ctx, loc, localfs.ListOptions{
		ShowHidden: snap.ShowHidden,
		Gitignore:  gitMatcher,
	})
	if err != nil {
		return nil, pathloc.Path{}, false, false, err
	}
	dotfilesHiddenActive := false
	if !snap.ShowHidden {
		dotfilesHiddenActive, err = localfs.DirHasDotfileNames(host)
		if err != nil {
			return nil, pathloc.Path{}, false, false, err
		}
	}
	listingLoc, err := pathloc.File(host)
	if err != nil {
		return nil, pathloc.Path{}, false, false, err
	}
	return entries, listingLoc, gitMatcher != nil, dotfilesHiddenActive, nil
}

// firstMissingChildName is the first path component of from that is a direct child of ancestor
// (the vanished directory sitting immediately under the existing ancestor).
func firstMissingChildName(ancestor, from pathloc.Path) string {
	current := from
	for {
		parent := current.Parent()
		if parent.Equal(ancestor) {
			return current.Base()
		}
		if parent.Equal(current) || parent.IsZero() {
			return from.Base()
		}
		current = parent
	}
}

// BackendEntriesFromPanel converts current panel rows to backend entries for listing comparison.
func BackendEntriesFromPanel(entries []localfs.Entry) []fsbackend.Entry {
	out := make([]fsbackend.Entry, len(entries))
	for i, e := range entries {
		out[i] = fsbackend.FromPanelEntry(e)
	}
	return out
}
