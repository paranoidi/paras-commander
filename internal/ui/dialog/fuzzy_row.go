package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
)

// fuzzyRowContent renders one list row with optional fuzzy highlights.
// When pathFit is true, relative paths use primitive.FitPathForWidth; otherwise
// primitive.TruncateMiddle is used for non-path lines.
func fuzzyRowContent(line string, ranges []search.Range, width int, matchStyle tcell.Style, pathFit bool) (string, []primitive.Span) {
	if width <= 0 {
		return "", nil
	}
	orig := []rune(line)
	var dispStr string
	switch {
	case len(orig) <= width:
		dispStr = line
	case pathFit:
		dispStr = primitive.FitPathForWidth(line, width)
	default:
		dispStr = primitive.TruncateMiddle(line, width)
	}
	disp := []rune(dispStr)
	spans := fuzzyHighlightSpans(orig, disp, ranges, matchStyle)
	return dispStr, spans
}

// fuzzyPathRowContent is fuzzyRowContent with path-aware shortening (find dialog paths).
func fuzzyPathRowContent(line string, ranges []search.Range, width int, matchStyle tcell.Style) (string, []primitive.Span) {
	return fuzzyRowContent(line, ranges, width, matchStyle, true)
}

func fuzzyHighlightSpans(orig, disp []rune, ranges []search.Range, matchStyle tcell.Style) []primitive.Span {
	if len(ranges) == 0 || len(disp) == 0 {
		return nil
	}
	origToDisp := alignOrigToDisp(orig, disp)
	spans := make([]primitive.Span, 0, len(ranges))
	for _, r := range ranges {
		start, end := -1, -1
		for i := r.Start; i < r.End && i < len(origToDisp); i++ {
			d := origToDisp[i]
			if d < 0 {
				continue
			}
			if start < 0 {
				start = d
			}
			if d+1 > end {
				end = d + 1
			}
		}
		if start >= 0 && end > start {
			spans = append(spans, primitive.Span{Start: start, End: end, Style: matchStyle})
		}
	}
	return spans
}

// alignOrigToDisp maps each original rune index to a display index when the display
// string is a shortened form of the original (FitPathForWidth / TruncateMiddle).
func alignOrigToDisp(orig, disp []rune) []int {
	m := make([]int, len(orig))
	for i := range m {
		m[i] = -1
	}
	oi, di := 0, 0
	for oi < len(orig) && di < len(disp) {
		if orig[oi] == disp[di] {
			m[oi] = di
			oi++
			di++
			continue
		}
		if disp[di] == '…' {
			di++
			continue
		}
		oi++
	}
	return m
}
