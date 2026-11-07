package previewpanel

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func styleCacheKey(s tcell.Style) uint64 {
	fg, bg, attrs := s.Decompose()
	h := uint64(attrs)
	h = h*31 + uint64(fg)
	h = h*31 + uint64(bg)
	return h
}

// wrapRow is one visual row: the flat cell range [start,end) it shows. cont marks a soft-wrapped
// continuation of the previous row, which draws behind a gutter indent when the body has one.
type wrapRow struct {
	start, end int
	cont       bool
}

// wrapLayout indexes a body into visual rows at one width without copying its cells: only the rows
// on screen are materialized, by appendRow. Building one is a single pass over the cells, so a
// reflow costs a few milliseconds on the largest preview. Immutable once built.
type wrapLayout struct {
	cells []AnsiCell
	rows  []wrapRow
	// lineRows holds the first row of each source line.
	lineRows []int
	width    int
	gutter   int
	source   Source
	// combinedText and styleKey identify an external body, which is parsed from text with base as
	// the default style. An internal body is identified by its cells alone.
	combinedText string
	styleKey     uint64
}

// cellWidth is the terminal width of r, with zero-width runes taking a cell of their own.
func cellWidth(r rune) int {
	if r < 0x7f {
		return 1 // covers the vast majority of cells; RuneWidth is a table search
	}
	return max(1, runewidth.RuneWidth(r))
}

// wrapRows breaks cells at '\n' and at width. A soft-wrapped row starts gutter cells in. A row always
// takes at least one cell, so a width narrower than the gutter still terminates. Without a gutter a
// trailing newline ends in an empty row; with one the formatter already emitted that row's number.
func wrapRows(cells []AnsiCell, width, gutter int) (rows []wrapRow, lineRows []int) {
	if width < 1 {
		width = 1
	}
	rows = make([]wrapRow, 0, len(cells)/max(width, 16)+1)
	lineRows = append(make([]int, 0, cap(rows)), 0)
	start, lineW := 0, 0
	cont := false
	for i := range cells {
		r := cells[i].R
		if r == '\n' {
			rows = append(rows, wrapRow{start, i, cont})
			lineRows = append(lineRows, len(rows))
			start, lineW, cont = i+1, 0, false
			continue
		}
		rw := cellWidth(r)
		if lineW+rw > width && i > start {
			rows = append(rows, wrapRow{start, i, cont})
			start, lineW, cont = i, gutter, gutter > 0
		}
		lineW += rw
	}
	if start < len(cells) || gutter == 0 || len(rows) == 0 {
		rows = append(rows, wrapRow{start, len(cells), cont})
	}
	return rows, lineRows
}

// WrapAnsiCells breaks cells into visual lines of at most width terminal cells. The lines alias cells.
func WrapAnsiCells(cells []AnsiCell, width int) [][]AnsiCell {
	rows, _ := wrapRows(cells, width, 0)
	lines := make([][]AnsiCell, len(rows))
	for i, r := range rows {
		lines[i] = cells[r.start:r.end:r.end]
	}
	return lines
}

func (l *wrapLayout) rowCount() int { return len(l.rows) }

// rowOffset maps row to the flat offset of its cell 0: on every row, cell i is at offset+i. A
// soft-wrapped row's indent cells have no flat counterpart, so its offset lands that many cells
// before its first real cell, and cells below gutter are not text.
func (l *wrapLayout) rowOffset(row int) int {
	r := l.rows[row]
	if r.cont {
		return r.start - l.gutter
	}
	return r.start
}

// rowAt returns the row showing flat offset off. A newline cell belongs to the row it ends.
func (l *wrapLayout) rowAt(off int) int {
	i := sort.Search(len(l.rows), func(i int) bool { return l.rows[i].start > off })
	return max(0, i-1)
}

// rowOfLine returns the first row of source line lineN, or the last row beyond the content.
func (l *wrapLayout) rowOfLine(lineN int) int {
	last := max(0, len(l.rows)-1)
	if lineN <= 0 {
		return 0
	}
	if lineN >= len(l.lineRows) {
		return last
	}
	return min(l.lineRows[lineN], last)
}

// appendRow appends row's cells to dst with search highlighting applied and, on a continuation
// row, the gutter indent in the style of the row's first text cell. search may be nil.
func (l *wrapLayout) appendRow(dst []AnsiCell, row int, search *SearchState) []AnsiCell {
	r := l.rows[row]
	indent := 0
	if r.cont {
		indent = l.gutter
	}
	base := len(dst)
	for range indent {
		dst = append(dst, AnsiCell{R: ' '})
	}
	dst = append(dst, l.cells[r.start:r.end]...)
	text := dst[base+indent:]
	if search != nil {
		search.overlay(text, r.start)
	}
	if indent > 0 {
		for i := base; i < base+indent; i++ {
			dst[i].St = text[0].St
		}
	}
	return dst
}

