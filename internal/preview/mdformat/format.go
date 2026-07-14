// Package mdformat renders markdown source to styled terminal cells for the
// preview panel, mirroring the shape of chromaformat.Highlight (parse once,
// walk the AST, emit previewpanel.AnsiCell runs). Styling reuses the active
// Chroma style's generic token types (GenericHeading, GenericStrong, ...) so
// the existing preview style picker (F9) restyles markdown output too.
package mdformat

import (
	"strconv"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// tableMarkdown builds the goldmark instance with GFM table support once; the
// parser is safe for concurrent Parse calls.
var tableMarkdown = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.Table))
})

// Options configures markdown rendering for a single file body.
type Options struct {
	Path         string
	StyleName    string
	BaseStyle    tcell.Style
	ContentWidth int
}

// Result is rendered terminal cells, same shape as chromaformat.Result.
type Result struct {
	Cells []previewpanel.AnsiCell
}

// Render parses source with a CommonMark parser plus the GFM table extension
// and walks the AST into styled, word-wrapped terminal cells.
func Render(source string, opts Options) Result {
	width := opts.ContentWidth
	if width < 1 {
		width = 1
	}
	src := []byte(source)
	doc := tableMarkdown().Parser().Parse(text.NewReader(src))
	r := &renderer{
		source:    src,
		style:     chromastyles.Get(opts.StyleName),
		base:      opts.BaseStyle,
		styleName: opts.StyleName,
		width:     width,
	}
	// textBase carries the Chroma style's Text-token colours (including its
	// background, which many styles set for every token) so plain prose
	// matches the uniform background raw Chroma highlighting of the same
	// file would produce; using r.base directly here would instead leave
	// plain text on the panel's own background while headings/tables/etc.
	// (styled via styleFor) pick up the syntax style's background, causing
	// a two-tone patchwork.
	r.textBase = r.styleFor(chroma.Text)
	r.renderSiblings(doc, nil, nil)
	return Result{Cells: r.out}
}

type renderer struct {
	source    []byte
	style     *chroma.Style
	base      tcell.Style
	textBase  tcell.Style
	styleName string
	width     int
	out       []previewpanel.AnsiCell
}

func (r *renderer) styleFor(tt chroma.TokenType) tcell.Style {
	return chromaformat.StyleEntryToTcell(r.base, r.style.Get(tt))
}

func (r *renderer) runes(s string, st tcell.Style) []previewpanel.AnsiCell {
	cells := make([]previewpanel.AnsiCell, 0, len(s))
	for _, rn := range s {
		cells = append(cells, previewpanel.AnsiCell{R: rn, St: st})
	}
	return cells
}

func (r *renderer) writeLine(cells []previewpanel.AnsiCell) {
	r.out = append(r.out, cells...)
	r.out = append(r.out, previewpanel.AnsiCell{R: '\n', St: r.textBase})
}

func (r *renderer) blank() {
	r.out = append(r.out, previewpanel.AnsiCell{R: '\n', St: r.textBase})
}

// renderSiblings renders every block child of parent, separated by a blank line.
func (r *renderer) renderSiblings(parent ast.Node, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	first := true
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		if !first {
			r.blank()
		}
		first = false
		r.renderBlock(c, firstPrefix, contPrefix)
	}
}

// renderIndentedChildren renders every block child of parent with the given
// prefixes, but without a blank separator line before the first child (the
// blank line, if any, was already emitted by the caller) and with a
// prefix-only blank line between children (so e.g. a blockquote's `▌` mark
// carries through blank lines inside it).
func (r *renderer) renderIndentedChildren(parent ast.Node, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	first := true
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		// A nested list attaches directly under the preceding block (no blank
		// separator row), matching how a nested list reads under its parent item.
		if _, nested := c.(*ast.List); !first && !nested {
			r.writeLine(append([]previewpanel.AnsiCell{}, contPrefix...))
		}
		fp := contPrefix
		if first {
			fp = firstPrefix
		}
		first = false
		r.renderBlock(c, fp, contPrefix)
	}
}

