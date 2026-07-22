package panel

import (
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// TreeEntry is the payload carried by each treeflat.Node in a panel's tree layout.
type TreeEntry struct {
	Entry localfs.Entry
	// Loading is true while an async child fetch is in flight for this node (see
	// setTreeNodeExpanded / ApplyTreeChildLoad in tree_load.go). Drives FolderIconContext.TreeLoading.
	Loading bool
	// LoadErr holds the error from the most recent failed child fetch. Cleared on the next
	// successful fetch. The directory stays collapsed (TreeExpanded never set) while set, so a
	// later expand attempt naturally retries.
	LoadErr error
}

// ListLayout selects how a panel's file list renders.
type ListLayout int

const (
	ListLayoutFlat ListLayout = iota
	ListLayoutTree
)

// maxTreeExpandDepth caps how deep ToggleTreeExpand will load, guarding against runaway
// recursion on pathological directory trees (e.g. symlink cycles) since Phase 1 has no async
// loading or cancellation yet.
const maxTreeExpandDepth = 32

// SetListLayout switches the panel's file-list rendering between flat rows and an
// expand/collapse tree. Entering tree mode seeds TreeRoots from the current (already-loaded)
// flat Entries as depth-0 nodes with no children loaded yet; leaving tree mode is a pure mode
// switch, flat Entries are never mutated by tree mode. Returns false (no-op) when CarouselMode
// is active, matching the existing carousel/listing-format mutual-exclusion guard used
// elsewhere (see ActionPanelListingFormatDialog / ActionPanelCycleListingFormat dispatch).
func (s *State) SetListLayout(layout ListLayout, viewportRows int) bool {
	if layout == ListLayoutTree && s.CarouselMode {
		return false
	}
	if s.ListLayout == layout {
		return true
	}
	s.ListLayout = layout
	if layout == ListLayoutTree {
		s.TreeRoots = treeRootsFromEntries(s.Entries)
		if s.TreeExpanded == nil {
			s.TreeExpanded = make(map[string]bool)
		}
		s.rebuildTreeRows()
		s.clampCursor()
		s.EnsureCursorInViewport(viewportRows)
	}
	return true
}

// resyncTreeOrder reorders TreeRoots' depth-0 nodes to match the current order of s.Entries,
// preserving each node's already-loaded Children/Loading/LoadErr (matched by path) so expand
// state survives. Called from ApplySort so every sort-changing path (sort dialog, cycle sort,
// idle disk-usage sort) keeps the tree view's visible order in sync — without this, treeRows
// (what tree mode actually renders, see VisibleEntry) stays a stale snapshot from whenever tree
// mode was entered or the directory last (re)loaded. No-op outside tree mode.
func (s *State) resyncTreeOrder() {
	if s.ListLayout != ListLayoutTree || len(s.TreeRoots) == 0 {
		return
	}
	byID := make(map[string]treeflat.Node[TreeEntry], len(s.TreeRoots))
	for _, n := range s.TreeRoots {
		byID[n.ID] = n
	}
	roots := make([]treeflat.Node[TreeEntry], 0, len(s.Entries))
	for _, e := range s.Entries {
		if n, ok := byID[e.Path]; ok {
			n.Value.Entry = e
			roots = append(roots, n)
			continue
		}
		roots = append(roots, treeflat.Node[TreeEntry]{ID: e.Path, Value: TreeEntry{Entry: e}})
	}
	s.TreeRoots = roots
	s.resyncTreeChildOrder(s.TreeRoots)
	s.rebuildTreeRows()
}

// resyncTreeChildOrder recursively re-sorts every already-loaded node's Children in place using
// the panel's current sort settings, so directories expanded under an old sort mode/order pick up
// changes the same way resyncTreeOrder already does for the top level. Disk-usage idle-primary
// sort stays forced off here, matching the existing rule for tree-mode children (see
// ApplyTreeChildLoad). Nodes with nil Children (not yet loaded, or leaf files) are left alone.
func (s *State) resyncTreeChildOrder(nodes []treeflat.Node[TreeEntry]) {
	for i := range nodes {
		if nodes[i].Children == nil {
			continue
		}
		entries := make([]localfs.Entry, len(nodes[i].Children))
		for j, c := range nodes[i].Children {
			entries[j] = c.Value.Entry
		}
		SortEntries(entries, s.Sort, s.DiskSorter, false)
		byID := make(map[string]treeflat.Node[TreeEntry], len(nodes[i].Children))
		for _, c := range nodes[i].Children {
			byID[c.ID] = c
		}
		reordered := make([]treeflat.Node[TreeEntry], len(entries))
		for j, e := range entries {
			reordered[j] = byID[e.Path]
		}
		nodes[i].Children = reordered
		s.resyncTreeChildOrder(nodes[i].Children)
	}
}

func treeRootsFromEntries(entries []localfs.Entry) []treeflat.Node[TreeEntry] {
	if len(entries) == 0 {
		return nil
	}
	roots := make([]treeflat.Node[TreeEntry], len(entries))
	for i, e := range entries {
		roots[i] = treeflat.Node[TreeEntry]{ID: e.Path, Value: TreeEntry{Entry: e}}
	}
	return roots
}

// ToggleTreeExpand expands or collapses the directory row under the cursor. No-op outside tree
// mode, out of range, and on file rows. Expanding a directory whose children were never loaded
// reads them synchronously (Phase 1 has no async loading yet) and caches them on the node, so a
// later collapse/expand reuses the cached children instead of re-reading the directory.
func (s *State) ToggleTreeExpand(viewportRows int) error {
	if s.ListLayout != ListLayoutTree || s.Cursor < 0 || s.Cursor >= len(s.treeRows) {
		return nil
	}
	row := s.treeRows[s.Cursor]
	if row.Value.Entry.Type != localfs.EntryDirectory {
		return nil
	}
	id := row.ID
	if err := s.setTreeNodeExpanded(id, row.Depth, !s.TreeExpanded[id]); err != nil {
		return err
	}
	s.treeCursorID = id
	s.rebuildTreeRows()
	s.reattachTreeCursorByID(id, viewportRows)
	return nil
}

// ExpandTreeCursorRow expands the directory row under the cursor. No-op outside tree mode, out
// of range, on file rows, or when the row is already expanded. Unlike ToggleTreeExpand's caller
// (toggleTreeForPanel in internal/app/panels.go), this does not enable tree mode itself — the
// caller is responsible for calling SetListLayout first if auto-enabling is desired.
func (s *State) ExpandTreeCursorRow(viewportRows int) error {
	if s.ListLayout != ListLayoutTree || s.Cursor < 0 || s.Cursor >= len(s.treeRows) {
		return nil
	}
	row := s.treeRows[s.Cursor]
	if row.Value.Entry.Type != localfs.EntryDirectory || s.TreeExpanded[row.ID] {
		return nil
	}
	id := row.ID
	if err := s.setTreeNodeExpanded(id, row.Depth, true); err != nil {
		return err
	}
	s.treeCursorID = id
	s.rebuildTreeRows()
	s.reattachTreeCursorByID(id, viewportRows)
	return nil
}

// CollapseTreeCursorRow collapses the directory row under the cursor. No-op outside tree mode or
// out of range. When the cursor row is itself an expanded directory, that row is collapsed in
// place. Otherwise (a file row, or a nested collapsed directory), it walks backward to the
// cursor row's immediate parent (the nearest preceding row with a strictly smaller depth — see
// treeflat.Flatten's depth-first pre-order guarantee) and, if the cursor isn't already at depth
// 0, collapses that parent and moves the cursor onto it. This mirrors the collapse-or-jump-to-
// parent pattern in dedup.Handler.CollapseOrParent, but also collapses the parent rather than
// only moving the cursor onto it.
func (s *State) CollapseTreeCursorRow(viewportRows int) error {
	if s.ListLayout != ListLayoutTree || s.Cursor < 0 || s.Cursor >= len(s.treeRows) {
		return nil
	}
	row := s.treeRows[s.Cursor]
	if row.Value.Entry.Type == localfs.EntryDirectory && s.TreeExpanded[row.ID] {
		return s.collapseTreeRow(row.ID, row.Depth, viewportRows)
	}
	if row.Depth == 0 {
		return nil
	}
	for i := s.Cursor - 1; i >= 0; i-- {
		if s.treeRows[i].Depth < row.Depth {
			return s.collapseTreeRow(s.treeRows[i].ID, s.treeRows[i].Depth, viewportRows)
		}
	}
	return nil
}

// collapseTreeRow collapses the directory row identified by id and moves the cursor onto it.
// Shared by CollapseTreeCursorRow's in-place and jump-to-parent branches.
func (s *State) collapseTreeRow(id string, depth int, viewportRows int) error {
	if err := s.setTreeNodeExpanded(id, depth, false); err != nil {
		return err
	}
	s.treeCursorID = id
	s.rebuildTreeRows()
	s.reattachTreeCursorByID(id, viewportRows)
	return nil
}

// CollapseAllTree clears all expand state, collapsing the tree back to depth 0. No-op outside
// tree mode. The cursor follows the same nesting logic as CollapseTreeCursorRow: it lands on the
// cursor row's depth-0 ancestor (or the row itself if already at depth 0), not on whatever row
// ends up at the old cursor index.
func (s *State) CollapseAllTree(viewportRows int) {
	if s.ListLayout != ListLayoutTree {
		return
	}
	targetID := s.treeRootAncestorID()
	s.TreeExpanded = nil
	s.rebuildTreeRows()
	if targetID != "" {
		s.reattachTreeCursorByID(targetID, viewportRows)
		return
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// treeRootAncestorID returns the ID of the cursor row's depth-0 ancestor (or the cursor row's own
// ID if it is already at depth 0). Empty if the cursor is out of range.
func (s *State) treeRootAncestorID() string {
	if s.Cursor < 0 || s.Cursor >= len(s.treeRows) {
		return ""
	}
	row := s.treeRows[s.Cursor]
	if row.Depth == 0 {
		return row.ID
	}
	for i := s.Cursor - 1; i >= 0; i-- {
		if s.treeRows[i].Depth == 0 {
			return s.treeRows[i].ID
		}
	}
	return ""
}

// ExpandAllTreeShallow expands every depth-0 directory by exactly one level (loading/revealing
// its immediate children) without recursing into those newly-revealed children. Does not itself
// enable tree mode — same split of responsibility as ExpandTreeCursorRow.
func (s *State) ExpandAllTreeShallow(viewportRows int) error {
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	for i := range s.TreeRoots {
		root := &s.TreeRoots[i]
		if root.Value.Entry.Type != localfs.EntryDirectory {
			continue
		}
		if err := s.setTreeNodeExpanded(root.ID, 0, true); err != nil {
			return err
		}
	}
	s.rebuildTreeRows()
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
	return nil
}

// setTreeNodeExpanded sets the expand flag for id, lazily loading and caching its children the
// first time it is expanded, respecting maxTreeExpandDepth. Shared by ToggleTreeExpand and the
// dedicated expand/collapse methods so the child-loading/dispatch logic lives in one place.
//
// The first expand of an unloaded directory (Children == nil) marks the node Loading and either
// dispatches the fetch via ScheduleTreeChildLoad (async: TreeExpanded[id] is deliberately left
// false here — the caller's immediate rebuildTreeRows()/cursor-reattach only shows the row's
// loading icon; the real expand happens later in ApplyTreeChildLoad on the main thread once the
// fetch completes) or, when no scheduler is wired (nil, or it declines the request — same
// fallback convention as RemoteLoadScheduler), reads the children synchronously inline exactly as
// before. A second expand press while a fetch is already in flight for the same node is a no-op.
//
// ponytail: no placeholder "Loading…"/error child row is synthesized while a fetch is in flight —
// treeflat.Flatten only reveals a node's children once Node.Children is non-nil, so a loading
// directory simply shows no children yet (correct, there aren't any to show); the loading
// affordance lives entirely in the row's own icon (TreeEntry.Loading -> FolderIconScanning, see
// panellist.ResolveFolderIconKind). This also makes retry free on failure: TreeExpanded[id] is
// never set and Children stays nil, so pressing expand again just re-enters this function and
// dispatches a new fetch — no separate retry state machine needed.
func (s *State) setTreeNodeExpanded(id string, depth int, expand bool) error {
	if s.TreeExpanded == nil {
		s.TreeExpanded = make(map[string]bool)
	}
	if expand && depth < maxTreeExpandDepth {
		if node := findTreeNode(s.TreeRoots, id); node != nil && node.Children == nil {
			if node.Value.Loading {
				return nil // already loading; don't dispatch a second concurrent fetch
			}
			node.Value.Loading = true
			if s.ScheduleTreeChildLoad != nil {
				loc, err := pathloc.Parse(id)
				if err != nil {
					node.Value.Loading = false
					return err
				}
				if s.ScheduleTreeChildLoad(TreeChildLoadRequest{DirID: id, Loc: loc}) {
					s.rebuildTreeRows() // show the loading icon now; real expand happens on async apply
					return nil
				}
			}
			children, err := s.loadTreeChildren(id)
			node.Value.Loading = false
			if err != nil {
				node.Value.LoadErr = err
				return err
			}
			node.Value.LoadErr = nil
			node.Children = children
		}
	}
	s.TreeExpanded[id] = expand
	return nil
}

// loadTreeChildren reads dirPath's immediate children via the same listing path Load/Refresh
// use (FetchListing + fsbackend.ToPanelEntries), so hidden-file/gitignore options and remote
// backends behave identically to the flat listing.
func (s *State) loadTreeChildren(dirPath string) ([]treeflat.Node[TreeEntry], error) {
	loc, err := pathloc.Parse(dirPath)
	if err != nil {
		return nil, err
	}
	backendEntries, _, _, _, err := s.fetchBackendEntries(loc)
	if err != nil {
		return nil, err
	}
	entries, err := fsbackend.ToPanelEntries(backendEntries)
	if err != nil {
		return nil, err
	}
	return treeRootsFromEntries(entries), nil
}

func findTreeNode(nodes []treeflat.Node[TreeEntry], id string) *treeflat.Node[TreeEntry] {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
		if found := findTreeNode(nodes[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}

func (s *State) rebuildTreeRows() {
	s.treeRows = treeflat.Flatten(s.TreeRoots, func(id string) bool { return s.TreeExpanded[id] })
	if s.Filter.Active {
		s.rebuildFilter()
	}
}

// reattachTreeCursorByID keeps the cursor on the same node (by ID) after a rebuild when
// possible, else clamps to a valid row.
func (s *State) reattachTreeCursorByID(id string, viewportRows int) {
	for i := range s.treeRows {
		if s.treeRows[i].ID == id {
			s.Cursor = i
			break
		}
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// TreeRowShape returns the tree-shape fields for VisibleEntry(index) when in tree mode: depth,
// last-sibling flag, ancestor continuation guides, whether the row is currently shown expanded
// (per treeflat.Flatten), and whether an async child fetch is in flight for the row (sourced
// directly from TreeEntry.Loading — independent of Flatten's structural HasChildren/Expanded,
// which stay false while Children is still nil during a load). ok is false outside tree mode or
// index range.
func (s State) TreeRowShape(index int) (depth int, lastChild bool, ancestorHasNext []bool, expanded bool, loading bool, ok bool) {
	if s.ListLayout != ListLayoutTree || index < 0 || index >= len(s.treeRows) {
		return 0, false, nil, false, false, false
	}
	row := s.treeRows[index]
	return row.Depth, row.LastChild, row.AncestorHasNext, row.Expanded, row.Value.Loading, true
}
