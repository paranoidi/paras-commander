package ui

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// DedupCache memoizes what only changes with the snapshot, the display toggles or
// the marks: the built node tree (collapse state only affects the flatten) with its
// main-pane Size/Count column widths (over every node of the tree, not only the
// visible rows, so expanding or collapsing never shifts the columns), the
// mark/kept directory sets and the copies-pane column widths. DedupViewState.Cache is a
// pointer so value copies of the view share it; a nil Cache computes directly.
// Keys: the snapshot's Groups slice identity (&Groups[0] and len; the cache holds
// the pointer, so the address cannot be reused), TreeDirs, SortByWasted,
// IgnoreEmpty and DedupViewState.GroupFilterRev for the tree; the same slice
// identity plus DedupViewState.MarksRev for the mark sets. Whoever assigns
// GroupFilter bumps GroupFilterRev, whoever changes Marked or Kept bumps MarksRev.
// Used from the UI goroutine only.
type DedupCache struct {
	tree      *dedupTree
	marks     *dedupMarkSets
	copyMarks *dedupCopyMarkSet
	widths    dedupWidths // copies pane
}

// dedupGroupsKey identifies a snapshot's Groups slice by its first element.
type dedupGroupsKey struct {
	first *comparepkg.DedupGroup
	n     int
}

func dedupGroupsID(g []comparepkg.DedupGroup) dedupGroupsKey {
	if len(g) == 0 {
		return dedupGroupsKey{}
	}
	return dedupGroupsKey{&g[0], len(g)}
}

type dedupTreeKey struct {
	groups                              dedupGroupsKey
	treeDirs, sortByWasted, ignoreEmpty bool
	filterRev                           int
}

// dedupTree is the built node tree for one key; the flatten step runs per call.
type dedupTree struct {
	key          dedupTreeKey
	roots        []treeflat.Node[*DedupRowData]
	ignoredEmpty int // files dropped by the ignore-empty filter
	ids          []string
	idsDone      bool
	sizeW        int // Size column width over every node, see columnWidths
	countW       int
	widthsDone   bool
}

// columnWidths is the Size/Count column width over every node of the tree,
// collapsed or not (so toggling a folder never changes the layout), computed
// once per tree.
func (t *dedupTree) columnWidths() (sizeW, countW int) {
	if !t.widthsDone {
		s, c := len([]rune(dedupListSizeTitle)), len([]rune(dedupListCountTitle))
		treeflat.Walk(t.roots, func(n *treeflat.Node[*DedupRowData], _ int) {
			st, ct := dedupRowDetailTexts(n.Value)
			s, c = max(s, len([]rune(st))), max(c, len([]rune(ct)))
		})
		t.sizeW, t.countW, t.widthsDone = max(s, 1), max(c, 1), true
	}
	return t.sizeW, t.countW
}

// DedupWalkNodes visits every node of the cached tree for snap/view depth-first,
// expanded or not, with its row depth. Nothing is visited until the scan is done.
func DedupWalkNodes(snap comparepkg.DedupSnapshot, view DedupViewState, fn func(id string, depth int, hasChildren bool)) {
	if snap.Phase != comparepkg.DedupDone {
		return
	}
	treeflat.Walk(dedupTreeFor(snap, view).roots, func(n *treeflat.Node[*DedupRowData], depth int) {
		fn(n.ID, depth, len(n.Children) > 0)
	})
}

// expandableIDs lists the collapsible node IDs; callers must not modify the result.
func (t *dedupTree) expandableIDs() []string {
	if !t.idsDone {
		t.ids, t.idsDone = treeflat.ExpandableIDs(t.roots), true
	}
	return t.ids
}

