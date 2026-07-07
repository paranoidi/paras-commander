package ui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// DedupRowKind distinguishes markable file rows from directory nodes (dirs mode).
type DedupRowKind int

const (
	DedupRowFile DedupRowKind = iota
	DedupRowDir
)

// DedupRowData is the payload of one visible dedup tree row. In groups mode the
// group's first file doubles as the expandable group header row.
type DedupRowData struct {
	Kind        DedupRowKind
	File        comparepkg.DedupFile // file rows only
	AbsKey      string               // file rows: mark-map key
	DirRel      string               // dir rows: slash-separated rel path under scan root
	GroupIdx    int                  // file rows: index into snapshot Groups; -1 for dirs
	Size        int64                // file rows: group content size (mark accounting)
	Copies      int
	DupCount    int    // dir rows: duplicate files in the subtree
	WastedBytes int64  // dir rows: reclaimable bytes in the subtree (keep-one-survivor rule)
	ShowSize    bool   // paint the "size ×N" column on this row
	Display     string // row text (rel path in groups mode, base name in dirs mode)
}

// DedupRow is one visible row of the dedup results tree.
type DedupRow = treeflat.Row[DedupRowData]

// DedupPane is one tree pane's cursor/scroll/collapse state (main results pane
// and the copies pane each own one).
type DedupPane struct {
	Selected   int
	ListScroll int
	Collapsed  map[string]bool // collapsed node IDs (absent = expanded)
}

// DedupCollapsedSet builds a collapse map with every listed node ID collapsed.
func DedupCollapsedSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// SetCollapsed marks/unmarks a node ID as collapsed, allocating the map lazily.
func (p *DedupPane) SetCollapsed(id string, collapsed bool) {
	if collapsed {
		if p.Collapsed == nil {
			p.Collapsed = map[string]bool{}
		}
		p.Collapsed[id] = true
		return
	}
	delete(p.Collapsed, id)
}

// DedupCollapseNewIDs marks expandable IDs that appear only in newIDs as
// collapsed, preserving collapse/expand state for IDs that existed in prevIDs.
func DedupCollapseNewIDs(pane *DedupPane, prevIDs, newIDs []string) {
	prev := make(map[string]struct{}, len(prevIDs))
	for _, id := range prevIDs {
		prev[id] = struct{}{}
	}
	for _, id := range newIDs {
		if _, ok := prev[id]; ok {
			continue
		}
		pane.SetCollapsed(id, true)
	}
}

// DedupViewState tracks the two tree panes (main results + copies of the
// selected file), focus, marks, and display toggles for the find-duplicates
// screen.
type DedupViewState struct {
	Main                  DedupPane
	Copies                DedupPane
	FocusCopies           bool            // Tab focus: false = main pane, true = copies pane
	Marked                map[string]bool // absolute paths marked for deletion
	MarkedCount           int
	MarkedReclaimBytes    int64
	SortByWasted          bool // groups mode: false = order by path; true = most space wasted first
	IgnoreEmpty           bool // hide zero-byte duplicate groups (default true, set on Open)
	IgnoredEmptyCount     int  // files hidden by IgnoreEmpty, for the title
	TreeDirs              bool // true = directory hierarchy tree (default); false = duplicate groups tree
	DirsCollapsePending   bool // dirs mode: collapse all folders on first DedupDone list build
	GroupsCollapsePending bool // groups mode: collapse all group headers on first DedupDone list build
}

func dedupGroupID(g comparepkg.DedupGroup) string { return fmt.Sprintf("g:%x", g.Hash) }

// DedupActiveGroups returns the snapshot groups the view operates on, applying
// the ignore-empty filter. Mark helpers and the delete flow use this so hidden
// (collapsed) rows are never silently skipped.
func DedupActiveGroups(snap comparepkg.DedupSnapshot, ignoreEmpty bool) []comparepkg.DedupGroup {
	if !ignoreEmpty {
		return snap.Groups
	}
	out := make([]comparepkg.DedupGroup, 0, len(snap.Groups))
	for _, g := range snap.Groups {
		if g.Size != 0 {
			out = append(out, g)
		}
	}
	return out
}