func (r *renderer) renderBlock(n ast.Node, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	switch v := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		r.emitWrapped(r.inline(n, r.textBase), firstPrefix, contPrefix)
	case *ast.Heading:
		st := r.styleFor(chroma.GenericHeading).Bold(true)
		marker := r.runes(strings.Repeat("#", v.Level)+" ", st)
		fp := append(append([]previewpanel.AnsiCell{}, firstPrefix...), marker...)
		cp := append(append([]previewpanel.AnsiCell{}, contPrefix...), r.runes(strings.Repeat(" ", len(marker)), st)...)
		r.emitWrapped(r.inline(n, st), fp, cp)
	case *ast.ThematicBreak:
		st := r.styleFor(chroma.Comment)
		w := r.width - len(firstPrefix)
		if w < 1 {
			w = 1
		}
		cells := append(append([]previewpanel.AnsiCell{}, firstPrefix...), r.runes(strings.Repeat("─", w), st)...)
		r.writeLine(cells)
	case *ast.Blockquote:
		st := r.styleFor(chroma.Comment)
		marker := r.runes("▌ ", st)
		fp := append(append([]previewpanel.AnsiCell{}, firstPrefix...), marker...)
		cp := append(append([]previewpanel.AnsiCell{}, contPrefix...), marker...)
		r.renderIndentedChildren(n, fp, cp)
	case *ast.CodeBlock:
		r.renderCode(string(v.Lines().Value(r.source)), "", firstPrefix, contPrefix)
	case *ast.FencedCodeBlock:
		r.renderCode(string(v.Lines().Value(r.source)), string(v.Language(r.source)), firstPrefix, contPrefix)
	case *ast.HTMLBlock:
		r.renderRawLines(string(v.Lines().Value(r.source)), firstPrefix, contPrefix)
	case *ast.List:
		r.renderList(v, firstPrefix, contPrefix)
	case *east.Table:
		r.renderTable(v, firstPrefix, contPrefix)
	default:
		r.emitWrapped(r.inline(n, r.textBase), firstPrefix, contPrefix)
	}
}

func (r *renderer) renderList(l *ast.List, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	idx := l.Start
	if idx == 0 {
		idx = 1
	}
	first := true
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		if !first && !l.IsTight {
			r.writeLine(append([]previewpanel.AnsiCell{}, contPrefix...))
		}
		marker := "• "
		if l.IsOrdered() {
			marker = strconv.Itoa(idx) + ". "
			idx++
		}
		markerCells := r.runes(marker, r.textBase)
		pad := r.runes(strings.Repeat(" ", len([]rune(marker))), r.textBase)
		base := contPrefix
		if first {
			base = firstPrefix
		}
		itemFirst := append(append([]previewpanel.AnsiCell{}, base...), markerCells...)
		itemCont := append(append([]previewpanel.AnsiCell{}, contPrefix...), pad...)
		r.renderIndentedChildren(item, itemFirst, itemCont)
		first = false
	}
}

// renderTable renders a GFM table glow-style: aligned columns separated by
// "│", a "─┼─" rule under the header, and cells word-wrapped within their
// column when the table is wider than the available width. Alignment
// (left/center/right) from the column's delimiter row is honored.
func (r *renderer) renderTable(t *east.Table, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	header, ok := t.FirstChild().(*east.TableHeader)
	if !ok {
		return
	}
	headerStyle := r.styleFor(chroma.GenericHeading).Bold(true)
	headerCells := r.tableRowCells(header, headerStyle)
	numCols := len(headerCells)
	if numCols == 0 {
		return
	}
	var bodyRows [][][]previewpanel.AnsiCell
	for row := header.NextSibling(); row != nil; row = row.NextSibling() {
		tr, ok := row.(*east.TableRow)
		if !ok {
			continue
		}
		bodyRows = append(bodyRows, r.tableRowCells(tr, r.textBase))
	}
	widths := r.tableColumnWidths(numCols, len(contPrefix), headerCells, bodyRows)

	first := true
	emit := func(lines [][]previewpanel.AnsiCell) {
		for _, ln := range lines {
			prefix := contPrefix
			if first {
				prefix = firstPrefix
			}
			first = false
			r.writeLine(append(append([]previewpanel.AnsiCell{}, prefix...), ln...))
		}
	}
	emit(r.tableRowLines(headerCells, widths, t.Alignments, headerStyle))
	emit([][]previewpanel.AnsiCell{r.tableRuleLine(widths)})
	for _, row := range bodyRows {
		emit(r.tableRowLines(row, widths, t.Alignments, r.textBase))
	}
}

// tableRowCells renders each cell of a table row (or header) to inline cells,
// one entry per column.
func (r *renderer) tableRowCells(row ast.Node, style tcell.Style) [][]previewpanel.AnsiCell {
	var cells [][]previewpanel.AnsiCell
	for c := row.FirstChild(); c != nil; c = c.NextSibling() {
		cells = append(cells, r.inline(c, style))
	}
	return cells
}

