package mdformat

import "github.com/paranoidi/paras-commander/internal/ui/previewpanel"

// wrapCells breaks cells into lines of at most width runes, preserving per-cell
// style. Preference is given to breaking at space cells (word wrap); a single
// "word" longer than width is hard-broken. An embedded '\n' cell forces a line
// break (used for markdown hard line breaks). Mirrors the shape of
// ui.WrapWordsToWidth, but keeps styles instead of discarding them.
func wrapCells(cells []previewpanel.AnsiCell, width int) [][]previewpanel.AnsiCell {
	if width < 1 {
		width = 1
	}
	var out [][]previewpanel.AnsiCell
	var seg []previewpanel.AnsiCell
	flush := func() {
		out = append(out, wrapSegment(seg, width)...)
		seg = nil
	}
	for _, c := range cells {
		if c.R == '\n' {
			flush()
			continue
		}
		seg = append(seg, c)
	}
	flush()
	if len(out) == 0 {
		out = append(out, nil)
	}
	return out
}

func wrapSegment(cells []previewpanel.AnsiCell, width int) [][]previewpanel.AnsiCell {
	cells = trimSpaceCells(cells)
	if len(cells) == 0 {
		return [][]previewpanel.AnsiCell{{}}
	}
	words := splitWords(cells)
	var lines [][]previewpanel.AnsiCell
	var cur []previewpanel.AnsiCell
	flush := func() {
		lines = append(lines, cur)
		cur = nil
	}
	for _, w := range words {
		if len(w) > width {
			if len(cur) > 0 {
				flush()
			}
			for len(w) > 0 {
				take := width
				if take > len(w) {
					take = len(w)
				}
				lines = append(lines, append([]previewpanel.AnsiCell{}, w[:take]...))
				w = w[take:]
			}
			continue
		}
		if len(cur) == 0 {
			cur = append([]previewpanel.AnsiCell{}, w...)
			continue
		}
		if len(cur)+1+len(w) > width {
			flush()
			cur = append([]previewpanel.AnsiCell{}, w...)
		} else {
			cur = append(cur, previewpanel.AnsiCell{R: ' ', St: w[0].St})
			cur = append(cur, w...)
		}
	}
	if len(cur) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

func splitWords(cells []previewpanel.AnsiCell) [][]previewpanel.AnsiCell {
	var words [][]previewpanel.AnsiCell
	var cur []previewpanel.AnsiCell
	for _, c := range cells {
		if c.R == ' ' {
			if len(cur) > 0 {
				words = append(words, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, c)
	}
	if len(cur) > 0 {
		words = append(words, cur)
	}
	return words
}

func trimSpaceCells(cells []previewpanel.AnsiCell) []previewpanel.AnsiCell {
	start := 0
	for start < len(cells) && cells[start].R == ' ' {
		start++
	}
	end := len(cells)
	for end > start && cells[end-1].R == ' ' {
		end--
	}
	return cells[start:end]
}

// splitCellLines splits cells on '\n' without any width wrapping (used for
// fenced/indented code bodies, which are hard-wrapped downstream instead).
func splitCellLines(cells []previewpanel.AnsiCell) [][]previewpanel.AnsiCell {
	var lines [][]previewpanel.AnsiCell
	var cur []previewpanel.AnsiCell
	for _, c := range cells {
		if c.R == '\n' {
			lines = append(lines, cur)
			cur = nil
			continue
		}
		cur = append(cur, c)
	}
	lines = append(lines, cur)
	return lines
}
