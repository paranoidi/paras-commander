package ui

import (
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/selstrip"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// DedupMainRects splits the dedup view's primary column into the results tree and the
// Selected strip listing the marked files (strip.Height == 0: none). Single source for
// drawing, the main pane's visible rows and mouse hit-testing.
func DedupMainRects(primary Rect, model *Model) (tree, strip Rect) {
	return SplitPanelForSelected(primary, SelectedStripSplitParams{
		StripItemCount:     len(DedupStripRows(model.DedupSnapshot, model.DedupView, model.SelectedStripTree())),
		MaxRows:            model.SelectedPanelMaxRows,
		ActivePercent:      model.SelectedPanelActivePercent,
		StripFocused:       model.DedupView.FocusSelected,
		Orientation:        model.SplitOrientation,
		MinFileContentRows: MinFileListContentRows,
	})
}

// dedupMarkedPaths is the Selected strip's cache: the sorted marked paths, the label tree over
// them and the strip rows built from either.
//
// paths reflect MarksRev rev. A journal of the per-file changes since then lets the next use
// move paths (and tree) by just those files instead of resorting every mark: NoteMarkChange
// appends to it while jrev (the MarksRev the journal runs up to) equals the revision before
// the bump, anything else drops it (jok false) and the next use rebuilds from view.Marked.
type dedupMarkedPaths struct {
	rev   int
	paths []string
	gen   int // bumped whenever paths changes, the rows' key

	journal map[string]bool // abs -> marked now, for the files changed since rev
	jrev    int
	jok     bool

	tree    *treeflat.PathTree // labels of paths, kept in step with them; nil: built on the next tree-mode use
	treeKey dedupStripTreeKey

	rows    []selstrip.Row
	rowsKey dedupStripKey
	rowsOK  bool
}

// dedupStripKey is what the cached rows were built from; treeKey only matters in tree mode.
type dedupStripKey struct {
	gen          int
	tree         bool
	collapsedRev int
	treeKey      dedupStripTreeKey
}

// dedupStripTreeKey is what the label tree was built for: the label base and the snapshot.
type dedupStripTreeKey struct {
	base   string
	groups dedupGroupsKey
}

// dedupJournalTooBig reports whether k journaled changes over n paths are not worth applying
// one by one.
// ponytail: fixed crossover (a full resort costs about what 1/8 of the marks cost to merge and
// re-label); retune with BenchmarkDedupStripBatch if the numbers move.
func dedupJournalTooBig(k, n int) bool { return k > 64 && k*8 > n }

// note records one change from NoteMarkChange: old and now are MarksRev before and after it.
// A keep (not a mark) only keeps the journal in step with the revision.
func (m *dedupMarkedPaths) note(old, now int, absKey string, keep, on bool) {
	if m.paths == nil || !m.jok || m.jrev != old {
		m.jok, m.journal = false, nil
		return
	}
	m.jrev = now
	if keep {
		return
	}
	if m.journal == nil {
		m.journal = map[string]bool{}
	}
	m.journal[absKey] = on
	if dedupJournalTooBig(len(m.journal), len(m.paths)) {
		m.jok, m.journal = false, nil
	}
}

// replay moves paths (and tree) to marks revision rev by the journal, in O(n + k log k) for k
// changes; false when the journal does not cover rev or does not add up (the caller rebuilds).
func (m *dedupMarkedPaths) replay(rev, marked int) bool {
	if m.paths == nil || !m.jok || m.jrev != rev || dedupJournalTooBig(len(m.journal), len(m.paths)) {
		return false
	}
	if len(m.journal) > 0 {
		edits := make([]selstrip.Edit, 0, len(m.journal))
		for p, on := range m.journal {
			edits = append(edits, selstrip.Edit{Path: p, On: on})
		}
		slices.SortFunc(edits, func(a, b selstrip.Edit) int { return strings.Compare(a.Path, b.Path) })
		// A mismatch leaves the tree edited, but the caller's rebuild drops it with the paths.
		out := selstrip.ApplyEdits(m.paths, edits, m.tree, m.treeKey.base)
		if len(out) != marked {
			return false
		}
		m.paths = out
		m.gen++
		clear(m.journal)
	}
	m.rev, m.jrev = rev, rev
	return true
}

// DedupMarkedPaths returns the marked paths in sorted order, cached per MarksRev and moved by
// the noted changes (see dedupMarkedPaths) rather than resorted. The slice is shared with the
// cache: callers must not modify it.
func DedupMarkedPaths(view DedupViewState) []string {
	c := view.Cache
	if c != nil {
		m := &c.marked
		if m.paths != nil && m.rev == view.MarksRev && len(m.paths) == len(view.Marked) {
			return m.paths
		}
		if m.replay(view.MarksRev, len(view.Marked)) {
			return m.paths
		}
	}
	paths := make([]string, 0, len(view.Marked))
	for p := range view.Marked {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	if c != nil {
		c.marked = dedupMarkedPaths{rev: view.MarksRev, paths: paths, gen: c.marked.gen + 1, jok: true, jrev: view.MarksRev}
	}
	return paths
}

// DedupStripRows returns the strip's rows: the marked paths, flat or as a folded tree (tree),
// cached per change of the marked paths. The slice is shared with the cache: callers must not
// modify it.
func DedupStripRows(snap comparepkg.DedupSnapshot, view DedupViewState, tree bool) []selstrip.Row {
	paths := DedupMarkedPaths(view) // first: it moves c.marked.gen, part of the rows key
	c := view.Cache
	var key dedupStripKey
	if c != nil {
		key = dedupStripKey{gen: c.marked.gen, tree: tree, collapsedRev: view.Strip.CollapsedRev}
		if tree {
			key.treeKey = dedupStripTreeKeyFor(snap)
		}
		if c.marked.rowsOK && c.marked.rowsKey == key {
			return c.marked.rows
		}
	}
	var rows []selstrip.Row
	if tree {
		rows = selstrip.TreeRows(DedupStripTreeNodes(snap, view), view.Strip.Collapsed)
	} else {
		rows = selstrip.FlatRows(paths)
	}
	if c != nil {
		c.marked.rows, c.marked.rowsKey, c.marked.rowsOK = rows, key, true
	}
	return rows
}

// dedupStripTreeKeyFor is what a strip tree of snap is built for: its labels' base and groups.
func dedupStripTreeKeyFor(snap comparepkg.DedupSnapshot) dedupStripTreeKey {
	return dedupStripTreeKey{snap.EffectiveDisplayRoot().String(), dedupGroupsID(snap.Groups)}
}

// DedupStripTreeNodes returns the full label tree of the marked paths (every folder open),
// cached and moved with the marks. Shared with the cache: read only.
func DedupStripTreeNodes(snap comparepkg.DedupSnapshot, view DedupViewState) []treeflat.Node[treeflat.PathNode] {
	paths := DedupMarkedPaths(view)
	key := dedupStripTreeKeyFor(snap)
	c := view.Cache
	if c == nil {
		return selstrip.NewTree(paths, key.base).Nodes()
	}
	m := &c.marked
	if m.tree == nil || m.treeKey != key {
		m.tree, m.treeKey = selstrip.NewTree(paths, key.base), key
	}
	return m.tree.Nodes()
}

// drawDedupStrip paints the Selected strip of marked files.
func drawDedupStrip(screen tcell.Screen, rect Rect, model Model, styles theme.Theme, chromeBlocked bool) {
	view := model.DedupView
	if rect.Height <= 0 || model.DedupSnapshot.Phase != comparepkg.DedupDone {
		return
	}
	drawSelectedStrip(screen, rect, SelectedStripData{
		Rows:      DedupStripRows(model.DedupSnapshot, view, model.SelectedStripTree()),
		Strip:     view.Strip,
		Tree:      model.SelectedStripTree(),
		BaseDir:   model.DedupSnapshot.EffectiveDisplayRoot().String(),
		SizeLabel: view.MarkedSummary(),
	}, view.FocusSelected, chromeBlocked, SelectedStripOpts{
		Styles: styles, UserHomeDir: model.UserHomeDir, ScrollbarStyle: model.PanelScrollbar,
		ScrollbarShowInactive: model.PanelScrollbarInactive,
		ViMotionActive:        model.ViMotionMode && view.FocusSelected,
	})
}

// DedupRevealPath locates absKey (a file or, in the directory tree, a directory) in the main
// tree: the node ID of its row and the IDs of the ancestors that must be expanded to show it. ok
// is false when it has no row (hidden by the ignore-empty or group filter, or a directory in the
// groups view).
func DedupRevealPath(snap comparepkg.DedupSnapshot, view DedupViewState, absKey string) (id string, ancestors []string, ok bool) {
	if snap.Phase != comparepkg.DedupDone {
		return "", nil, false
	}
	var find func(nodes []treeflat.Node[*DedupRowData], path []string) bool
	find = func(nodes []treeflat.Node[*DedupRowData], path []string) bool {
		for i := range nodes {
			n := &nodes[i]
			if (n.Value.Kind == DedupRowFile || n.Value.Kind == DedupRowDir) && dedupRowAbsPath(snap, n.Value) == absKey {
				id, ancestors = n.ID, slices.Clone(path)
				return true
			}
			if find(n.Children, append(path, n.ID)) {
				return true
			}
		}
		return false
	}
	return id, ancestors, find(dedupTreeFor(snap, view).roots, nil)
}
