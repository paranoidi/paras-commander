package panel

import (
	"sort"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// AsyncLoadRequest describes a directory listing to run off the UI thread. Not remote-specific:
// a local path can be just as slow as a remote one (network mount, autofs trigger), and Go
// cannot cancel a goroutine blocked inside a real blocking syscall — running it off-thread keeps
// the UI responsive even when the underlying fetch never returns.
type AsyncLoadRequest struct {
	Loc           pathloc.Path
	SelectedName  string
	ViewportRows  int
	IndexFallback int
	// CenterRecalledCursor scrolls to center the restored highlight when re-entering a directory.
	CenterRecalledCursor bool
	// Rollback runs on the main thread when listing fails (e.g. revert history index).
	Rollback func()
	// OnApplied runs on the main thread once, after ApplyListing succeeds for this request.
	OnApplied func()
	// SyncHistoryHead, when true, sets History[0] to Path after a successful load (NavigateToPath).
	SyncHistoryHead bool
	// ListingEpoch is State.ListingEpoch at schedule time. Same-directory applies whose epoch no
	// longer matches are dropped (a rename/mkdir insert bumped the panel's epoch meanwhile).
	ListingEpoch uint64
	// TreePrefetch lists remembered-expanded directories under Loc (all ancestors up to Loc
	// expanded too) whose children the loader should fetch together with the root listing, so
	// the re-entered tree lands in one apply. Empty outside tree layout.
	TreePrefetch []string
	// HistoryVisit/HistoryPath identify the recordVisit performed when this navigation was
	// scheduled. A stale apply (superseded generation) undoes that visit so A→B→C cannot leave
	// B in the timeline when B never landed.
	HistoryVisit uint64
	HistoryPath  string
}

// TreePrefetchResult is one prefetched directory's children (or fetch error), keyed by directory
// id in the map handed to ApplyListingPrefetched.
type TreePrefetchResult struct {
	Entries []localfs.Entry
	Err     error
}

// treePrefetchIDs returns the expanded directories under loc that ApplyListingPrefetched will
// re-expand (the same expanded set it picks: live TreeExpanded on a same-directory reload,
// otherwise the recall snapshot), restricted to those whose ancestors up to loc are all expanded.
func (s *State) treePrefetchIDs(loc pathloc.Path) []string {
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	var keep map[string]bool
	if loc.Equal(s.Path) {
		keep = s.TreeExpanded
	} else if snap, ok := s.HistoryCursorByPath[cleanPathString(loc.String())]; ok {
		keep = snap.TreeExpanded
	}
	var ids []string
	for id, on := range keep {
		if !on {
			continue
		}
		p, err := pathloc.Parse(id)
		if err != nil || !p.HasPrefix(loc) || p.Equal(loc) {
			continue
		}
		ok := true
		for par := p.Parent(); !par.Equal(loc); par = par.Parent() {
			if !keep[par.String()] || par.Equal(par.Parent()) {
				ok = false
				break
			}
		}
		if ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// AsyncLoadScheduler starts an off-thread listing for req.
// Results are applied via ApplyListing on the main event thread. Return false to list synchronously.
type AsyncLoadScheduler func(req AsyncLoadRequest) bool

// asyncLoadOpts carries per-load rollback/history behavior.
type asyncLoadOpts struct {
	rollback        func()
	onApplied       func()
	syncHistoryHead bool
	historyVisit    uint64
	historyPath     string
}

// RevertHistoryVisit removes the visit recorded under token if it is still the latest visit
// for target. Used when an async navigation is superseded before it applies.
func (s *State) RevertHistoryVisit(target string, token uint64) {
	s.revertHistoryVisit(target, token)
}

func (s *State) revertHistoryVisit(target string, token uint64) {
	target = cleanPathString(target)
	if token == 0 || target == "" {
		return
	}
	if s.historyVisitToken[target] != token {
		return
	}
	delete(s.historyVisitToken, target)
	s.History = removePathFromSlice(s.History, target)
	if s.HistoryIndex >= len(s.History) {
		if len(s.History) == 0 {
			s.HistoryIndex = 0
		} else {
			s.HistoryIndex = len(s.History) - 1
		}
	}
}
