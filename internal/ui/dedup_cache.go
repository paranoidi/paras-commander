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
// mark/kept directory counters and the copies-pane column widths. DedupViewState.Cache is a
// pointer so value copies of the view share it; a nil Cache computes directly.
// Keys: the snapshot's Groups slice identity (&Groups[0] and len; the cache holds
// the pointer, so the address cannot be reused), TreeDirs, SortByWasted,
// IgnoreEmpty and DedupViewState.GroupFilterRev for the tree; the same slice
// identity plus DedupViewState.MarksRev for the mark counters (kept over a dir table
// keyed by the identity alone; DedupViewState.NoteMarkChange moves them for one file,
// any other MarksRev bump makes the next draw recount). Whoever assigns
// GroupFilter bumps GroupFilterRev, whoever changes Marked or Kept bumps MarksRev.
// The snapshot index (dedupIndex) is keyed by the Groups and DirGroups slice
// identities. Used from the UI goroutine only.
type DedupCache struct {
	tree      *dedupTree
	dirs      *dedupDirTable
	marks     *dedupMarkSets
	copyMarks *dedupCopyMarkSet
	widths    dedupWidths // copies pane
	index     *dedupIndex
	marked    dedupMarkedPaths
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

// dedupMarkSets are the snapshot-wide directory counters the tree rows are painted
// from, counted per dir id over the dir table: marked files below, kept files below
// and files below that belong to a fully-marked group (same definition as
// DedupGroupFullyMarked). A dir "has a marked file" when markedN > 0, "holds a
// fully-marked group's file" when dangerN > 0, "holds a kept file" when keptN > 0 and
// is "fully marked" when every file below it is marked. rev is the MarksRev the
// counters reflect; NoteMarkChange moves them one file at a time, otherwise a rev
// mismatch makes dedupMarkSetsFor recount.
type dedupMarkSets struct {
	groups      dedupGroupsKey
	rev         int
	t           *dedupDirTable
	markedN     []int32 // dir id -> marked files below
	keptN       []int32 // dir id -> kept files below
	dangerN     []int32 // dir id -> files below that belong to a fully-marked group
	groupMarked []int32 // group index -> marked members
}

func (m *dedupMarkSets) count(n []int32, rel string) int32 {
	if m == nil {
		return 0
	}
	if d, ok := m.t.ids[rel]; ok {
		return n[d]
	}
	return 0
}

// marked: dir holds a marked file.
func (m *dedupMarkSets) marked(rel string) bool { return m != nil && m.count(m.markedN, rel) > 0 }

// danger: dir holds a file of a fully-marked group.
func (m *dedupMarkSets) danger(rel string) bool { return m != nil && m.count(m.dangerN, rel) > 0 }

// kept: dir holds a kept file.
func (m *dedupMarkSets) kept(rel string) bool { return m != nil && m.count(m.keptN, rel) > 0 }

// full: every snapshot file below the dir is marked.
func (m *dedupMarkSets) full(rel string) bool {
	if m == nil {
		return false
	}
	d, ok := m.t.ids[rel]
	return ok && m.markedN[d] > 0 && m.markedN[d] == m.t.total[d]
}

// dedupDirTable numbers every ancestor dir of every snapshot file so the mark counters
// are kept over int ids instead of hashing each ancestor path per file.
type dedupDirTable struct {
	groups dedupGroupsKey
	rels   []string         // dir id -> rel
	ids    map[string]int32 // rel -> dir id
	parent []int32          // dir id -> parent dir id (always smaller), -1 for a top-level dir
	total  []int32          // dir id -> files below it
	leaf   []int32          // file's own dir id (-1: none), all groups' files concatenated
	start  []int            // leaf offset of each group's first file
	absIdx map[string]int32 // file abs -> index into leaf; built by locate on first use
}

// locate finds the group and in-group file index of the snapshot file at abs.
// ponytail: a path listed twice in the snapshot resolves to its last entry; groups
// hold distinct files, so a duplicate would be a snapshot bug.
func (t *dedupDirTable) locate(snap comparepkg.DedupSnapshot, abs string) (gi, fi int, ok bool) {
	if t.absIdx == nil {
		t.absIdx = make(map[string]int32, len(t.leaf))
		for g := range snap.Groups {
			for i, f := range snap.Groups[g].Files {
				t.absIdx[f.Abs.String()] = int32(t.start[g] + i)
			}
		}
	}
	flat, ok := t.absIdx[abs]
	if !ok {
		return 0, 0, false
	}
	gi = sort.Search(len(t.start), func(i int) bool { return t.start[i] > int(flat) }) - 1
	return gi, int(flat) - t.start[gi], true
}

// ponytail: same &Groups[0] and len means same files, true while snapshots are rebuilt
// wholesale and never edited in place; edit-in-place needs the table dropped.
func newDedupDirTable(snap comparepkg.DedupSnapshot) *dedupDirTable {
	t := &dedupDirTable{groups: dedupGroupsID(snap.Groups), start: make([]int, 0, len(snap.Groups)), ids: map[string]int32{}}
	var intern func(rel string) int32
	intern = func(rel string) int32 {
		if id, ok := t.ids[rel]; ok {
			return id
		}
		p := int32(-1)
		if i := strings.LastIndexByte(rel, '/'); i >= 0 {
			p = intern(rel[:i])
		}
		id := int32(len(t.rels))
		t.ids[rel] = id
		t.rels, t.parent, t.total = append(t.rels, rel), append(t.parent, p), append(t.total, 0)
		return id
	}
	for _, g := range snap.Groups {
		t.start = append(t.start, len(t.leaf))
		for _, f := range g.Files {
			d := int32(-1)
			if i := strings.LastIndexByte(f.Rel, '/'); i >= 0 {
				d = intern(f.Rel[:i])
			}
			t.leaf = append(t.leaf, d)
			for ; d >= 0; d = t.parent[d] {
				t.total[d]++
			}
		}
	}
	return t
}

// dirTableFor returns the cached dir table for snap or builds (and caches) one.
func (c *DedupCache) dirTableFor(snap comparepkg.DedupSnapshot) *dedupDirTable {
	id := dedupGroupsID(snap.Groups)
	if c != nil && c.dirs != nil && c.dirs.groups == id {
		return c.dirs
	}
	t := newDedupDirTable(snap)
	if c != nil {
		c.dirs = t
	}
	return t
}

// dedupMarkSetsFor returns the mark/kept dir counters, recounted (in one pass over every
// snapshot file, visible or not) only when the snapshot groups or MarksRev change
// without NoteMarkChange having kept them current.
func dedupMarkSetsFor(snap comparepkg.DedupSnapshot, view DedupViewState) *dedupMarkSets {
	id := dedupGroupsID(snap.Groups)
	c := view.Cache
	if c != nil && c.marks != nil && c.marks.groups == id && c.marks.rev == view.MarksRev {
		return c.marks
	}
	t := c.dirTableFor(snap)
	n := len(t.rels)
	m := &dedupMarkSets{groups: id, rev: view.MarksRev, t: t,
		markedN: make([]int32, n), keptN: make([]int32, n), dangerN: make([]int32, n), groupMarked: make([]int32, len(snap.Groups))}
	hasMarked, hasKept := len(view.Marked) > 0, len(view.Kept) > 0
	if hasMarked || hasKept {
		// Count at each file's own dir, then roll up once: parent ids are smaller
		// than their children's, so one reverse pass sums every subtree.
		for gi, g := range snap.Groups {
			leaf := t.leaf[t.start[gi]:]
			for fi, f := range g.Files {
				abs := f.Abs.String()
				if hasMarked && view.Marked[abs] {
					m.groupMarked[gi]++
					if d := leaf[fi]; d >= 0 {
						m.markedN[d]++
					}
				}
				if hasKept && view.Kept[abs] {
					if d := leaf[fi]; d >= 0 {
						m.keptN[d]++
					}
				}
			}
			if len(g.Files) > 0 && int(m.groupMarked[gi]) == len(g.Files) {
				for _, d := range leaf[:len(g.Files)] {
					if d >= 0 {
						m.dangerN[d]++
					}
				}
			}
		}
		for d := n - 1; d >= 0; d-- {
			if p := t.parent[d]; p >= 0 {
				m.markedN[p] += m.markedN[d]
				m.keptN[p] += m.keptN[d]
				m.dangerN[p] += m.dangerN[d]
			}
		}
	}
	if c != nil {
		c.marks = m
	}
	return m
}

// NoteMarkChange is the one place MarksRev is bumped for a single file: the caller
// has just flipped absKey in Marked (keep false) or Kept (keep true) to on. When the
// cached counters reflect the revision before this change they are moved by that one
// file and carry the new revision; otherwise they stay stale and the next draw
// recounts, so a writer that bumps MarksRev some other way is still correct.
func (s *DedupViewState) NoteMarkChange(snap comparepkg.DedupSnapshot, absKey string, keep, on bool) {
	old := s.MarksRev
	s.MarksRev++
	c := s.Cache
	if c == nil || c.marks == nil || c.marks.groups != dedupGroupsID(snap.Groups) || c.marks.rev != old {
		return
	}
	m := c.marks
	m.rev = s.MarksRev
	gi, fi, ok := m.t.locate(snap, absKey)
	if !ok {
		return // a file outside the snapshot sits under no dir and in no group
	}
	delta := int32(1)
	if !on {
		delta = -1
	}
	t := m.t
	leaf := t.leaf[t.start[gi]:]
	add := func(n []int32, fi int) {
		for d := leaf[fi]; d >= 0; d = t.parent[d] {
			n[d] += delta
		}
	}
	if keep {
		add(m.keptN, fi)
		return
	}
	size := int32(len(snap.Groups[gi].Files))
	wasFull := m.groupMarked[gi] == size
	m.groupMarked[gi] += delta
	add(m.markedN, fi)
	if isFull := m.groupMarked[gi] == size; isFull != wasFull {
		for i := range size {
			add(m.dangerN, int(i))
		}
	}
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

// sortFiles builds files, once.
func (x *dedupIndex) sortFiles(snap comparepkg.DedupSnapshot) {
	if x.filesDone {
		return
	}
	for gi, g := range snap.Groups {
		for fi, f := range g.Files {
			x.files = append(x.files, dedupFileRef{f.Rel, gi, fi})
		}
	}
	slices.SortFunc(x.files, func(a, b dedupFileRef) int { return strings.Compare(a.rel, b.rel) })
	x.filesDone = true
}

// lowerBound is the first position in files (sortFiles first) whose rel is >= rel.
func (x *dedupIndex) lowerBound(rel string) int {
	return sort.Search(len(x.files), func(i int) bool { return x.files[i].rel >= rel })
}

// filesUnder yields every file (with its group index) inside any of the dirs,
// once each: a dir nested under another listed dir adds nothing, and "" is the
// whole snapshot. Only the matching ranges of the sorted file index are visited.
func (x *dedupIndex) filesUnder(snap comparepkg.DedupSnapshot, dirs []string) iter.Seq2[int, comparepkg.DedupFile] {
	x.sortFiles(snap)
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
			i := x.lowerBound(prefix)
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
