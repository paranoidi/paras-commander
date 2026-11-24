package ui

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// UniqueFile is one file of the Find unique files results.
type UniqueFile struct {
	Path string
	Size int64
	// Unverified: its copies cannot be ruled out yet (not fully hashed, or a potential duplicate
	// outside the scope).
	Unverified bool
}

// UniqueViewState is the Find unique files view: the results tree in the active panel's place and
// the inactive panel beside it, the destination of copy and move. It stays in the model after
// leaving the view, so the action that opened it returns to it.
type UniqueViewState struct {
	// Loaded: Files holds a finished lookup. Loading: a lookup is running (a first one, or a
	// refresh with the previous Files still shown).
	Loaded, Loading bool
	// Root is the panel directory as the index records it and Scope the directories checked:
	// the selected directories, else Root.
	Root  string
	Scope []string
	Files []UniqueFile
	// ToHash are the files whose full hash would settle the Unverified flags.
	ToHash []string
	Pane   DedupPane
	// PrevExpandable are the directory IDs of the last rows build: only IDs new to it start
	// collapsed, so a refresh keeps what was open.
	PrevExpandable []string
	// FocusPanel: the destination panel has focus instead of the tree.
	FocusPanel bool
	MarkSet
	// RangeAnchor is the file marked as the pending range-select start ("" = none).
	RangeAnchor string
	// HideHidden leaves out files with a dot-prefixed path component below Root.
	HideHidden bool
	// MarksRev keys the cached marked-files-per-directory counts: whoever changes Marked bumps
	// MarksRev (SetMark does; so do Handler.keepMarksOf and Handler.ClearMarks).
	MarksRev int
	// Cache memoizes the shown files, the node tree and the column widths (see UniqueCache). A
	// pointer so value copies of the view share it; nil computes directly.
	Cache *UniqueCache
}

// MarkSet is the marked paths of a list view and their total size.
type MarkSet struct {
	// Marked holds the marked paths; MarkedBytes their total size.
	Marked      map[string]bool
	MarkedBytes int64
}

// SetMark marks or unmarks one file, keeping MarkedBytes in step; it reports whether the set
// changed.
func (m *MarkSet) SetMark(f UniqueFile, on bool) bool {
	if m.Marked[f.Path] == on {
		return false
	}
	if on {
		if m.Marked == nil {
			m.Marked = map[string]bool{}
		}
		m.Marked[f.Path] = true
		m.MarkedBytes += f.Size
		return true
	}
	delete(m.Marked, f.Path)
	m.MarkedBytes -= f.Size
	return true
}

// UniqueRowData is the payload of one tree row.
type UniqueRowData struct {
	IsDir bool
	// Path is the file's path, or the deepest directory of a combined directory row.
	Path string
	// Display is the row text: a base name, or a/b for a combined directory row.
	Display string
	// Size and Count are the file's size and 1, or the totals of the directory's subtree.
	Size  int64
	Count int
	// Unverified is how many files of the row (the subtree of a directory) are flagged.
	Unverified int
}

// UniqueRow is one visible row of the results tree.
type UniqueRow = treeflat.Row[*UniqueRowData]

// RelPath is a listed path as the view names it, below Root (see uniqueRel).
func (s *UniqueViewState) RelPath(path string) string { return uniqueRel(s.Root, path) }

// uniqueRel is path below Root, or below the filesystem root for a selected directory off Root (a
// selection can outlive the directory it was made in).
func uniqueRel(root, path string) string {
	if rel := strings.TrimPrefix(path, strings.TrimSuffix(root, "/")+"/"); rel != path {
		return rel
	}
	return strings.TrimPrefix(path, "/")
}

func uniqueIsHidden(rel string) bool {
	return slices.ContainsFunc(strings.Split(rel, "/"), func(p string) bool { return strings.HasPrefix(p, ".") })
}

// VisibleFiles is every file the tree shows, whatever is collapsed. Marks and operations go
// through it, never through the visible rows, so files in collapsed directories count. The
// result is shared: callers must not modify it.
func (s *UniqueViewState) VisibleFiles() []UniqueFile { return s.shown().files }

