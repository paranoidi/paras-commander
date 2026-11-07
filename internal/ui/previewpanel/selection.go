package previewpanel

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"

	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

// Selection is a mouse text selection as cell offsets into the flat pre-wrap body (same indexing
// as SearchMatch), so it stays on the same text while scrolling. The zero value selects nothing.
type Selection struct {
	Anchor, Head int
	Dragging     bool
	// Word marks a double-clicked word, which counts as selected even when it is one cell long.
	Word bool
}

// Range is the selected half-open offset range, including the cells under both ends. ok is false
// until the pointer has left the anchor cell, so a plain click selects nothing; a Word selection
// is always ok.
func (s Selection) Range() (lo, hi int, ok bool) {
	switch {
	case s.Anchor == s.Head && !s.Word:
		return 0, 0, false
	case s.Anchor <= s.Head:
		return s.Anchor, s.Head + 1, true
	}
	return s.Head, s.Anchor + 1, true
}

// ClearSelection drops the selection; its offsets mean nothing once the body is replaced.
func (st *State) ClearSelection() { st.Selection = Selection{} }

// TextSelectable reports whether st paints a text body the mouse can select from.
func (st State) TextSelectable() bool {
	return st.Open && st.Phase == PhaseDone && st.ImagePayload == "" &&
		strings.TrimSpace(st.ErrorMsg) == "" && hasDrawableBody(st)
}

// wrapGutter is the gutter width the wrapper applies to st's body. Line numbers an external
// command prints itself are plain text and do not count.
func (st State) wrapGutter() int {
	if st.Source == SourceInternalHighlighted {
		return st.GutterWidth
	}
	return 0
}

// CellOffsetAt returns the flat offset of the text cell at wrapped row and screen column, for a
// body wrapped at textWidth. Row is clamped to the body, a column inside the gutter snaps to the
// first text cell, and one past the row's end gives its newline (or last cell on a soft-wrapped row).
func (st State) CellOffsetAt(row, col, textWidth int, base tcell.Style) int {
	l := st.layoutFor(textWidth, base)
	if len(l.cells) == 0 || l.rowCount() == 0 {
		return 0
	}
	row = max(0, min(row, l.rowCount()-1))
	gutter := st.wrapGutter()
	start := l.rowOffset(row)
	x := 0
	cells := l.appendRow(nil, row, nil)
	for i, c := range cells {
		x += cellWidth(c.R)
		if i >= gutter && col < x {
			return start + i
		}
	}
	end := start + len(cells)
	if end < len(l.cells) && l.cells[end].R == '\n' {
		return end
	}
	return max(0, end-1)
}

// WordSelectionAt returns the selection of the word at wrapped row and screen column, with the
// word runes of the line editor. A column on the gutter, a blank or punctuation selects nothing.
// The word is whole across a soft wrap because the walk runs over the flat cells.
func (st State) WordSelectionAt(row, col, textWidth int, base tcell.Style) Selection {
	if col < st.wrapGutter() {
		return Selection{}
	}
	cells := st.bodyCellsRaw(base)
	off := st.CellOffsetAt(row, col, textWidth, base)
	if off >= len(cells) || !lineedit.IsWordRune(cells[off].R) {
		return Selection{}
	}
	lo, hi := off, off
	for lo > 0 && lineedit.IsWordRune(cells[lo-1].R) {
		lo--
	}
	for hi+1 < len(cells) && lineedit.IsWordRune(cells[hi+1].R) {
		hi++
	}
	return Selection{Anchor: lo, Head: hi, Word: true}
}

// SelectedText returns the selected text with its hard line breaks and without gutter cells; soft
// wraps do not add line breaks.
func (st State) SelectedText() string {
	lo, hi, ok := st.Selection.Range()
	if !ok {
		return ""
	}
	cells := st.bodyCellsRaw(tcell.StyleDefault) // style is irrelevant; only .R is read
	lo, hi = min(lo, len(cells)), min(hi, len(cells))
	gutter := st.wrapGutter()
	col := 0
	for i := lo - 1; i >= 0 && cells[i].R != '\n'; i-- {
		col++
	}
	var b strings.Builder
	for _, c := range cells[lo:hi] {
		if c.R == '\n' {
			b.WriteByte('\n')
			col = 0
			continue
		}
		if col >= gutter {
			b.WriteRune(c.R)
		}
		col++
	}
	return b.String()
}

// paintSelection repaints the selected text cells of one drawn row in reverse video. It paints over
// DrawCells so the row's trailing fill keeps its own style. start is the row's wrapLayout.rowOffset.
func paintSelection(screen tcell.Screen, x, y, maxW int, line []AnsiCell, start, gutter, lo, hi int) {
	col := 0
	for i, c := range line {
		rw := max(1, runewidth.RuneWidth(c.R))
		if col+rw > maxW {
			return
		}
		if off := start + i; i >= gutter && off >= lo && off < hi {
			screen.SetContent(x+col, y, c.R, nil, c.St.Reverse(true))
		}
		col += rw
	}
}
