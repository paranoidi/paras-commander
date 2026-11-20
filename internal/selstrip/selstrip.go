// Package selstrip holds the cursor, scroll and quick-filter state of the Selected strip,
// shared by the file panel and the find-duplicates view. The caller owns the path list.
package selstrip

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// State is the strip's view state; rows come from the caller (n rows / paths).
type State struct {
	Cursor int
	Scroll int
	// CursorPath is the last focused row path; restores the cursor on re-focus.
	CursorPath string
	// Filter is the strip quick filter (basename fuzzy match).
	Filter quickfilter.Filter
	// Tree lists the rows as a folded directory tree instead of one flat row per path. The
	// session-wide mode lives on the file panels' strips (ui.Model.SelectedStripTree).
	Tree bool
	// Collapsed holds the folder rows (by Path) folded shut in tree mode; CollapsedRev bumps
	// on every change so callers can key row caches on it.
	Collapsed    map[string]bool
	CollapsedRev int
}

// Row is one strip row: a selected path, or (tree mode) a synthetic folder grouping them.
type Row struct {
	// Path is the selected path, or the folder's directory path for a Group row.
	Path string
	// Display is the tree label ("file.txt", folded "foo/bar"); empty in flat mode, where the
	// renderer fits Path relative to the base directory itself.
	Display string
	Group   bool
	// Expanded is false for a tree-mode folder row that is collapsed.
	Expanded bool
	// Depth, LastChild and AncestorHasNext shape the tree connector (treeflat.Row semantics).
	Depth           int
	LastChild       bool
	AncestorHasNext []bool
}

// Rows lists paths as strip rows: one per path (flat), or a folded tree of their labels
// relative to baseDir (tree; tilde-less absolute when baseDir does not contain the path), with
// the folders in collapsed hiding their children.
func Rows(paths []string, baseDir string, tree bool, collapsed map[string]bool) []Row {
	if !tree {
		return FlatRows(paths)
	}
	labels := make([]string, len(paths))
	for i, p := range paths {
		labels[i] = Label(baseDir, p)
	}
	return TreeRows(treeflat.FromPaths(labels, paths), collapsed)
}

// FlatRows is one row per path.
func FlatRows(paths []string) []Row {
	rows := make([]Row, len(paths))
	for i, p := range paths {
		rows[i] = Row{Path: p}
	}
	return rows
}

// Label is p's tree label: its "/"-separated path relative to baseDir, or p itself when
// that cannot be expressed. The single source of the labels fed to treeflat.PathTree.
func Label(baseDir, p string) string {
	label, err := filepath.Rel(baseDir, p)
	if err != nil {
		label = p
	}
	return filepath.ToSlash(label)
}

// TreeRows flattens a path tree (labels from Label, ids the paths) into strip rows, with the
// folders in collapsed hiding their children.
func TreeRows(nodes []treeflat.Node[treeflat.PathNode], collapsed map[string]bool) []Row {
	flat := treeflat.Flatten(nodes, func(id string) bool { return !collapsed[id] })
	rows := make([]Row, len(flat))
	for i, r := range flat {
		rows[i] = Row{
			Path: r.Value.Path, Display: r.Value.Display, Group: r.Value.Group, Expanded: r.Expanded,
			Depth: r.Depth, LastChild: r.LastChild, AncestorHasNext: r.AncestorHasNext,
		}
	}
	return rows
}

// RowNames returns the basename of each row's path (the strip filter's match text).
func RowNames(rows []Row) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = filepath.Base(r.Path)
	}
	return names
}