// FilesUnder is the shown files below the directory dir.
func (s *UniqueViewState) FilesUnder(dir string) []UniqueFile {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	var out []UniqueFile
	for _, f := range s.VisibleFiles() {
		if strings.HasPrefix(f.Path, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// MarkedFiles is the marked files in path order.
func (s *UniqueViewState) MarkedFiles() []UniqueFile {
	var out []UniqueFile
	for _, f := range s.VisibleFiles() {
		if s.Marked[f.Path] {
			out = append(out, f)
		}
	}
	return out
}

// UnverifiedCount is how many shown files are flagged.
func (s *UniqueViewState) UnverifiedCount() int { return s.shown().unverified }

// SetMark marks or unmarks one file, keeping MarkedBytes in step and bumping MarksRev when the
// set changed.
func (s *UniqueViewState) SetMark(f UniqueFile, on bool) {
	if s.MarkSet.SetMark(f, on) {
		s.MarksRev++
	}
}

// markedUnderDirs counts the marked files below each directory path; the result is shared (and
// recomputed only when MarksRev changes), callers must not modify it.
func (s *UniqueViewState) markedUnderDirs() map[string]int {
	c := s.Cache
	if c != nil && c.marks != nil && c.marksRev == s.MarksRev {
		return c.marks
	}
	out := map[string]int{}
	stop := strings.TrimSuffix(s.Root, "/")
	for p, on := range s.Marked {
		if !on {
			continue
		}
		for dir := pathloc.Dir(p); dir != stop && dir != "/" && dir != "."; dir = pathloc.Dir(dir) {
			out[dir]++
		}
	}
	if c != nil {
		c.marks, c.marksRev = out, s.MarksRev
	}
	return out
}

// uniqueDirNode is a directory while the tree is built.
type uniqueDirNode struct {
	path  string
	dirs  map[string]*uniqueDirNode
	files []UniqueFile
	data  UniqueRowData
}

func (n *uniqueDirNode) add(f UniqueFile) {
	n.data.Count++
	n.data.Size += f.Size
	if f.Unverified {
		n.data.Unverified++
	}
}

func uniqueFileNode(f UniqueFile) treeflat.Node[*UniqueRowData] {
	data := UniqueRowData{Path: f.Path, Display: filepath.Base(f.Path), Size: f.Size, Count: 1}
	if f.Unverified {
		data.Unverified = 1
	}
	return treeflat.Node[*UniqueRowData]{ID: f.Path, Value: &data}
}

// nodes lists n's children: directories, then files, each by name. A directory holding only one
// directory folds into it (Display a/b), as the duplicates tree does.
func (n *uniqueDirNode) nodes() []treeflat.Node[*UniqueRowData] {
	names := make([]string, 0, len(n.dirs))
	for name := range n.dirs {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]treeflat.Node[*UniqueRowData], 0, len(names)+len(n.files))
	for _, name := range names {
		d, display := n.dirs[name], name
		for len(d.files) == 0 && len(d.dirs) == 1 {
			for childName, child := range d.dirs {
				d, display = child, display+"/"+childName
			}
		}
		data := d.data
		data.IsDir, data.Path, data.Display = true, d.path, display
		out = append(out, treeflat.Node[*UniqueRowData]{ID: d.path, Value: &data, Children: d.nodes()})
	}
	slices.SortFunc(n.files, func(a, b UniqueFile) int { return cmp.Compare(a.Path, b.Path) })
	for _, f := range n.files {
		out = append(out, uniqueFileNode(f))
	}
	return out
}

// uniqueNodes builds the tree of the shown files.
func (s *UniqueViewState) uniqueNodes(files []UniqueFile) []treeflat.Node[*UniqueRowData] {
	root := &uniqueDirNode{}
	for _, f := range files {
		rel := uniqueRel(s.Root, f.Path)
		parent := strings.TrimSuffix(strings.TrimSuffix(f.Path, rel), "/")
		parts := strings.Split(rel, "/")
		cur := root
		for _, part := range parts[:len(parts)-1] {
			parent += "/" + part
			next := cur.dirs[part]
			if next == nil {
				next = &uniqueDirNode{path: parent}
				if cur.dirs == nil {
					cur.dirs = map[string]*uniqueDirNode{}
				}
				cur.dirs[part] = next
			}
			next.add(f)
			cur = next
		}
		cur.files = append(cur.files, f)
	}
	return root.nodes()
}

// UniqueExpandableIDs lists every directory row ID of the tree; the result is shared, callers
// must not modify it.
func UniqueExpandableIDs(s *UniqueViewState) []string {
	sh := s.shown()
	if !sh.idsDone {
		sh.ids, sh.idsDone = treeflat.ExpandableIDs(sh.tree(s)), true
	}
	return sh.ids
}

// UniqueRows flattens the tree against the pane's collapsed set.
func UniqueRows(s *UniqueViewState) []UniqueRow {
	return treeflat.Flatten(s.shown().tree(s), func(id string) bool { return !s.Pane.Collapsed[id] })
}

// UniqueRects are the tree's and the destination panel's rects: the tree takes the active panel's
// place, so the destination stays where that panel is in the browser. Single source for drawing,
// visible-row counts and hit tests.
func UniqueRects(layout Layout, activePanel int) (tree, dest Rect) {
	if activePanel == SecondaryPanel {
		return layout.Secondary, layout.Primary
	}
	return layout.Primary, layout.Secondary
}