// tableColumnWidths computes the natural (widest-cell) width per column, then
// shrinks the widest column(s) one rune at a time until the row fits the
// available width. minColWidth caps how far a column can shrink.
func (r *renderer) tableColumnWidths(numCols, contPrefixLen int, headerCells [][]previewpanel.AnsiCell, bodyRows [][][]previewpanel.AnsiCell) []int {
	const minColWidth = 5
	widths := make([]int, numCols)
	measure := func(cells [][]previewpanel.AnsiCell) {
		for i := 0; i < numCols && i < len(cells); i++ {
			if w := len(cells[i]); w > widths[i] {
				widths[i] = w
			}
		}
	}
	measure(headerCells)
	for _, row := range bodyRows {
		measure(row)
	}
	for i := range widths {
		if widths[i] == 0 {
			widths[i] = 1
		}
	}

	avail := r.width - contPrefixLen
	if avail < 1 {
		avail = 1
	}
	// Non-content overhead: " " + cell + " " per column, "│" between columns.
	overhead := 3*numCols - 1
	target := avail - overhead

	// ponytail: naive shrink-the-widest loop instead of proportional
	// distribution; fine for the handful of columns a preview table has.
	sum := func() int {
		s := 0
		for _, w := range widths {
			s += w
		}
		return s
	}
	for target > 0 && sum() > target {
		maxI := 0
		for i, w := range widths {
			if w > widths[maxI] {
				maxI = i
			}
		}
		if widths[maxI] <= minColWidth {
			break
		}
		widths[maxI]--
	}
	return widths
}

// tableRowLines renders one logical table row (header or body) into physical
// output lines, word-wrapping each cell to its column width and padding
// shorter cells with blank lines so the row stays rectangular.
func (r *renderer) tableRowLines(cells [][]previewpanel.AnsiCell, widths []int, aligns []east.Alignment, style tcell.Style) [][]previewpanel.AnsiCell {
	numCols := len(widths)
	wrapped := make([][][]previewpanel.AnsiCell, numCols)
	maxLines := 1
	for i := 0; i < numCols; i++ {
		var content []previewpanel.AnsiCell
		if i < len(cells) {
			content = cells[i]
		}
		lines := wrapCells(content, widths[i])
		wrapped[i] = lines
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	sepStyle := r.styleFor(chroma.Comment)
	out := make([][]previewpanel.AnsiCell, maxLines)
	for ln := 0; ln < maxLines; ln++ {
		var line []previewpanel.AnsiCell
		for i := 0; i < numCols; i++ {
			if i > 0 {
				line = append(line, previewpanel.AnsiCell{R: '│', St: sepStyle})
			}
			var cellLine []previewpanel.AnsiCell
			if ln < len(wrapped[i]) {
				cellLine = wrapped[i][ln]
			}
			line = append(line, previewpanel.AnsiCell{R: ' ', St: style})
			line = append(line, r.padCell(cellLine, widths[i], alignOf(aligns, i), style)...)
			line = append(line, previewpanel.AnsiCell{R: ' ', St: style})
		}
		out[ln] = line
	}
	return out
}

// tableRuleLine draws the "─┼─" rule between the header and body rows.
func (r *renderer) tableRuleLine(widths []int) []previewpanel.AnsiCell {
	st := r.styleFor(chroma.Comment)
	var line []previewpanel.AnsiCell
	for i, w := range widths {
		if i > 0 {
			line = append(line, previewpanel.AnsiCell{R: '┼', St: st})
		}
		line = append(line, r.runes(strings.Repeat("─", w+2), st)...)
	}
	return line
}

// padCell pads cells (already wrapped to at most width runes) up to width
// according to align, filling with style-colored spaces.
func (r *renderer) padCell(cells []previewpanel.AnsiCell, width int, align east.Alignment, style tcell.Style) []previewpanel.AnsiCell {
	if len(cells) > width {
		cells = cells[:width]
	}
	pad := width - len(cells)
	switch align {
	case east.AlignRight:
		return append(r.runes(strings.Repeat(" ", pad), style), cells...)
	case east.AlignCenter:
		left := pad / 2
		out := r.runes(strings.Repeat(" ", left), style)
		out = append(out, cells...)
		out = append(out, r.runes(strings.Repeat(" ", pad-left), style)...)
		return out
	default: // AlignLeft, AlignNone
		return append(append([]previewpanel.AnsiCell{}, cells...), r.runes(strings.Repeat(" ", pad), style)...)
	}
}

// alignOf returns the alignment for column i, defaulting to AlignNone (left)
// when the table's delimiter row didn't specify enough columns.
func alignOf(aligns []east.Alignment, i int) east.Alignment {
	if i < len(aligns) {
		return aligns[i]
	}
	return east.AlignNone
}

// renderCode highlights a fenced/indented code block body with chromaformat and
// indents every line by two spaces. Code is hard-wrapped downstream, not
// word-wrapped here.
func (r *renderer) renderCode(code, lang string, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	code = strings.TrimSuffix(code, "\n")
	const indent = "  "
	width := r.width - len(contPrefix) - len(indent)
	if width < 1 {
		width = 1
	}
	opts := chromaformat.Options{
		Language:     lang,
		StyleName:    r.styleName,
		BaseStyle:    r.textBase,
		TabWidth:     config.DefaultPreviewTabWidth,
		ContentWidth: width,
	}
	if lang == "" {
		opts.Path = "code.txt"
	}
	hl := chromaformat.Highlight(code, opts)
	indentCells := r.runes(indent, r.textBase)
	first := true
	for _, line := range splitCellLines(hl.Cells) {
		prefix := contPrefix
		if first {
			prefix = firstPrefix
		}
		first = false
		cells := append(append([]previewpanel.AnsiCell{}, prefix...), indentCells...)
		cells = append(cells, line...)
		r.writeLine(cells)
	}
}

// renderRawLines writes text as-is (one output line per source line), used for
// raw HTML blocks where word-wrapping would garble markup.
func (r *renderer) renderRawLines(text string, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	text = strings.TrimSuffix(text, "\n")
	first := true
	for _, line := range strings.Split(text, "\n") {
		prefix := contPrefix
		if first {
			prefix = firstPrefix
		}
		first = false
		cells := append(append([]previewpanel.AnsiCell{}, prefix...), r.runes(line, r.textBase)...)
		r.writeLine(cells)
	}
}

// emitWrapped word-wraps content to the width available after contPrefix and
// writes it, prefixing the first line with firstPrefix and the rest with
// contPrefix. firstPrefix and contPrefix are expected to be the same length.
func (r *renderer) emitWrapped(content, firstPrefix, contPrefix []previewpanel.AnsiCell) {
	width := r.width - len(contPrefix)
	if width < 1 {
		width = 1
	}
	for i, line := range wrapCells(content, width) {
		prefix := contPrefix
		if i == 0 {
			prefix = firstPrefix
		}
		cells := append(append([]previewpanel.AnsiCell{}, prefix...), line...)
		r.writeLine(cells)
	}
}

// inline renders the inline children of a block node n using base as the
// default style; nested emphasis/strong/code/link nodes layer their own style
// on top.
func (r *renderer) inline(n ast.Node, base tcell.Style) []previewpanel.AnsiCell {
	var out []previewpanel.AnsiCell
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, r.renderInline(c, base)...)
	}
	return out
}

