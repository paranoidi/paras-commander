package previewpanel

import (
	"sort"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// minAnchorRunes is the shortest row text worth searching for: anything less, such as a lone "}"
// or "end", repeats too often to say where the view is.
const minAnchorRunes = 4

// diffNoisePrefixes start the lines of a diff that are not text of the file as it is now. A removed
// line is among them, so "-" covers the "---" header too.
var diffNoisePrefixes = []string{"-", "@@", "diff --git", "index ", "+++ ", `\ No newline`}

func diffNoiseLine(cells []AnsiCell) bool {
	var b strings.Builder
	for _, c := range cells[:min(len(cells), len(`\ No newline`))] {
		b.WriteRune(c.R)
	}
	text := b.String()
	for _, prefix := range diffNoisePrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// fileRows returns the comparable text of each wrapped row: letters and digits only, lowered, so a
// row reads the same whatever marked it up (diff "+", markdown "#", "**", table bars). Rows holding
// no file text, such as a gutter-only row or diff noise, come back empty.
func (st State) fileRows(textWidth int, base tcell.Style) []string {
	l := st.layoutFor(textWidth, base)
	rows := make([]string, l.rowCount())
	var b strings.Builder
	noise := false
	for i, r := range l.rows {
		start := r.start
		if !r.cont {
			start = min(start+l.gutter, r.end)
			noise = st.IsDiff && diffNoiseLine(l.cells[start:r.end])
		}
		if noise {
			continue
		}
		b.Reset()
		for _, c := range l.cells[start:r.end] {
			if unicode.IsLetter(c.R) || unicode.IsDigit(c.R) {
				b.WriteRune(unicode.ToLower(c.R))
			}
		}
		rows[i] = b.String()
	}
	return rows
}

// ScrollAnchors returns the file text of the count rows from top that are worth locating in
// another rendering of the same file, topmost first.
func (st State) ScrollAnchors(top, count, textWidth int, base tcell.Style) []string {
	rows := st.fileRows(textWidth, base)
	top = max(0, min(top, len(rows)))
	var anchors []string
	for _, row := range rows[top:min(top+count, len(rows))] {
		if len([]rune(row)) >= minAnchorRunes {
			anchors = append(anchors, row)
		}
	}
	return anchors
}

// LocateAnchor returns the row where the first locatable anchor starts. An anchor repeated in the
// body resolves to the occurrence nearest nearRow. The body is searched as one string because the
// two renderings wrap differently, so an anchor may straddle rows here.
func (st State) LocateAnchor(anchors []string, nearRow, textWidth int, base tcell.Style) (row int, ok bool) {
	rows := st.fileRows(textWidth, base)
	starts := make([]int, len(rows))
	var b strings.Builder
	for i, text := range rows {
		starts[i] = b.Len()
		b.WriteString(text)
	}
	body := b.String()
	for _, anchor := range anchors {
		for off := 0; ; {
			i := strings.Index(body[off:], anchor)
			if i < 0 {
				break
			}
			off += i
			candidate := sort.SearchInts(starts, off+1) - 1
			if !ok || abs(candidate-nearRow) < abs(row-nearRow) {
				row, ok = candidate, true
			}
			off++
		}
		if ok {
			return row, true
		}
	}
	return 0, false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
