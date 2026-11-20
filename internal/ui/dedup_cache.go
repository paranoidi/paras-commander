package ui

import (
	"iter"
	"slices"
	"sort"
	"strings"

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
// The snapshot index (dedupIndex) is keyed by the Groups and DirGroups slice
// identities. Used from the UI goroutine only.
type DedupCache struct {
	tree      *dedupTree
	marks     *dedupMarkSets
	copyMarks *dedupCopyMarkSet
	widths    dedupWidths // copies pane
	index     *dedupIndex
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
	ignoredEmpty int      // files dropped by the ignore-empty filter
	ids          []string // expandable node IDs, depth-first, see walkOnce
	idDepths     []int    // depth of each ids entry
	depthCounts  []int    // node count per depth, expanded or not
	walked       bool
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

// walkOnce computes expandable IDs (with their depths) and per-depth node counts in
// one depth-first pass over the whole tree, expanded or not.
func (t *dedupTree) walkOnce() {
	if t.walked {
		return
	}
	t.walked = true
	var walk func(nodes []treeflat.Node[*DedupRowData], depth int)
	walk = func(nodes []treeflat.Node[*DedupRowData], depth int) {
		if len(nodes) == 0 {
			return
		}
		for depth >= len(t.depthCounts) {
			t.depthCounts = append(t.depthCounts, 0)
		}
		t.depthCounts[depth] += len(nodes)
		for i := range nodes {
			if len(nodes[i].Children) > 0 {
				t.ids = append(t.ids, nodes[i].ID)
				t.idDepths = append(t.idDepths, depth)
				walk(nodes[i].Children, depth+1)
			}
		}
	}
	walk(t.roots, 0)
}

// expandableIDs lists the collapsible node IDs; callers must not modify the result.
func (t *dedupTree) expandableIDs() []string {
	t.walkOnce()
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
		t.roots = dedupDirRoots(snap, dedupIndexFor(snap, c).marks, func(yield func(int, comparepkg.DedupFile) bool) {
			for _, gi := range idxs {
				for _, f := range snap.Groups[gi].Files {
					if !yield(gi, f) {
						return
					}
				}
			}
		})
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

// dedupDirGroupsKey identifies a snapshot's DirGroups slice by its first element.
type dedupDirGroupsKey struct {
	first *comparepkg.DedupDirGroup
	n     int
}

// dedupIndex holds what the copies pane and tree look up per snapshot: the
// member-dir marker map (the single source for "which dir group is dirRel in",
// an exact group wins over a content group) and, built on first use, every
// file sorted by Rel so the files under a directory form one contiguous range.
type dedupIndex struct {
	groups    dedupGroupsKey
	dirGroups dedupDirGroupsKey
	marks     map[string]dedupDirMark // member dir rel -> marker; nil without dir groups
	files     []dedupFileRef          // sorted by rel; built by sortedFiles
	filesDone bool
}

type dedupFileRef struct {
	rel    string
	gi, fi int // snap.Groups[gi].Files[fi]
}

// dedupIndexFor returns the snapshot index, reusing the cached one while the
// Groups and DirGroups slices are the same; a nil cache builds a fresh one.
func dedupIndexFor(snap comparepkg.DedupSnapshot, c *DedupCache) *dedupIndex {
	key := dedupIndex{groups: dedupGroupsID(snap.Groups)}
	if len(snap.DirGroups) > 0 {
		key.dirGroups = dedupDirGroupsKey{&snap.DirGroups[0], len(snap.DirGroups)}
	}
	if c != nil && c.index != nil && c.index.groups == key.groups && c.index.dirGroups == key.dirGroups {
		return c.index
	}
	// ponytail: zero-size dir groups need no IgnoreEmpty check; their zero-byte files are hidden then, so no row exists.
	if len(snap.DirGroups) > 0 {
		key.marks = map[string]dedupDirMark{}
		for gi, g := range snap.DirGroups {
			for i, r := range g.Rels {
				if m, ok := key.marks[r]; !ok || (m.kind == comparepkg.DirContent && g.Kind == comparepkg.DirExact) {
					key.marks[r] = dedupDirMark{kind: g.Kind, hidden: g.Hidden[i], group: gi}
				}
			}
		}
	}
	idx := &key
	if c != nil {
		c.index = idx
	}
	return idx
}

// filesUnder yields every file (with its group index) inside any of the dirs,
// once each: a dir nested under another listed dir adds nothing, and "" is the
// whole snapshot. Only the matching ranges of the sorted file index are visited.
func (x *dedupIndex) filesUnder(snap comparepkg.DedupSnapshot, dirs []string) iter.Seq2[int, comparepkg.DedupFile] {
	if !x.filesDone {
		for gi, g := range snap.Groups {
			for fi, f := range g.Files {
				x.files = append(x.files, dedupFileRef{f.Rel, gi, fi})
			}
		}
		slices.SortFunc(x.files, func(a, b dedupFileRef) int { return strings.Compare(a.rel, b.rel) })
		x.filesDone = true
	}
	set := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		set[d] = true
	}
	return func(yield func(int, comparepkg.DedupFile) bool) {
		if set[""] {
			for _, r := range x.files {
				if !yield(r.gi, snap.Groups[r.gi].Files[r.fi]) {
					return
				}
			}
			return
		}
		for _, d := range dirs {
			if dedupHasListedAncestor(set, d) {
				continue
			}
			prefix := d + "/"
			i := sort.Search(len(x.files), func(i int) bool { return x.files[i].rel >= prefix })
			for ; i < len(x.files) && strings.HasPrefix(x.files[i].rel, prefix); i++ {
				r := x.files[i]
				if !yield(r.gi, snap.Groups[r.gi].Files[r.fi]) {
					return
				}
			}
		}
	}
}

// dedupHasListedAncestor reports whether a strict ancestor dir of rel is in set.
func dedupHasListedAncestor(set map[string]bool, rel string) bool {
	for {
		i := strings.LastIndexByte(rel, '/')
		if i < 0 {
			return false
		}
		if rel = rel[:i]; set[rel] {
			return true
		}
	}
}