func (r *renderer) renderInline(n ast.Node, base tcell.Style) []previewpanel.AnsiCell {
	switch v := n.(type) {
	case *ast.Text:
		cells := r.runes(string(v.Segment.Value(r.source)), base)
		switch {
		case v.HardLineBreak():
			cells = append(cells, previewpanel.AnsiCell{R: '\n', St: base})
		case v.SoftLineBreak():
			cells = append(cells, previewpanel.AnsiCell{R: ' ', St: base})
		}
		return cells
	case *ast.String:
		return r.runes(string(v.Value), base)
	case *ast.Emphasis:
		st := r.styleFor(chroma.GenericEmph).Italic(true)
		if v.Level == 2 {
			st = r.styleFor(chroma.GenericStrong).Bold(true)
		}
		return r.inline(n, st)
	case *ast.CodeSpan:
		return r.inline(n, r.styleFor(chroma.LiteralString))
	case *ast.AutoLink:
		st := r.styleFor(chroma.NameAttribute).Underline(true)
		return r.runes(string(v.URL(r.source)), st)
	case *ast.Link:
		return r.renderLinkLike(n, v.Destination, base)
	case *ast.Image:
		return r.renderLinkLike(n, v.Destination, base)
	case *ast.RawHTML:
		return r.runes(string(v.Segments.Value(r.source)), base)
	default:
		return r.inline(n, base)
	}
}

// renderLinkLike renders link/image text (its inline children) followed by
// a dimmed " (destination)" suffix when a destination is present.
func (r *renderer) renderLinkLike(n ast.Node, destination []byte, base tcell.Style) []previewpanel.AnsiCell {
	st := r.styleFor(chroma.NameAttribute).Underline(true)
	out := r.inline(n, st)
	if len(destination) > 0 {
		dim := r.styleFor(chroma.Comment)
		out = append(out, r.runes(" ("+string(destination)+")", dim)...)
	}
	return out
}
