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
	Loc         pathloc.Path
	ShowHidden  bool
	Gitignore   *gitignore.Cache
	ListTimeout time.Duration
}

// ListingRefreshSnapshot builds a snapshot for loc using the panel's current visibility options.
func (s *State) ListingRefreshSnapshot(loc pathloc.Path, listTimeout time.Duration) ListingRefreshSnapshot {
	return ListingRefreshSnapshot{
		Loc:         loc,
		ShowHidden:  s.ShowHidden,
		Gitignore:   s.Gitignore,
		ListTimeout: listTimeout,
	}
}

// FetchListing reads a directory listing using snap (safe to call from a worker goroutine).
func FetchListing(ctx context.Context, snap ListingRefreshSnapshot) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
	if snap.ListTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, snap.ListTimeout)
		defer cancel()
	}
	if snap.Loc.IsRemote() {
		be, berr := fsbackend.Default().Backend(snap.Loc)
		if berr != nil {
			return nil, pathloc.Path{}, false, false, berr
		}
		entries, err := be.List(ctx, snap.Loc)
		if err != nil {
			return nil, pathloc.Path{}, false, false, err
		}
		dotfilesHiddenActive := !snap.ShowHidden && fsbackend.HasDotfileNames(entries)
		return fsbackend.FilterHidden(entries, snap.ShowHidden), snap.Loc, false, dotfilesHiddenActive, nil
	}
	host, ferr := snap.Loc.FilePath()
	if ferr != nil {
		return nil, pathloc.Path{}, false, false, ferr
	}
	gitMatcher, gerr := localfs.MatcherForListing(snap.ShowHidden, snap.Gitignore, host)
	if gerr != nil {
		return nil, pathloc.Path{}, false, false, gerr
	}
	be := file.New()
	entries, err := be.ListWithOptions(ctx, snap.Loc, localfs.ListOptions{
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

// BackendEntriesFromPanel converts current panel rows to backend entries for listing comparison.
func BackendEntriesFromPanel(entries []localfs.Entry) []fsbackend.Entry {
	out := make([]fsbackend.Entry, len(entries))
	for i, e := range entries {
		loc, _ := pathloc.Parse(e.Path)
		out[i] = fsbackend.Entry{
			Name:       e.Name,
			Loc:        loc,
			Type:       backendTypeFromLocal(e.Type),
			Size:       e.Size,
			Mode:       e.Mode,
			ModifiedAt: e.ModifiedAt,
		}
	}
	return out
}

func backendTypeFromLocal(t localfs.EntryType) fsbackend.EntryType {
	switch t {
	case localfs.EntryDirectory:
		return fsbackend.EntryDirectory
	case localfs.EntrySymlink:
		return fsbackend.EntrySymlink
	case localfs.EntryOther:
		return fsbackend.EntryOther
	default:
		return fsbackend.EntryFile
	}
}