// dedupTreeFor returns the tree for snap/view, reusing the cached one on a key match.
func dedupTreeFor(snap comparepkg.DedupSnapshot, view DedupViewState) *dedupTree {
	key := dedupTreeKey{
		groups:       dedupGroupsID(snap.Groups),
		treeDirs:     view.TreeDirs,
		sortByWasted: view.SortByWasted && !view.TreeDirs, // the order only applies to groups mode
		ignoreEmpty:  view.IgnoreEmpty,
		filterRev:    view.GroupFilterRev,
	}
	c := view.Cache
	if c != nil && c.tree != nil && c.tree.key == key {
		return c.tree
	}
	t := &dedupTree{key: key}
	idxs := make([]int, 0, len(snap.Groups))
	for i, g := range snap.Groups {
		if view.IgnoreEmpty && g.Size == 0 {
			t.ignoredEmpty += len(g.Files)
			continue
		}
		if !view.groupVisible(g) {
			continue
		}
		idxs = append(idxs, i)
	}
	if view.TreeDirs {
		t.roots = dedupDirRoots(snap, idxs, nil)
	} else {
		t.roots = dedupGroupRoots(snap, idxs, view.SortByWasted)
	}
	if c != nil {
		c.tree = t
	}
	return t
}

// dedupMarkSets are the snapshot-wide directory sets the tree rows are painted from.
type dedupMarkSets struct {
	groups                         dedupGroupsKey
	rev                            int
	marked, danger, kept, treeFull map[string]bool
}

// dedupMarkSetsFor returns the mark/kept dir sets, recomputed only when the
// snapshot groups or MarksRev change.
func dedupMarkSetsFor(snap comparepkg.DedupSnapshot, view DedupViewState) *dedupMarkSets {
	id := dedupGroupsID(snap.Groups)
	c := view.Cache
	if c != nil && c.marks != nil && c.marks.groups == id && c.marks.rev == view.MarksRev {
		return c.marks
	}
	m := &dedupMarkSets{
		groups:   id,
		rev:      view.MarksRev,
		marked:   dedupMarkedDirSet(snap, view.Marked),
		danger:   dedupDangerMarkedDirSet(snap, view.Marked),
		kept:     dedupKeptDirSet(snap, view.Kept),
		treeFull: dedupSnapshotFullyMarkedDirSet(snap, view.Marked),
	}
	if c != nil {
		c.marks = m
	}
	return m
}

// dedupCopyMarkSet is the copies-pane fully-marked set of one source file row.
type dedupCopyMarkSet struct {
	groups   dedupGroupsKey
	rev      int
	absKey   string
	groupIdx int
	set      map[string]bool
}

// dedupCopyFullyMarkedFor memoizes dedupCopyPaneFullyMarkedDirSet for a file row
// (it reads only the snapshot groups, the marks and the row's AbsKey/GroupIdx).
func dedupCopyFullyMarkedFor(snap comparepkg.DedupSnapshot, view DedupViewState, sel DedupRow) map[string]bool {
	id := dedupGroupsID(snap.Groups)
	c := view.Cache
	if c != nil && c.copyMarks != nil {
		m := c.copyMarks
		if m.groups == id && m.rev == view.MarksRev && m.absKey == sel.Value.AbsKey && m.groupIdx == sel.Value.GroupIdx {
			return m.set
		}
	}
	set := dedupCopyPaneFullyMarkedDirSet(snap, view, sel)
	if c != nil {
		c.copyMarks = &dedupCopyMarkSet{id, view.MarksRev, sel.Value.AbsKey, sel.Value.GroupIdx, set}
	}
	return set
}

// dedupWidths is the column widths of one row slice, kept with the slice so its
// address stays unique while cached.
type dedupWidths struct {
	rows          []DedupRow
	sizeW, countW int
}

// columnWidths is dedupListColumnWidths over all rows of the copies pane (so
// widths do not jitter on scroll), computed once per row slice.
// ponytail: same &rows[0] and len means same content; true while row slices are
// rebuilt wholesale and never edited in place.
func (c *DedupCache) columnWidths(rows []DedupRow) (sizeW, countW int) {
	if c == nil || len(rows) == 0 {
		return dedupListColumnWidths(rows)
	}
	w := &c.widths
	if len(w.rows) != len(rows) || &w.rows[0] != &rows[0] {
		w.sizeW, w.countW = dedupListColumnWidths(rows)
		w.rows = rows
	}
	return w.sizeW, w.countW
}
