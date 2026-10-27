package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// MassRenameDiff aligns old and new with an LCS so scattered edits highlight only changed
// runes, and returns rune-index half-open ranges: in old, the segments dropped with nothing in
// their place (removed) and the segments new swaps for other text (replaced); in new, the
// segments not in old (added). The before preview column styles removed and replaced like a
// find/replace match, so every mode colors its changes the same way.
func MassRenameDiff(old, new string) (removed, replaced, added []search.Range) {
	if old == new {
		return nil, nil, nil
	}
	a := []rune(old)
	b := []rune(new)
	if len(a) == 0 {
		return nil, nil, []search.Range{{Start: 0, End: len(b)}}
	}
	if len(b) == 0 {
		return []search.Range{{Start: 0, End: len(a)}}, nil, nil
	}
	return massRenameDiffFromLCS(a, b, massRenameLCSMatches(a, b))
}

func massRenameLCSMatches(a, b []rune) [][2]int {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	pairs := make([][2]int, 0, min(len(a), len(b)))
	i, j := n, m
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			pairs = append([][2]int{{i - 1, j - 1}}, pairs...)
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return pairs
}

// massRenameDiffFromLCS walks the gaps between matched rune pairs: an old gap opposite a new
// gap is replaced, an old gap alone is removed, and every new gap is added.
func massRenameDiffFromLCS(a, b []rune, pairs [][2]int) (removed, replaced, added []search.Range) {
	ai, bi := 0, 0
	gap := func(aEnd, bEnd int) {
		if ai < aEnd {
			if bi < bEnd {
				replaced = massRenameAppendRange(replaced, ai, aEnd)
			} else {
				removed = massRenameAppendRange(removed, ai, aEnd)
			}
		}
		added = massRenameAppendRange(added, bi, bEnd)
	}
	for _, p := range pairs {
		gap(p[0], p[1])
		ai, bi = p[0]+1, p[1]+1
	}
	gap(len(a), len(b))
	return removed, replaced, added
}

func massRenameAppendRange(ranges []search.Range, start, end int) []search.Range {
	if start >= end {
		return ranges
	}
	if n := len(ranges); n > 0 && ranges[n-1].End == start {
		ranges[n-1].End = end
		return ranges
	}
	return append(ranges, search.Range{Start: start, End: end})
}

// massRenameBeforePreviewRow prepares display text and highlight spans for the before preview column.
func massRenameBeforePreviewRow(line string, removed, replaced []search.Range, ignored search.Range, width int, base, removedStyle, replacedStyle, ignoredStyle tcell.Style) (string, []primitive.Span) {
	if width <= 0 {
		return "", nil
	}
	orig := []rune(line)
	dispStr := line
	if len(orig) > width {
		dispStr = primitive.TruncateRight(line, width)
	}
	disp := []rune(dispStr)
	_, bg, _ := base.Decompose()
	removedStyle = removedStyle.Background(bg)
	replacedStyle = replacedStyle.Background(bg)
	var spans []primitive.Span
	spans = append(spans, draw.FuzzyHighlightSpans(orig, disp, replaced, replacedStyle)...)
	spans = append(spans, draw.FuzzyHighlightSpans(orig, disp, removed, removedStyle)...)
	if ignored.End > ignored.Start {
		spans = append(spans, draw.FuzzyHighlightSpans(orig, disp, []search.Range{ignored}, ignoredStyle.Background(bg))...)
	}
	return dispStr, spans
}

// massRenameAfterPreviewRow prepares the after preview column, including validation error styling.
func massRenameAfterPreviewRow(line string, ranges []search.Range, width int, base, added, errorStyle tcell.Style, hasError bool) (string, []primitive.Span) {
	if hasError {
		return massRenamePreviewRowStyled(line, nil, width, base, errorStyle, true, errorStyle)
	}
	return massRenamePreviewRowStyled(line, ranges, width, base, added, false, added)
}

func massRenamePreviewRowStyled(line string, ranges []search.Range, width int, base, highlight tcell.Style, fullLine bool, highlightStyle tcell.Style) (string, []primitive.Span) {
	if width <= 0 {
		return "", nil
	}
	orig := []rune(line)
	dispStr := line
	if len(orig) > width {
		dispStr = primitive.TruncateRight(line, width)
	}
	disp := []rune(dispStr)
	_, bg, _ := base.Decompose()
	highlightStyle = highlightStyle.Background(bg)
	var spans []primitive.Span
	if fullLine && len(orig) > 0 {
		end := len(orig)
		if end > len(disp) {
			end = len(disp)
		}
		spans = draw.FuzzyHighlightSpans(orig, disp, []search.Range{{Start: 0, End: end}}, highlightStyle)
	} else {
		highlight = highlight.Background(bg)
		spans = draw.FuzzyHighlightSpans(orig, disp, ranges, highlight)
	}
	return dispStr, spans
}
