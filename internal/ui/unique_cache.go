package ui

import (
	"slices"

	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// UniqueCache memoizes what only changes with the file list or the hidden toggle: the shown
// files, their unverified count, the built node tree (collapse state only affects the flatten)
// and the Size/Count column widths; plus the marked-files-per-directory counts, which change
// with the marks. UniqueViewState.Cache is a pointer so value copies of the view share it; a nil
// Cache computes directly. Keys: the Files slice identity (&Files[0] and len; the cache holds the
// pointer, so the address cannot be reused), Root and HideHidden for the files and tree;
// UniqueViewState.MarksRev for the marks. Whoever replaces Files assigns a new slice (never
// edits one in place), whoever changes Marked bumps MarksRev. Used from the UI goroutine only.
type UniqueCache struct {
	shown    *uniqueShown
	marks    map[string]int
	marksRev int
}

type uniqueShownKey struct {
	first      *UniqueFile
	n          int
	root       string
	hideHidden bool
}

// uniqueShown is the shown files for one key and what is derived from them, each part built on
// first use.
type uniqueShown struct {
	key        uniqueShownKey
	files      []UniqueFile
	unverified int
	roots      []treeflat.Node[*UniqueRowData]
	rootsDone  bool
	ids        []string
	idsDone    bool
	// sizeW and countW are the widest Size and Count texts over every node of the tree, not just
	// the visible rows, so the columns do not jump on expand or scroll.
	sizeW, countW int
	widthsDone    bool
}

// shown returns the shown files for s, reusing the cached ones on a key match.
func (s *UniqueViewState) shown() *uniqueShown {
	key := uniqueShownKey{n: len(s.Files), root: s.Root, hideHidden: s.HideHidden}
	if len(s.Files) > 0 {
		key.first = &s.Files[0]
	}
	if c := s.Cache; c != nil && c.shown != nil && c.shown.key == key {
		return c.shown
	}
	sh := &uniqueShown{key: key, files: s.Files}
	if s.HideHidden {
		sh.files = slices.DeleteFunc(slices.Clone(s.Files), func(f UniqueFile) bool {
			return uniqueIsHidden(uniqueRel(s.Root, f.Path))
		})
	}
	for _, f := range sh.files {
		if f.Unverified {
			sh.unverified++
		}
	}
	if s.Cache != nil {
		s.Cache.shown = sh
	}
	return sh
}

func (sh *uniqueShown) tree(s *UniqueViewState) []treeflat.Node[*UniqueRowData] {
	if !sh.rootsDone {
		sh.roots, sh.rootsDone = s.uniqueNodes(sh.files), true
	}
	return sh.roots
}

// uniqueColumnWidths are the widest Size and Count texts of the whole tree.
func (s *UniqueViewState) uniqueColumnWidths() (sizeW, countW int) {
	sh := s.shown()
	if !sh.widthsDone {
		treeflat.Walk(sh.tree(s), func(n *treeflat.Node[*UniqueRowData], _ int) {
			size, count := uniqueRowDetailTexts(n.Value)
			sh.sizeW, sh.countW = max(sh.sizeW, len([]rune(size))), max(sh.countW, len([]rune(count)))
		})
		sh.widthsDone = true
	}
	return sh.sizeW, sh.countW
}
