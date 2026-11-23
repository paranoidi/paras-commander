package panel

import (
	"maps"
	"slices"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/selstrip"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// Duplicate returns an independent copy of s for a new panel tab: the two share no map, slice or
// pointer (except the Gitignore cache) and diverge from here on. Everything the tab shows carries
// over (path, cursor, scroll, history, sort, entry filter, tree, carousel, columns); the live
// selection, strip, pending range, quick filter, stash and every in-flight load do not. Callers
// duplicate a state that is not mid-load (the app settles it first); the pending flags are cleared
// regardless, so the copy never waits for a result that is delivered to its source.
//
// Closures on State (schedulers, MetaValue, ...) are shared by value: they resolve the panel by
// ID, not by State pointer. The generations are copied unchanged; the app raises them past the
// tab it replaces (BumpGenerationsPast).
func (s *State) Duplicate() State {
	d := *s

	d.RangeAnchor, d.RangeUnselect = "", false
	d.Entries = slices.Clone(s.Entries)
	d.History = slices.Clone(s.History)
	// Tokens only matter to a navigation still in flight, and the copy has none.
	d.historyVisitToken = nil
	d.HistoryCursorByPath = nil
	if s.HistoryCursorByPath != nil {
		d.HistoryCursorByPath = make(map[string]historyCursorSnapshot, len(s.HistoryCursorByPath))
		for dir, snap := range s.HistoryCursorByPath {
			snap.TreeExpanded = maps.Clone(snap.TreeExpanded)
			d.HistoryCursorByPath[dir] = snap
		}
	}

	d.clearSelectionState()
	d.StripForced = false
	// Strip.Tree is the session-wide strip tree mode, not selection state.
	d.Strip = selstrip.State{Tree: s.Strip.Tree}
	d.selDerivedCache = selectionDerivedCache{}
	d.SelectionStashPaths, d.SelectionStashStripCursorPath = nil, ""
	d.Filter = quickfilter.Filter{}
	d.invalidateSelectionDerived()

	d.matchNames = &matchNamesCache{}
	d.GitByPath = maps.Clone(s.GitByPath)
	if s.ActiveEntryFilter != nil {
		f := *s.ActiveEntryFilter
		d.ActiveEntryFilter = &f
	}
	d.CarouselChildPreviewCoalesce = false
	d.CursorNameHintCoalesce = false
	d.CarouselSideCache.Parent.Entries = slices.Clone(s.CarouselSideCache.Parent.Entries)
	d.CarouselSideCache.Child.Entries = slices.Clone(s.CarouselSideCache.Child.Entries)

	d.NewFileMarksByDir = maps.Clone(s.NewFileMarksByDir)
	for dir, m := range d.NewFileMarksByDir {
		if m != nil {
			d.NewFileMarksByDir[dir] = &dirNewFileMarks{latest: maps.Clone(m.latest), previous: maps.Clone(m.previous), older: maps.Clone(m.older)}
		}
	}
	d.RenameMarksByDir = maps.Clone(s.RenameMarksByDir)
	for dir, names := range d.RenameMarksByDir {
		d.RenameMarksByDir[dir] = maps.Clone(names)
	}

	d.clearPending()
	d.indexListed = nil
	d.ancestorClimbOrigin = pathloc.Path{}

	d.TreeRoots = cloneTreeNodes(s.TreeRoots)
	d.TreeExpanded = maps.Clone(s.TreeExpanded)
	d.treeByPath = maps.Clone(s.treeByPath)
	d.treePriorChildren, d.treePrefetch = nil, nil
	d.treeCursorFallback = slices.Clone(s.treeCursorFallback)
	d.treeExpandQuiet = 0

	d.listingByPath, d.filteredIdx, d.filteredTreeShape, d.treeRows = nil, nil, nil, nil
	d.rebuildListingByPath()
	if d.ListLayout == ListLayoutTree {
		d.rebuildTreeRows()
	} else {
		d.rebuildFilter()
	}
	return d
}

// Park drops everything the state is waiting for, so a tab that stores it never expects a result
// the app has just superseded: the pending listing and git fetches, the drift passes and the tree
// child loads (the expanded folders stay marked and are re-read when the tab is entered).
// Refresh returns early for a pending load to another directory, so a stale flag would block the
// reload that follows.
func (s *State) Park() {
	s.clearPending()
	s.abandonTreeChildLoads(false)
}

// clearPending resets every flag that marks a result as awaited.
func (s *State) clearPending() {
	s.ListingPending, s.ListingPendingPath, s.ShowLoadingIcon = false, "", false
	s.GitPending, s.gitStatusChildPending = false, 0
	s.indexDriftPending, s.SkipIndexDrift = nil, false
	s.climbToExistingAncestor = false
	s.treeExpandAllAuto = false
}

// AdoptPanelSettings takes over from another tab's state what belongs to the panel, not to the
// tab: the scroll settings, the disk usage idle sort, the strip tree mode and the selection stash.
// Hidden-file visibility is panel-wide too, but changing it needs a reload (App.swapTab).
func (s *State) AdoptPanelSettings(from *State) {
	s.ScrollMode, s.ScrollEdgeMargin = from.ScrollMode, from.ScrollEdgeMargin
	s.DiskUsageIdleSortEligible = from.DiskUsageIdleSortEligible
	s.Strip.Tree = from.Strip.Tree
	s.SelectionStashPaths = from.SelectionStashPaths
	s.SelectionStashStripCursorPath = from.SelectionStashStripCursorPath
}

// cloneTreeNodes deep-copies a tree (nodes are mutated in place) with no node marked Loading: a
// copy never receives its source's in-flight child fetches. A nil Children (not loaded) stays nil.
func cloneTreeNodes(nodes []treeflat.Node[TreeEntry]) []treeflat.Node[TreeEntry] {
	if nodes == nil {
		return nil
	}
	out := make([]treeflat.Node[TreeEntry], len(nodes))
	for i, n := range nodes {
		n.Value.Loading = false
		n.Children = cloneTreeNodes(n.Children)
		out[i] = n
	}
	return out
}

// BumpGenerationsPast raises ListingEpoch and the selection generation above both s's and other's,
// so every result or cache guarded by either is dropped (two tabs may hold the same values, e.g. a
// duplicate and its source).
func (s *State) BumpGenerationsPast(other *State) {
	s.ListingEpoch = max(s.ListingEpoch, other.ListingEpoch) + 1
	s.selectionDerivedGen = max(s.selectionDerivedGen, other.selectionDerivedGen)
	s.invalidateSelectionDerived()
}