// DedupRowsFromSnapshot builds the visible tree rows from a done-phase snapshot,
// honoring the view's tree mode, sort order, ignore-empty filter, and collapse
// state. Returns nil when the results list is not shown (walking, hashing, etc.).
// ignoredEmpty is the number of files dropped by the ignore-empty filter.
func DedupRowsFromSnapshot(snap comparepkg.DedupSnapshot, view DedupViewState) (rows []DedupRow, ignoredEmpty int) {
	if snap.Phase != comparepkg.DedupDone {
		return nil, 0
	}
	idxs := make([]int, 0, len(snap.Groups))
	for i, g := range snap.Groups {
		if view.IgnoreEmpty && g.Size == 0 {
			ignoredEmpty += len(g.Files)
			continue
		}
		idxs = append(idxs, i)
	}
	var roots []treeflat.Node[DedupRowData]
	if view.TreeDirs {
		roots = dedupDirRoots(snap, idxs, "")
	} else {
		roots = dedupGroupRoots(snap, idxs, view.SortByWasted)
	}
	return treeflat.Flatten(roots, dedupExpandedFn(view.Main.Collapsed)), ignoredEmpty
}

func dedupExpandedFn(collapsed map[string]bool) func(string) bool {
	return func(id string) bool { return !collapsed[id] }
}

// DedupCopyRows builds the copies-pane rows for the main pane's selected row:
// the directory tree of where the selected file's other copies live. Returns
// nil when the selection is not a file row (directory rows have no single
// duplicate group).
func DedupCopyRows(snap comparepkg.DedupSnapshot, sel DedupRow, collapsed map[string]bool) []DedupRow {
	roots := dedupCopyRoots(snap, sel)
	if roots == nil {
		return nil
	}
	return treeflat.Flatten(roots, dedupExpandedFn(collapsed))
}

// DedupCopyExpandableIDs lists collapsible node IDs of the copies pane (collapse-all).
func DedupCopyExpandableIDs(snap comparepkg.DedupSnapshot, sel DedupRow) []string {
	return treeflat.ExpandableIDs(dedupCopyRoots(snap, sel))
}

func dedupCopyRoots(snap comparepkg.DedupSnapshot, sel DedupRow) []treeflat.Node[DedupRowData] {
	d := sel.Value
	if d.Kind != DedupRowFile || d.GroupIdx < 0 || d.GroupIdx >= len(snap.Groups) {
		return nil
	}
	return dedupDirRoots(snap, []int{d.GroupIdx}, d.AbsKey)
}

// DedupExpandableIDs returns every collapsible node ID for the view's current
// tree mode, ignoring current collapse state (collapse-all).
func DedupExpandableIDs(snap comparepkg.DedupSnapshot, view DedupViewState) []string {
	if snap.Phase != comparepkg.DedupDone {
		return nil
	}
	idxs := make([]int, 0, len(snap.Groups))
	for i, g := range snap.Groups {
		if view.IgnoreEmpty && g.Size == 0 {
			continue
		}
		idxs = append(idxs, i)
	}
	if view.TreeDirs {
		return treeflat.ExpandableIDs(dedupDirRoots(snap, idxs, ""))
	}
	return treeflat.ExpandableIDs(dedupGroupRoots(snap, idxs, view.SortByWasted))
}

