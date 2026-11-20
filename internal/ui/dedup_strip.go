package ui

import (
	"slices"

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

// dedupMarkedPaths is the sorted marked paths of one marks revision, and the strip rows built
// from them for one tree mode.
type dedupMarkedPaths struct {
	rev, n   int
	paths    []string
	rows     []selstrip.Row
	rowsTree bool
	rowsRev  int // Strip.CollapsedRev the rows were built with
	rowsOK   bool
}

// DedupMarkedPaths returns the marked paths in sorted order, cached per MarksRev. The slice is
// shared with the cache: callers must not modify it.
func DedupMarkedPaths(view DedupViewState) []string {
	c := view.Cache
	if c != nil && c.marked.paths != nil && c.marked.rev == view.MarksRev && c.marked.n == len(view.Marked) {
		return c.marked.paths
	}
	paths := make([]string, 0, len(view.Marked))
	for p := range view.Marked {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	if c != nil {
		c.marked = dedupMarkedPaths{rev: view.MarksRev, n: len(view.Marked), paths: paths}
	}
	return paths
}

// DedupStripRows returns the strip's rows: the marked paths, flat or as a folded tree (tree),
// cached per marks revision. The slice is shared with the cache: callers must not modify it.
func DedupStripRows(snap comparepkg.DedupSnapshot, view DedupViewState, tree bool) []selstrip.Row {
	paths := DedupMarkedPaths(view)
	c := view.Cache
	if c != nil && c.marked.rowsOK && c.marked.rowsTree == tree && c.marked.rowsRev == view.Strip.CollapsedRev && c.marked.rev == view.MarksRev && c.marked.n == len(view.Marked) {
		return c.marked.rows
	}
	rows := selstrip.Rows(paths, snap.EffectiveDisplayRoot().String(), tree, view.Strip.Collapsed)
	if c != nil {
		c.marked.rows, c.marked.rowsTree, c.marked.rowsRev, c.marked.rowsOK = rows, tree, view.Strip.CollapsedRev, true
	}
	return rows
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
