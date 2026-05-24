package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
)

// MassRenameDiff returns rune-index half-open ranges in old for removed segments and in new
// for added segments, using an LCS alignment so scattered edits highlight only changed runes.
func MassRenameDiff(old, new string) (removedInOld, addedInNew []search.Range) {
	if old == new {
		return nil, nil
	}
	a := []rune(old)
	b := []rune(new)
	if len(a) == 0 {
		if len(b) > 0 {
			return nil, []search.Range{{Start: 0, End: len(b)}}
		}
		return nil, nil
	}
	if len(b) == 0 {
		return []search.Range{{Start: 0, End: len(a)}}, nil
	}

	pairs := massRenameLCSMatches(a, b)
	removedInOld, addedInNew = massRenameDiffFromLCS(a, b, pairs)
	return removedInOld, addedInNew
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

func massRenameDiffFromLCS(a, b []rune, pairs [][2]int) (removed, added []search.Range) {
	ai, bi := 0, 0
	for _, p := range pairs {
		if ai < p[0] {
			removed = massRenameAppendRange(removed, ai, p[0])
			ai = p[0]
		}
		if bi < p[1] {
			added = massRenameAppendRange(added, bi, p[1])
			bi = p[1]
		}
		ai++
		bi++
	}
	if ai < len(a) {
		removed = massRenameAppendRange(removed, ai, len(a))
	}
	if bi < len(b) {
		added = massRenameAppendRange(added, bi, len(b))
	}
	return removed, added
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
func massRenameBeforePreviewRow(line string, removed, replaced []search.Range, width int, base, removedStyle, replacedStyle tcell.Style) (string, []primitive.Span) {
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
	spans = append(spans, fuzzyHighlightSpans(orig, disp, replaced, replacedStyle)...)
	spans = append(spans, fuzzyHighlightSpans(orig, disp, removed, removedStyle)...)
	return dispStr, spans
}

// massRenamePreviewRow prepares display text and highlight spans for one preview column.
func massRenamePreviewRow(line string, ranges []search.Range, width int, base, highlight tcell.Style) (string, []primitive.Span) {
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
	highlight = highlight.Background(bg)
	spans := fuzzyHighlightSpans(orig, disp, ranges, highlight)
	return dispStr, spans
}