// dedupGroupRoots builds one root node per duplicate group; the group's first
// file is the header row and the remaining copies are its children.
func dedupGroupRoots(snap comparepkg.DedupSnapshot, idxs []int, sortByWasted bool) []treeflat.Node[DedupRowData] {
	idxs = slices.Clone(idxs)
	if sortByWasted {
		slices.SortFunc(idxs, func(a, b int) int {
			return comparepkg.DedupGroupBySize(snap.Groups[a], snap.Groups[b])
		})
	} else {
		slices.SortFunc(idxs, func(a, b int) int {
			return cmp.Compare(snap.Groups[a].Files[0].Rel, snap.Groups[b].Files[0].Rel)
		})
	}
	roots := make([]treeflat.Node[DedupRowData], 0, len(idxs))
	for _, gi := range idxs {
		g := snap.Groups[gi]
		root := treeflat.Node[DedupRowData]{
			ID: dedupGroupID(g),
			Value: DedupRowData{
				Kind:     DedupRowFile,
				File:     g.Files[0],
				AbsKey:   g.Files[0].Abs.String(),
				GroupIdx: gi,
				Size:     g.Size,
				Copies:   len(g.Files),
				ShowSize: true,
				Display:  g.Files[0].Rel,
			},
		}
		for _, f := range g.Files[1:] {
			root.Children = append(root.Children, treeflat.Node[DedupRowData]{
				ID: f.Abs.String(),
				Value: DedupRowData{
					Kind:     DedupRowFile,
					File:     f,
					AbsKey:   f.Abs.String(),
					GroupIdx: gi,
					Size:     g.Size,
					Copies:   len(g.Files),
					Display:  f.Rel,
				},
			})
		}
		roots = append(roots, root)
	}
	return roots
}

// dedupDirBuild is the intermediate mutable trie used to assemble the dirs-mode tree.
type dedupDirBuild struct {
	sub   map[string]*dedupDirBuild
	files []treeflat.Node[DedupRowData]
}

// dedupDirRoots builds the directory-hierarchy tree: only directories containing
// duplicate files appear; leaves are the duplicate files themselves. excludeAbs
// drops one file (the copies pane hides the selected file itself).
func dedupDirRoots(snap comparepkg.DedupSnapshot, idxs []int, excludeAbs string) []treeflat.Node[DedupRowData] {
	root := &dedupDirBuild{}
	for _, gi := range idxs {
		g := snap.Groups[gi]
		for _, f := range g.Files {
			if excludeAbs != "" && f.Abs.String() == excludeAbs {
				continue
			}
			parts := strings.Split(f.Rel, "/")
			cur := root
			for _, part := range parts[:len(parts)-1] {
				if cur.sub == nil {
					cur.sub = map[string]*dedupDirBuild{}
				}
				next := cur.sub[part]
				if next == nil {
					next = &dedupDirBuild{}
					cur.sub[part] = next
				}
				cur = next
			}
			cur.files = append(cur.files, treeflat.Node[DedupRowData]{
				ID: f.Abs.String(),
				Value: DedupRowData{
					Kind:     DedupRowFile,
					File:     f,
					AbsKey:   f.Abs.String(),
					GroupIdx: gi,
					Size:     g.Size,
					Copies:   len(g.Files),
					ShowSize: true,
					Display:  parts[len(parts)-1],
				},
			})
		}
	}
	nodes, _ := dedupDirNodes(root, "", snap)
	return nodes
}

// dedupDirNodes converts a build trie level to sorted tree nodes: directories
// first (by name), then files (by name). It also aggregates per-group copy
// counts bottom-up so every directory row carries its details column values:
// DupCount (duplicate files in the subtree) and WastedBytes (reclaimable bytes
// under the keep-one-survivor rule — all k copies count when the group also
// lives outside the subtree, k-1 when it lives entirely inside).
func dedupDirNodes(b *dedupDirBuild, rel string, snap comparepkg.DedupSnapshot) ([]treeflat.Node[DedupRowData], map[int]int) {
	names := make([]string, 0, len(b.sub))
	for name := range b.sub {
		names = append(names, name)
	}
	slices.Sort(names)
	// ponytail: per-level map merge is O(files×depth); fine for dedup result sizes.
	counts := map[int]int{}
	for _, f := range b.files {
		counts[f.Value.GroupIdx]++
	}
	out := make([]treeflat.Node[DedupRowData], 0, len(names)+len(b.files))
	for _, name := range names {
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		children, childCounts := dedupDirNodes(b.sub[name], childRel, snap)
		dupCount := 0
		var wasted int64
		for gi, k := range childCounts {
			counts[gi] += k
			dupCount += k
			g := snap.Groups[gi]
			if k == len(g.Files) {
				k--
			}
			wasted += int64(k) * g.Size
		}
		out = append(out, treeflat.Node[DedupRowData]{
			ID: "d:" + childRel,
			Value: DedupRowData{
				Kind:        DedupRowDir,
				DirRel:      childRel,
				GroupIdx:    -1,
				DupCount:    dupCount,
				WastedBytes: wasted,
				// No trailing slash: FitPathForWidth would strip it anyway, and the
				// expander glyph already marks the row as a directory.
				Display: name,
			},
			Children: children,
		})
	}
	files := slices.Clone(b.files)
	slices.SortFunc(files, func(a, b treeflat.Node[DedupRowData]) int {
		return cmp.Compare(a.Value.Display, b.Value.Display)
	})
	return append(out, files...), counts
}