// Clamp keeps Cursor inside n rows and Scroll inside the viewport (vr <= 0 skips viewport math).
func (s *State) Clamp(n, vr int) {
	if n == 0 {
		s.Cursor = 0
		s.Scroll = 0
		return
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Cursor >= n {
		s.Cursor = n - 1
	}
	if vr <= 0 {
		if s.Scroll > s.Cursor {
			s.Scroll = s.Cursor
		}
		return
	}
	maxOffset := n - vr
	if maxOffset < 0 {
		maxOffset = 0
		s.Scroll = 0
	}
	if s.Scroll > maxOffset {
		s.Scroll = maxOffset
	}
	if s.Scroll < 0 {
		s.Scroll = 0
	}
	if s.Cursor < s.Scroll {
		s.Scroll = s.Cursor
	}
	if s.Cursor >= s.Scroll+vr {
		s.Scroll = s.Cursor - vr + 1
	}
}

// Move shifts the cursor by delta and keeps it visible.
func (s *State) Move(delta, n, vr int) {
	if n == 0 {
		s.Cursor = 0
		s.Scroll = 0
		return
	}
	s.Cursor += delta
	s.Clamp(n, vr)
}

// Top moves the cursor to the first row.
func (s *State) Top(n, vr int) {
	if n == 0 {
		return
	}
	s.Cursor = 0
	s.Scroll = 0
	s.Clamp(n, vr)
}

// Bottom moves the cursor to the last row.
func (s *State) Bottom(n, vr int) {
	if n == 0 {
		return
	}
	s.Cursor = n - 1
	s.Clamp(n, vr)
}

// Remember records the path under the cursor.
func (s *State) Remember(rows []Row) {
	if s.Cursor >= 0 && s.Cursor < len(rows) {
		s.CursorPath = rows[s.Cursor].Path
	}
}

// Restore moves the cursor back to CursorPath when it is still listed.
func (s *State) Restore(rows []Row) {
	if s.CursorPath == "" {
		return
	}
	for i, r := range rows {
		if r.Path == s.CursorPath {
			s.Cursor = i
			return
		}
	}
}

// OpenFilter starts editing the quick filter.
func (s *State) OpenFilter(names []string, opts quickfilter.Options, vr int) {
	s.Filter.Editing = true
	s.Filter.Rebuild(names, opts)
	s.Clamp(len(names), vr)
}

// AcceptFilter exits editing while keeping the current filtered cursor.
func (s *State) AcceptFilter(n, vr int) {
	s.Filter.Editing = false
	s.Filter.Active = s.Filter.Query != ""
	if !s.Filter.Active {
		s.Filter.ClearResults()
	}
	s.Clamp(n, vr)
}

// CancelFilter exits editing and clears the query.
func (s *State) CancelFilter(names []string, opts quickfilter.Options, vr int) {
	s.Filter.Editing = false
	s.applyQuery("", names, opts, vr)
}

// ClearFilter removes the query while preserving edit mode.
func (s *State) ClearFilter(names []string, opts quickfilter.Options, vr int) {
	editing := s.Filter.Editing
	s.applyQuery("", names, opts, vr)
	s.Filter.Editing = editing
}

// AppendFilterRune appends a printable rune to the query.
func (s *State) AppendFilterRune(r rune, names []string, opts quickfilter.Options, vr int) {
	s.applyQuery(s.Filter.InsertRune(r), names, opts, vr)
}

// BackspaceFilter removes the last rune from the query.
func (s *State) BackspaceFilter(names []string, opts quickfilter.Options, vr int) {
	if next, changed := s.Filter.Backspace(); changed {
		s.applyQuery(next, names, opts, vr)
	}
}

// CycleFilterMatch moves the cursor through fuzzy matches (or plain Move when none).
func (s *State) CycleFilterMatch(delta, n, vr int, opts quickfilter.Options) {
	cur, ok := s.Filter.Cycle(s.Cursor, delta, opts)
	if !ok {
		s.Move(delta, n, vr)
		return
	}
	s.Cursor = cur
	s.Clamp(n, vr)
}

func (s *State) applyQuery(query string, names []string, opts quickfilter.Options, vr int) {
	if cur, ok := s.Filter.Apply(query, names, opts); ok {
		s.Cursor = cur
	}
	s.Clamp(len(names), vr)
}

// MapBasenameRangesToDisplay offsets basename match ranges onto a display path that
// ends with that basename (common for Rel labels). Returns nil when the basename is not a suffix.
func MapBasenameRangesToDisplay(display, absPath string, baseRanges []search.Range) []search.Range {
	if len(baseRanges) == 0 {
		return nil
	}
	base := filepath.Base(absPath)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return nil
	}
	dispRunes := []rune(display)
	baseRunes := []rune(base)
	if len(baseRunes) == 0 || len(dispRunes) < len(baseRunes) {
		return nil
	}
	offset := len(dispRunes) - len(baseRunes)
	if string(dispRunes[offset:]) != base {
		// Truncated display — try last occurrence of basename as a substring.
		idx := strings.LastIndex(display, base)
		if idx < 0 {
			return nil
		}
		offset = len([]rune(display[:idx]))
	}
	out := make([]search.Range, len(baseRanges))
	for i, r := range baseRanges {
		out[i] = search.Range{Start: r.Start + offset, End: r.End + offset}
	}
	return out
}

