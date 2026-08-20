package panel

import (
	"errors"

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
	// LoadGen captures State.treeCollapseGen at the moment this node's async child fetch was
	// dispatched (see setTreeNodeExpanded). ApplyTreeChildLoad compares it against the current
	// generation to detect a straggler: a fetch dispatched before the user's last whole-tree
	// collapse (CollapseAllTree/CollapseAllTreeFully), landing after it. Without this, such a
	// fetch would still set TreeExpanded[dirID] = true on arrival — silently re-expanding a
	// directory (or, if its parent is still expanded elsewhere in the tree, introducing newly
	// visible content) moments after the user asked to collapse everything.
	LoadGen int
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

// MaxExpandAllShallowDepth caps how many successive ExpandAllTreeShallow presses deepen the
// whole tree (each press expands dirs at one more depth level).
const MaxExpandAllShallowDepth = 5

// ErrExpandAllDepthLimit is returned when ExpandAllTreeShallow is pressed after the tree has
// already been deepened to MaxExpandAllShallowDepth.
var ErrExpandAllDepthLimit = errors.New("expand all depth limit")

// MaxExpandAllTotalRows caps how many flattened tree rows ExpandAllTreeFully will grow to before
// it stops cascading further levels, guarding against unbounded memory/render cost on very wide
// trees (depth is already capped by MaxExpandAllShallowDepth; this caps breadth).
const MaxExpandAllTotalRows = 20000

// ErrExpandAllRowLimit is returned when ExpandAllTreeFully's cascade would exceed MaxExpandAllTotalRows.
var ErrExpandAllRowLimit = errors.New("expand all row limit")

// SetListLayout switches the panel's file-list rendering between flat rows and an
// expand/collapse tree. Entering tree mode seeds TreeRoots fresh from the current
// (already-loaded) flat Entries as depth-0 nodes with no children loaded yet, and starts with
// every node collapsed (TreeExpanded reset, not just lazily allocated) — there is no
// cross-session expand-state cache, so a directory expanded in an earlier tree-mode session
// does not silently resume expanded (with a stale Children==nil node) the next time tree mode is
// entered. Leaving tree mode is equivalent to collapsing the whole tree down to depth 0: flat
// Entries are never mutated by tree mode, and the cursor lands on the same depth-0 ancestor
// CollapseAllTree/CollapseTreeCursorRow would leave it on (treeRootAncestorID), not wherever a
// nested expanded row's numeric index happened to fall in the flat listing. Returns false (no-op)
// when CarouselMode is active, matching the existing carousel/listing-format mutual-exclusion
// guard used elsewhere (see ActionPanelListingFormatDialog / ActionPanelCycleListingFormat
// dispatch).
func (s *State) SetListLayout(layout ListLayout, viewportRows int) bool {
	if layout == ListLayoutTree && s.CarouselMode {
		return false
	}
	if s.ListLayout == layout {
		return true
	}
	var cursorAncestorID string
	if layout == ListLayoutFlat && s.ListLayout == ListLayoutTree {
		cursorAncestorID = s.treeRootAncestorID()
	}
	s.ListLayout = layout
	if layout == ListLayoutTree {
		s.TreeRoots = treeRootsFromEntries(s.Entries)
		s.TreeExpanded = make(map[string]bool)
		s.treeExpandAllDepth = 0
		s.treeExpandAllAuto = false
		s.rebuildTreeRows()
	} else {
		// filteredIdx was last built against treeRows; rebuild it against Entries (flat mode's
		// backing space) before translating cursorAncestorID's raw Entries index below.
		s.rebuildFilter()
		if cursorAncestorID != "" {
			for i, e := range s.Entries {
				if e.Path == cursorAncestorID {
					if pos, ok := s.cursorForRawIndex(i); ok {
						s.Cursor = pos
					}
					break
				}
			}
		}
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
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
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	rawIdx, ok := s.rawIndexForCursor()
	if !ok || rawIdx < 0 || rawIdx >= len(s.treeRows) {
		return nil
	}
	row := s.treeRows[rawIdx]
	if row.Value.Entry.Type != localfs.EntryDirectory {
		return nil
	}
	id := row.ID
	if err := s.setTreeNodeExpanded(id, row.Depth, !s.TreeExpanded[id], false); err != nil {
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
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	rawIdx, ok := s.rawIndexForCursor()
	if !ok || rawIdx < 0 || rawIdx >= len(s.treeRows) {
		return nil
	}
	row := s.treeRows[rawIdx]
	if row.Value.Entry.Type != localfs.EntryDirectory || s.TreeExpanded[row.ID] {
		return nil
	}
	id := row.ID
	if err := s.setTreeNodeExpanded(id, row.Depth, true, false); err != nil {
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
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	rawIdx, ok := s.rawIndexForCursor()
	if !ok || rawIdx < 0 || rawIdx >= len(s.treeRows) {
		return nil
	}
	row := s.treeRows[rawIdx]
	if row.Value.Entry.Type == localfs.EntryDirectory && s.TreeExpanded[row.ID] {
		return s.collapseTreeRow(row.ID, row.Depth, viewportRows)
	}
	if row.Depth == 0 {
		return nil
	}
	for i := rawIdx - 1; i >= 0; i-- {
		if s.treeRows[i].Depth < row.Depth {
			return s.collapseTreeRow(s.treeRows[i].ID, s.treeRows[i].Depth, viewportRows)
		}
	}
	return nil
}

// JumpTreeSiblingDir moves the cursor to the previous (delta -1) or next (delta +1) sibling
// directory of the subject row: the cursor directory itself, or — when the cursor is on a file —
// its immediate tree parent (same backward depth walk as CollapseTreeCursorRow). Only same-depth
// directory rows inside the sibling group are considered; files and nested children are skipped.
// No wrap. No-op outside tree mode, on a depth-0 file, or when no sibling directory exists in
// that direction. Does not expand, collapse, or rebuild tree rows.
func (s *State) JumpTreeSiblingDir(delta, viewportRows int) {
	if s.ListLayout != ListLayoutTree || (delta != -1 && delta != 1) {
		return
	}
	rawIdx, ok := s.rawIndexForCursor()
	if !ok || rawIdx < 0 || rawIdx >= len(s.treeRows) {
		return
	}
	row := s.treeRows[rawIdx]
	subjectIdx := rawIdx
	if row.Value.Entry.Type != localfs.EntryDirectory {
		if row.Depth == 0 {
			return
		}
		subjectIdx = -1
		for i := rawIdx - 1; i >= 0; i-- {
			if s.treeRows[i].Depth < row.Depth {
				subjectIdx = i
				break
			}
		}
		if subjectIdx < 0 {
			return
		}
	}
	subjectDepth := s.treeRows[subjectIdx].Depth
	for i := subjectIdx + delta; i >= 0 && i < len(s.treeRows); i += delta {
		r := s.treeRows[i]
		if r.Depth < subjectDepth {
			return
		}
		if r.Depth > subjectDepth || r.Value.Entry.Type != localfs.EntryDirectory {
			continue
		}
		// A matching sibling can itself be filtered out (e.g. an entry filter hiding it); keep
		// scanning in the same direction for the next visible one instead of landing on a row
		// that isn't actually shown.
		if pos, posOK := s.cursorForRawIndex(i); posOK {
			s.Cursor = pos
			s.EnsureCursorInViewport(viewportRows)
			s.syncTreeCursorIDToCursor()
			return
		}
	}
}

// collapseTreeRow collapses the directory row identified by id and moves the cursor onto it.
// Shared by CollapseTreeCursorRow's in-place and jump-to-parent branches.
func (s *State) collapseTreeRow(id string, depth int, viewportRows int) error {
	if err := s.setTreeNodeExpanded(id, depth, false, false); err != nil {
		return err
	}
	s.treeCursorID = id
	s.rebuildTreeRows()
	s.reattachTreeCursorByID(id, viewportRows)
	return nil
}

// CollapseAllTree collapses the whole tree by one level: every directory that is TreeExpanded but
// has no expanded directory child — the deepest currently-open point ("frontier") in its own
// branch — is collapsed, across every branch simultaneously, via collapseExpandedLeaves. This
// collapses by frontier rather than a shared absolute depth so an unevenly deep tree (the norm
// after ExpandAllTreeFully, since each branch expands as deep as it independently goes) still
// shows a visible change in every branch on every press, not just whichever branch is deepest.
// Also disarms any in-progress ExpandAllTreeFully cascade, so a still-loading "expand to max
// depth" can't re-expand what this collapse just closed. treeExpandAllDepth decrements by one per
// press; collapseExpandedLeaves also reports whether anything is left expanded in scope, and once
// nothing is, the counter snaps to 0 immediately rather than drifting above 0 on a tree that
// bottomed out shallower than MaxExpandAllShallowDepth. No-op outside tree mode. The cursor stays
// on its own row when that row survives the peel; otherwise it clamps into range.
func (s *State) CollapseAllTree(viewportRows int) {
	if s.ListLayout != ListLayoutTree {
		return
	}
	s.treeExpandAllAuto = false
	s.treeCollapseGen++
	if s.treeExpandAllDepth == 0 {
		s.CollapseAllTreeFully(viewportRows)
		return
	}
	anchorID := ""
	if rawIdx, ok := s.rawIndexForCursor(); ok && rawIdx >= 0 && rawIdx < len(s.treeRows) {
		anchorID = s.treeRows[rawIdx].ID
	}
	collapsedAny, remainingAny := s.collapseExpandedLeaves(s.TreeRoots, 0)
	if !collapsedAny {
		s.CollapseAllTreeFully(viewportRows)
		return
	}
	if s.treeExpandAllDepth > 0 {
		s.treeExpandAllDepth--
	}
	if !remainingAny {
		s.treeExpandAllDepth = 0
	}
	s.rebuildTreeRows()
	if anchorID != "" {
		s.reattachTreeCursorByID(anchorID, viewportRows)
		return
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// collapseExpandedLeaves collapses every directory that is TreeExpanded but has no expanded
// directory child — the deepest currently-open point in its own branch ("frontier") — leaving
// ancestors above it expanded, and reports (collapsedAny, remainingAny): whether anything was
// collapsed this call, and whether any in-scope directory is still TreeExpanded afterward (used by
// CollapseAllTree to snap treeExpandAllDepth to 0 once nothing is left to peel). Recursing into an
// already-expanded child first means a node with a still-open descendant stays expanded this round
// (only the descendant's frontier peels, and the ancestor counts toward remainingAny); a node
// whose children are all collapsed, unloaded, or absent is itself the frontier and collapses now.
//
// When an entry filter is active, a branch the filter doesn't match is skipped entirely (same
// Match check row-visibility filtering uses), so a press only ever spends itself on branches the
// user can actually see, and never counts a filtered-out branch toward remainingAny either.
// expandAllTreeDirsAtDepth deliberately does NOT apply the same filter skip when expanding (see
// its doc comment: filtering there would race the async git-status fetch a freshly-loaded level's
// own Match depends on), so after "expand all to max depth" under a filter, most of what got
// expanded typically sits outside it — this filter check is what keeps a collapse press from
// landing on one of those invisible branches instead.
func (s *State) collapseExpandedLeaves(nodes []treeflat.Node[TreeEntry], depth int) (collapsedAny, remainingAny bool) {
	for i := range nodes {
		n := &nodes[i]
		if n.Value.Entry.Type != localfs.EntryDirectory || !s.TreeExpanded[n.ID] {
			continue
		}
		if s.ActiveEntryFilter != nil && !s.ActiveEntryFilter.Match(n.Value.Entry, s) {
			continue
		}
		childCollapsed := false
		if n.Children != nil {
			childCollapsed, _ = s.collapseExpandedLeaves(n.Children, depth+1)
		}
		if childCollapsed {
			collapsedAny = true
			remainingAny = true
			continue
		}
		_ = s.setTreeNodeExpanded(n.ID, depth, false, false)
		collapsedAny = true
	}
	return collapsedAny, remainingAny
}

// CollapseAllTreeFully clears all expand state and resets the expand-all deepen counter.
// No-op outside tree mode. The cursor lands on the cursor row's depth-0 ancestor (or the row
// itself if already at depth 0).
func (s *State) CollapseAllTreeFully(viewportRows int) {
	if s.ListLayout != ListLayoutTree {
		return
	}
	s.treeCollapseGen++
	if len(s.TreeExpanded) == 0 && s.treeExpandAllDepth == 0 {
		return
	}
	targetID := s.treeRootAncestorID()
	s.TreeExpanded = nil
	s.treeExpandAllDepth = 0
	s.treeExpandAllAuto = false
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
	rawIdx, ok := s.rawIndexForCursor()
	if !ok || rawIdx < 0 || rawIdx >= len(s.treeRows) {
		return ""
	}
	row := s.treeRows[rawIdx]
	if row.Depth == 0 {
		return row.ID
	}
	for i := rawIdx - 1; i >= 0; i-- {
		if s.treeRows[i].Depth == 0 {
			return s.treeRows[i].ID
		}
	}
	return ""
}

// ExpandAllTreeShallow deepens the whole tree by one level: each successive call expands every
// loaded directory at depth == treeExpandAllDepth (press 1 → depth 0, press 2 → depth 1, …),
// up to MaxExpandAllShallowDepth. Further presses return ErrExpandAllDepthLimit. Async child
// loads are coalesced (treeExpandQuiet) so the list rebuilds once when all in-flight fetches
// finish, and the cursor stays on the row the command was issued from. Does not itself enable
// tree mode — same split of responsibility as ExpandTreeCursorRow.
func (s *State) ExpandAllTreeShallow(viewportRows int) error {
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	if s.treeExpandAllDepth >= MaxExpandAllShallowDepth {
		return ErrExpandAllDepthLimit
	}
	targetDepth := s.treeExpandAllDepth
	// Inside an ExpandAllTreeFully auto-cascade, treeCursorID was already captured once, before
	// the first level, by ExpandAllTreeFully itself — re-deriving it from the current cursor
	// *position* on each subsequent level would pick up the wrong node: earlier levels insert new
	// rows above/below the anchor as other branches expand, so the same numeric cursor position
	// increasingly points at a different row each time, walking the selection to random locations
	// over the course of the cascade. A manual single-level press (treeExpandAllAuto false) has no
	// such drift risk — it runs once, so deriving fresh from the cursor's current row is correct
	// and is what lets each individual press track wherever the user has since moved the cursor.
	anchorID := s.treeCursorID
	if !s.treeExpandAllAuto {
		anchorID = ""
		if rawIdx, ok := s.rawIndexForCursor(); ok && rawIdx >= 0 && rawIdx < len(s.treeRows) {
			anchorID = s.treeRows[rawIdx].ID
		}
		s.treeCursorID = anchorID
	}
	if err := s.expandAllTreeDirsAtDepth(s.TreeRoots, 0, targetDepth); err != nil {
		return err
	}
	s.treeExpandAllDepth++
	// Rebuild treeRows immediately so newly-dispatched directories' Loading state (the
	// scanning-icon indicator) shows up on screen the moment this level starts loading, not only
	// once the whole level's async fetches land.
	s.rebuildTreeRows()
	// Async quiet batch: cursor reattach waits until the last ApplyTreeChildLoad settles.
	if s.treeExpandQuiet > 0 {
		return nil
	}
	if anchorID != "" {
		s.reattachTreeCursorByID(anchorID, viewportRows)
		return nil
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
	return nil
}

// ExpandAllTreeFully deepens the whole tree in one action, from the current expand-all depth up
// to MaxExpandAllShallowDepth — the one-shot mirror of CollapseAllTreeFully. Because deepening
// past level 0 normally waits on async child loads, this arms treeExpandAllAuto and lets
// finishTreeChildLoadApply drive each subsequent level automatically as loads land; see
// driveExpandAllTreeAuto. No-op outside tree mode; returns ErrExpandAllDepthLimit when already
// at max depth.
func (s *State) ExpandAllTreeFully(viewportRows int) error {
	if s.ListLayout != ListLayoutTree {
		return nil
	}
	if s.treeExpandAllDepth >= MaxExpandAllShallowDepth {
		return ErrExpandAllDepthLimit
	}
	s.treeExpandAllAuto = true
	// Captured once, up front: every level of the cascade reattaches the cursor to this same node
	// (see the treeExpandAllAuto branch in ExpandAllTreeShallow) instead of re-deriving it from
	// cursor position on each level, which drifts as sibling branches insert rows.
	s.treeCursorID = ""
	if rawIdx, ok := s.rawIndexForCursor(); ok && rawIdx >= 0 && rawIdx < len(s.treeRows) {
		s.treeCursorID = s.treeRows[rawIdx].ID
	}
	return s.driveExpandAllTreeAuto(viewportRows)
}

// driveExpandAllTreeAuto advances the tree through successive ExpandAllTreeShallow levels while
// treeExpandAllAuto is armed. A level that resolves synchronously (nothing left to load, or no
// scheduler wired) lets the loop continue immediately; a level that dispatches async loads
// (treeExpandQuiet > 0) stops the loop here and returns — finishTreeChildLoadApply re-enters this
// once that batch's ApplyTreeChildLoad calls finish, continuing the cascade until depth 5.
func (s *State) driveExpandAllTreeAuto(viewportRows int) error {
	for s.treeExpandAllDepth < MaxExpandAllShallowDepth {
		if len(s.treeRows) > MaxExpandAllTotalRows {
			s.treeExpandAllAuto = false
			return ErrExpandAllRowLimit
		}
		if err := s.ExpandAllTreeShallow(viewportRows); err != nil {
			return err
		}
		if s.treeExpandQuiet > 0 {
			return nil
		}
	}
	s.treeExpandAllAuto = false
	return nil
}

// expandAllTreeDirsAtDepth expands every directory node at exactly targetDepth. Ancestors above
// targetDepth are walked only when already expanded with loaded children.
//
// Deliberately NOT filter-aware, unlike collapseExpandedLeaves: skipping a branch here would mean
// consulting s.ActiveEntryFilter.Match on a node whose own GitByPath cell may not have arrived
// yet. A directory's git-status cell is only populated once its *parent* has been expanded and
// scheduleTreeChildGitStatus's async fetch for it lands (see tree_load.go) — a separate dispatch
// from the tree-child *listing* fetch that finishTreeChildLoadApply/driveExpandAllTreeAuto
// actually waits on before advancing to the next depth. So the moment expand-all decides whether
// to descend into a level it just loaded, that level's own Match-relevant status can easily still
// be in flight; treating a not-yet-arrived (zero-value) Cell as "doesn't match" would wrongly cut
// the cascade short on branches that do match, well before real max depth. Expanding blind avoids
// the race entirely — collapseExpandedLeaves' filter check plus CollapseAllTree's
// anyExpandedInFilterScope reset (see there) are what actually keep treeExpandAllDepth in sync
// with what's visible, independent of whether expand itself is filter-scoped.
func (s *State) expandAllTreeDirsAtDepth(nodes []treeflat.Node[TreeEntry], depth, targetDepth int) error {
	for i := range nodes {
		n := &nodes[i]
		if n.Value.Entry.Type != localfs.EntryDirectory {
			continue
		}
		if depth == targetDepth {
			if err := s.setTreeNodeExpanded(n.ID, depth, true, true); err != nil {
				return err
			}
			continue
		}
		if depth < targetDepth && s.TreeExpanded[n.ID] && n.Children != nil {
			if err := s.expandAllTreeDirsAtDepth(n.Children, depth+1, targetDepth); err != nil {
				return err
			}
		}
	}
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
// fallback convention as AsyncLoadScheduler), reads the children synchronously inline exactly as
// before. A second expand press while a fetch is already in flight for the same node is a no-op.
//
// quiet=true (ExpandAllTreeShallow) skips the per-dispatch rebuildTreeRows and increments
// treeExpandQuiet so ApplyTreeChildLoad coalesces N async results into one rebuild/redraw.
//
// ponytail: no placeholder "Loading…"/error child row is synthesized while a fetch is in flight —
// treeflat.Flatten only reveals a node's children once Node.Children is non-nil, so a loading
// directory simply shows no children yet (correct, there aren't any to show); the loading
// affordance lives entirely in the row's own icon (TreeEntry.Loading -> FolderIconScanning, see
// panellist.ResolveFolderIconKind). This also makes retry free on failure: TreeExpanded[id] is
// never set and Children stays nil, so pressing expand again just re-enters this function and
// dispatches a new fetch — no separate retry state machine needed.
func (s *State) setTreeNodeExpanded(id string, depth int, expand bool, quiet bool) error {
	if s.TreeExpanded == nil {
		s.TreeExpanded = make(map[string]bool)
	}
	if expand && depth < maxTreeExpandDepth {
		if node := findTreeNode(s.TreeRoots, id); node != nil && node.Children == nil {
			if node.Value.Loading {
				return nil // already loading; don't dispatch a second concurrent fetch
			}
			node.Value.Loading = true
			node.Value.LoadGen = s.treeCollapseGen
			if s.ScheduleTreeChildLoad != nil {
				loc, err := pathloc.Parse(id)
				if err != nil {
					node.Value.Loading = false
					return err
				}
				if s.ScheduleTreeChildLoad(TreeChildLoadRequest{DirID: id, Loc: loc}) {
					if quiet {
						s.treeExpandQuiet++
						return nil
					}
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

// restoreTreeExpansions re-expands the directories recorded in TreeExpanded against a freshly
// rooted TreeRoots (see ApplyListing / ApplyTreeChildLoad): content is always re-fetched, never
// carried over from a stale snapshot, so a remembered expansion can never resurrect stale rows.
//
// Local directories load synchronously here, bypassing ScheduleTreeChildLoad even though the app
// wires it unconditionally for every panel: going through the async dispatch would render one
// frame with the directory collapsed (children nil) before the goroutine's result lands and
// re-expands it — a visible flicker on every return to a previously-expanded local directory.
// Loading inline instead means the first render after navigating back already shows the restored
// state. A directory whose children resolve this way is recursed into immediately, so several
// nested local levels restore within a single call.
//
// Remote directories still go through setTreeNodeExpanded's normal async dispatch — SFTP latency
// makes a synchronous load block the UI thread, the exact regression ScheduleTreeChildLoad was
// introduced to avoid — so a remembered SFTP expansion stays collapsed for one frame while its
// load is in flight, and the matching ApplyTreeChildLoad calls this again to cascade into the
// next level once each result lands.
//
// A load error (either path) drops the id from TreeExpanded so the row renders collapsed and a
// later manual expand retries — same recovery as a failed manual expand.
//
// ponytail: remote dispatches one setTreeNodeExpanded call per remembered directory rather than
// batching through the treeExpandQuiet coalescer ExpandAllTreeShallow uses; a stale dispatch from
// that counter never decrements it (ApplyTreeChildLoad returns early on a vanished node), so
// sharing it across independent per-directory restores risked wedging it permanently on rapid
// navigation. Each restore below is self-contained instead. Revisit if a very large expanded set
// makes restoring an SFTP directory visibly flicker row-by-row as each level lands.
func (s *State) restoreTreeExpansions() {
	s.restoreTreeExpansionsIn(s.TreeRoots, 0)
}

func (s *State) restoreTreeExpansionsIn(nodes []treeflat.Node[TreeEntry], depth int) {
	remote := s.Path.IsRemote()
	for i := range nodes {
		node := &nodes[i]
		if node.Value.Entry.Type != localfs.EntryDirectory || !s.TreeExpanded[node.ID] {
			continue
		}
		if node.Children == nil {
			if remote {
				if err := s.setTreeNodeExpanded(node.ID, depth, true, false); err != nil {
					delete(s.TreeExpanded, node.ID)
				}
				continue // async dispatch in flight; the matching ApplyTreeChildLoad cascades further
			}
			children, err := s.loadTreeChildren(node.ID)
			if err != nil {
				node.Value.LoadErr = err
				delete(s.TreeExpanded, node.ID)
				continue
			}
			node.Value.LoadErr = nil
			node.Children = children
			s.TreeExpanded[node.ID] = true
			entries := make([]localfs.Entry, len(children))
			for j, c := range children {
				entries[j] = c.Value.Entry
			}
			s.scheduleTreeChildGitStatus(node.ID, entries)
		}
		s.restoreTreeExpansionsIn(node.Children, depth+1)
	}
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

// PeekTreeRows rebuilds treeRows from the current TreeRoots/TreeExpanded state without touching
// treeExpandQuiet bookkeeping. Safe to call at any time, including mid-cascade while an
// ExpandAllTreeFully quiet batch is still landing — lets the app throttle a progress repaint
// during a long batch instead of leaving the screen static until the whole level settles.
//
// Reattaches by treeCursorID (the expand-all anchor, or a single-row expand's target) rather than
// just clamping the existing numeric Cursor position: a progress repaint fires mid-cascade, before
// the level that's still loading has settled and reattached the cursor itself, and other branches
// can insert rows above/below the anchor's row in the meantime — clamping alone would leave Cursor
// pointing at whatever row shifted into that same numeric slot, which is exactly what made the
// selection visibly jump to unrelated rows throughout a cascade instead of staying on the
// directory the command was issued from.
func (s *State) PeekTreeRows(viewportRows int) {
	s.rebuildTreeRows()
	if s.treeCursorID != "" {
		s.reattachTreeCursorByID(s.treeCursorID, viewportRows)
		return
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// syncTreeCursorIDToCursor updates treeCursorID to the row currently under the cursor, when in
// tree mode. Called from every user-driven cursor move (Move/Top/Bottom in state.go,
// JumpTreeSiblingDir above) so a still-in-flight ExpandAllTreeFully cascade or single async expand
// keeps reattaching to wherever the user has since moved the cursor, instead of snapping it back
// to the row the command was originally issued from on the next progress repaint or level settle.
// No-op outside tree mode or when the cursor is out of range.
func (s *State) syncTreeCursorIDToCursor() {
	if s.ListLayout != ListLayoutTree {
		return
	}
	if rawIdx, ok := s.rawIndexForCursor(); ok && rawIdx >= 0 && rawIdx < len(s.treeRows) {
		s.treeCursorID = s.treeRows[rawIdx].ID
	}
}

func (s *State) rebuildTreeRows() {
	s.treeRows = treeflat.Flatten(s.TreeRoots, func(id string) bool { return s.TreeExpanded[id] })
	// Unconditional: rebuildFilter also refreshes filteredIdx (see rebuildEntryFilter), which must
	// stay in sync with treeRows even when only an entry filter (not the quick filter) is active —
	// otherwise filteredIdx keeps pointing at stale treeRows positions after an expand/collapse.
	s.rebuildFilter()
}

// reattachTreeCursorByID keeps the cursor on the same node (by ID) after a rebuild when
// possible, else clamps to a valid row.
func (s *State) reattachTreeCursorByID(id string, viewportRows int) {
	for i := range s.treeRows {
		if s.treeRows[i].ID == id {
			// When the row is filtered out, fall through to clampCursor below instead of
			// reassigning — same fallback as when id isn't found in treeRows at all.
			if pos, ok := s.cursorForRawIndex(i); ok {
				s.Cursor = pos
			}
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
	if s.ListLayout != ListLayoutTree {
		return 0, false, nil, false, false, false
	}
	rawIdx, idxOK := s.translateVisibleIndex(index)
	if !idxOK || rawIdx < 0 || rawIdx >= len(s.treeRows) {
		return 0, false, nil, false, false, false
	}
	row := s.treeRows[rawIdx]
	lastChild, ancestorHasNext = row.LastChild, row.AncestorHasNext
	// A filter can remove siblings, so the raw tree structure's connector shape (computed by
	// treeflat.Flatten over every child, filtered or not) is wrong for what's actually on screen —
	// e.g. a directory whose only surviving visible child would still draw "├─" instead of "└─".
	// filteredTreeShape holds the shape recomputed for the filtered display sequence instead.
	if s.ActiveEntryFilter != nil && index >= 0 && index < len(s.filteredTreeShape) {
		lastChild = s.filteredTreeShape[index].LastChild
		ancestorHasNext = s.filteredTreeShape[index].AncestorHasNext
	}
	return row.Depth, lastChild, ancestorHasNext, row.Expanded, row.Value.Loading, true
}

// treeConnectorShape holds the recomputed LastChild/AncestorHasNext connector fields for one row
// of the filtered display sequence — see recomputeFilteredTreeConnectors.
type treeConnectorShape struct {
	LastChild       bool
	AncestorHasNext []bool
}

// recomputeFilteredTreeConnectors derives LastChild/AncestorHasNext for the filtered display
// sequence (s.filteredIdx) instead of trusting treeRows' structural values, which are computed by
// treeflat.Flatten over the full unfiltered child list. Mirrors Flatten's own algorithm but works
// from a plain sequence of row depths (post-filtering) rather than a Node tree — a row is
// LastChild when no later row in the filtered sequence at the same depth appears before a
// shallower one does; AncestorHasNext[k] tracks whether the ancestor at depth k+1 has such a
// following sibling too, exactly as Flatten defines it. Called after filteredIdx is rebuilt; no-op
// (clears the cache) outside tree mode or when filteredIdx is empty.
func (s *State) recomputeFilteredTreeConnectors() {
	if s.ListLayout != ListLayoutTree || len(s.filteredIdx) == 0 {
		s.filteredTreeShape = nil
		return
	}
	n := len(s.filteredIdx)
	depths := make([]int, n)
	for i, rawIdx := range s.filteredIdx {
		depths[i] = s.treeRows[rawIdx].Depth
	}
	isLast := make([]bool, n)
	openAtDepth := make(map[int]int, n)
	for i, d := range depths {
		if prev, ok := openAtDepth[d]; ok {
			isLast[prev] = false
		}
		isLast[i] = true
		openAtDepth[d] = i
		for dd := range openAtDepth {
			if dd > d {
				delete(openAtDepth, dd)
			}
		}
	}
	shapes := make([]treeConnectorShape, n)
	var stack []bool
	for i, d := range depths {
		if len(stack) > d {
			stack = stack[:d]
		}
		var ancestorHasNext []bool
		if d > 0 && len(stack) > 1 {
			ancestorHasNext = append([]bool(nil), stack[1:]...)
		}
		shapes[i] = treeConnectorShape{LastChild: isLast[i], AncestorHasNext: ancestorHasNext}
		hasNext := !isLast[i]
		if len(stack) == d {
			stack = append(stack, hasNext)
		} else {
			stack[d] = hasNext
		}
	}
	s.filteredTreeShape = shapes
}