// DedupRowIndexByID returns the row index with the given node ID, or -1.
func DedupRowIndexByID(rows []DedupRow, id string) int {
	if id == "" {
		return -1
	}
	for i, r := range rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// EnsureSelectionVisible clamps the pane's selected row and scroll offset.
func (p *DedupPane) EnsureSelectionVisible(total int, visibleRows int) {
	if total == 0 {
		p.Selected = 0
		p.ListScroll = 0
		return
	}
	if p.Selected >= total {
		p.Selected = total - 1
	}
	if p.Selected < 0 {
		p.Selected = 0
	}
	if visibleRows <= 0 {
		return
	}
	if p.Selected < p.ListScroll {
		p.ListScroll = p.Selected
	}
	if p.Selected >= p.ListScroll+visibleRows {
		p.ListScroll = p.Selected - visibleRows + 1
	}
	maxScroll := max(0, total-visibleRows)
	if p.ListScroll > maxScroll {
		p.ListScroll = maxScroll
	}
	if p.ListScroll < 0 {
		p.ListScroll = 0
	}
}

// DedupGroupFullyMarked reports whether every file in the group is marked for deletion.
func DedupGroupFullyMarked(g comparepkg.DedupGroup, marked map[string]bool) bool {
	for _, f := range g.Files {
		if !marked[f.Abs.String()] {
			return false
		}
	}
	return len(g.Files) > 0
}

// DedupRedundantUnder returns the AbsKeys of duplicate copies under dirAbs that
// can be deleted while leaving exactly one surviving copy of each content group
// ("keep uniques in this folder"). When a copy already survives outside the
// directory, every under-directory copy is redundant; otherwise the first
// under-directory copy is kept.
func DedupRedundantUnder(groups []comparepkg.DedupGroup, dirAbs string) []string {
	return dedupMarkUnder(groups, dirAbs, true)
}

// DedupDuplicatesUnder returns the AbsKeys of copies under dirAbs that are also
// stored outside it ("delete duplicates from this folder"). A group living
// entirely under the directory is left untouched — nothing here is dropped
// unless a copy survives elsewhere.
func DedupDuplicatesUnder(groups []comparepkg.DedupGroup, dirAbs string) []string {
	return dedupMarkUnder(groups, dirAbs, false)
}

// dedupMarkUnder collects under-directory copies to mark for deletion. When
// keepOneWhenIsolated is true a group living entirely under the directory keeps
// its first copy; when false such a group is skipped entirely.
func dedupMarkUnder(groups []comparepkg.DedupGroup, dirAbs string, keepOneWhenIsolated bool) []string {
	if dirAbs == "" {
		return nil
	}
	dir := dirAbs + "/"
	var out []string
	for _, g := range groups {
		var under []string
		outside := 0
		for _, f := range g.Files {
			abs := f.Abs.String()
			// Trailing slash on both sides keeps "/a/b" from matching "/a/bc".
			if strings.HasPrefix(abs+"/", dir) {
				under = append(under, abs)
			} else {
				outside++
			}
		}
		if outside == 0 {
			if keepOneWhenIsolated && len(under) > 0 {
				under = under[1:] // keep the first copy alive
			} else {
				under = nil // no external survivor: leave the group untouched
			}
		}
		out = append(out, under...)
	}
	return out
}

// MarkedSummary returns the end-label text for marked files, or "" when none.
func (s DedupViewState) MarkedSummary() string {
	if s.MarkedCount == 0 {
		return ""
	}
	return fmt.Sprintf("%d marked · %s", s.MarkedCount, formatJobBytes(s.MarkedReclaimBytes))
}