// bodyCellsRaw returns the flat pre-wrap body cell stream for st.Source. For
// SourceInternalHighlighted this is HighlightedCells BY REFERENCE (same backing array reused across
// frames): callers must not mutate it.
func (st *State) bodyCellsRaw(base tcell.Style) []AnsiCell {
	switch st.Source {
	case SourceInternalHighlighted:
		return st.HighlightedCells
	default:
		return AnsiStyledCells(st.CombinedText, base)
	}
}

func sameCells(a, b []AnsiCell) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// describes reports whether l indexes st's current body at this width. Cells are compared by
// identity: HighlightedCells is only ever replaced, never edited in place.
func (l *wrapLayout) describes(st *State, textWidth int) bool {
	if l == nil || l.width != textWidth || l.source != st.Source || l.gutter != st.wrapGutter() {
		return false
	}
	if st.Source == SourceInternalHighlighted {
		return sameCells(l.cells, st.HighlightedCells)
	}
	return l.combinedText == st.CombinedText
}

// cachedLayout returns st's layout when it fits textWidth, regardless of the style an external body
// was parsed with: row boundaries do not depend on it.
func (st *State) cachedLayout(textWidth int) *wrapLayout {
	if st.layout.describes(st, textWidth) {
		return st.layout
	}
	return nil
}

func (st *State) buildLayout(textWidth int, base tcell.Style) *wrapLayout {
	cells := st.bodyCellsRaw(base)
	gutter := st.wrapGutter()
	rows, lineRows := wrapRows(cells, textWidth, gutter)
	return &wrapLayout{
		cells: cells, rows: rows, lineRows: lineRows, width: textWidth, gutter: gutter,
		source: st.Source, combinedText: st.CombinedText, styleKey: styleCacheKey(base),
	}
}

// layoutFor returns a layout for drawing with base: st's own when it fits, else a fresh one. An
// internal body carries its styles in its cells, so only an external one is tied to base.
func (st *State) layoutFor(textWidth int, base tcell.Style) *wrapLayout {
	if textWidth < 1 {
		textWidth = 1
	}
	l := st.cachedLayout(textWidth)
	if l != nil && (st.Source == SourceInternalHighlighted || l.styleKey == styleCacheKey(base)) {
		return l
	}
	return st.buildLayout(textWidth, base)
}

// WrapCacheSnapshot adopts src's layout when it describes this state's body.
func (st *State) WrapCacheSnapshot(src State) {
	if src.layout != nil && src.layout.describes(st, src.layout.width) {
		st.layout = src.layout
	}
}

// CachedWrappedLineCount returns the row count when the layout cache matches textWidth.
func (st *State) CachedWrappedLineCount(textWidth int) (count int, ok bool) {
	if l := st.cachedLayout(textWidth); l != nil {
		return l.rowCount(), true
	}
	return 0, false
}

// EnsureWrapLayout builds the row index for textWidth unless a matching one is cached.
func (st *State) EnsureWrapLayout(textWidth int, base tcell.Style) {
	st.layout = st.layoutFor(textWidth, base)
}

// WrappedLineCount returns the number of wrapped preview rows.
func (st *State) WrappedLineCount(textWidth int, base tcell.Style) int {
	if textWidth < 1 {
		textWidth = 1
	}
	if count, ok := st.CachedWrappedLineCount(textWidth); ok {
		return count
	}
	st.EnsureWrapLayout(textWidth, base)
	return st.layout.rowCount()
}

// WrappedRows materializes every wrapped row, search highlighting included. For small bodies such as
// an image caption; a scrolling view paints rows on demand instead.
func (st *State) WrappedRows(textWidth int, base tcell.Style) [][]AnsiCell {
	st.EnsureWrapLayout(textWidth, base)
	rows := make([][]AnsiCell, st.layout.rowCount())
	for i := range rows {
		rows[i] = st.layout.appendRow(nil, i, &st.Search)
	}
	return rows
}

// ReflowsAtDraw reports whether a width change only needs a new layout. True for a finished Chroma
// body; a markdown body has wrap and tables baked in, and external output and archive listings are
// produced for one width.
func (st State) ReflowsAtDraw() bool {
	return st.Phase == PhaseDone && st.Source == SourceInternalHighlighted && !st.IsMarkdown &&
		st.ChromaStyle != "" && st.ErrorMsg == "" && st.ImagePayload == ""
}

// TotalLines returns wrapped line count for scroll metrics.
func TotalLines(st State, textWidth int, base tcell.Style) int {
	if textWidth < 1 {
		textWidth = 1
	}
	if !hasDrawableBody(st) {
		return 0
	}
	return st.WrappedLineCount(textWidth, base)
}

func hasDrawableBody(st State) bool {
	if st.Source == SourceInternalHighlighted && len(st.HighlightedCells) > 0 {
		return true
	}
	return strings.TrimSpace(st.CombinedText) != ""
}