// Expand opens the collapsed folder row under the cursor. It reports whether the rows changed.
func (s *State) Expand(rows []Row) bool {
	if s.Cursor < 0 || s.Cursor >= len(rows) || !rows[s.Cursor].Group || !s.Collapsed[rows[s.Cursor].Path] {
		return false
	}
	delete(s.Collapsed, rows[s.Cursor].Path)
	s.CollapsedRev++
	return true
}

// Collapse folds the open folder row under the cursor, or moves the cursor to the row's parent
// folder (as the other tree views do). It reports whether the rows changed.
func (s *State) Collapse(rows []Row) bool {
	if s.Cursor < 0 || s.Cursor >= len(rows) {
		return false
	}
	row := rows[s.Cursor]
	if row.Group && row.Expanded {
		if s.Collapsed == nil {
			s.Collapsed = map[string]bool{}
		}
		s.Collapsed[row.Path] = true
		s.CollapsedRev++
		return true
	}
	for i := s.Cursor - 1; i >= 0; i-- {
		if rows[i].Depth < row.Depth {
			s.Cursor = i
			break
		}
	}
	return false
}

// ExpandAll opens every folder. The cursor stays on its row. The bulk fold methods act only on
// tree rows (a flat list has no folder rows).
func (s *State) ExpandAll(rows []Row) {
	if !hasGroup(rows) || len(s.Collapsed) == 0 {
		return
	}
	s.Remember(rows)
	s.Collapsed = nil
	s.CollapsedRev++
}

// ExpandLevel opens every collapsed folder that is visible, one tree level per press.
func (s *State) ExpandLevel(rows []Row) {
	s.Remember(rows)
	s.setCollapsed(rows, func(r Row) bool { return r.Group && !r.Expanded }, false)
}

// CollapseLevel folds the deepest open folders, one tree level per press.
func (s *State) CollapseLevel(rows []Row) {
	deepest := -1
	for _, r := range rows {
		if r.Group && r.Expanded {
			deepest = max(deepest, r.Depth)
		}
	}
	s.collapse(rows, func(r Row) bool { return r.Group && r.Expanded && r.Depth == deepest }, false)
}

// CollapseAll folds every folder of the tree over paths (rows is the visible list), leaving
// only the top-level rows.
func (s *State) CollapseAll(paths []string, baseDir string, rows []Row) {
	if !hasGroup(rows) {
		return
	}
	s.collapse(rows, func(Row) bool { return false }, true)
	for _, r := range Rows(paths, baseDir, true, nil) {
		if r.Group && !s.Collapsed[r.Path] {
			s.Collapsed[r.Path] = true
			s.CollapsedRev++
		}
	}
}

// collapse folds the visible folders matching fold and points CursorPath at the cursor row, or
// at its shallowest ancestor that will be folded (every ancestor when all is set, as CollapseAll
// folds them all), so Restore lands on a visible row once the caller rebuilds the rows.
func (s *State) collapse(rows []Row, fold func(Row) bool, all bool) {
	if !hasGroup(rows) || s.Cursor < 0 || s.Cursor >= len(rows) {
		return
	}
	if s.Collapsed == nil {
		s.Collapsed = map[string]bool{}
	}
	s.Remember(rows)
	// Ancestors of the cursor row, shallowest first.
	var anc []Row
	for i, d := s.Cursor-1, rows[s.Cursor].Depth; i >= 0 && d > 0; i-- {
		if rows[i].Depth < d {
			anc = append([]Row{rows[i]}, anc...)
			d = rows[i].Depth
		}
	}
	for _, a := range anc {
		if all || fold(a) {
			s.CursorPath = a.Path
			break
		}
	}
	s.setCollapsed(rows, fold, true)
}

// setCollapsed sets the collapsed state of the folder rows matching fn.
func (s *State) setCollapsed(rows []Row, fn func(Row) bool, collapsed bool) {
	for _, r := range rows {
		if !fn(r) {
			continue
		}
		if collapsed {
			if s.Collapsed == nil {
				s.Collapsed = map[string]bool{}
			}
			s.Collapsed[r.Path] = true
		} else {
			delete(s.Collapsed, r.Path)
		}
		s.CollapsedRev++
	}
}

func hasGroup(rows []Row) bool {
	for _, r := range rows {
		if r.Group {
			return true
		}
	}
	return false
}
