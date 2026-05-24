package panelcarousel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func fuzzySpans(entry localfs.Entry, colWidth int, ranges []search.Range, highlightCursor bool, styles theme.Theme, showIcons bool, jobMark rune, subtree bool, nameBGAt func(displayIndex int) tcell.Style) []primitive.Span {
	if len(ranges) == 0 {
		return nil
	}
	nameWidth := nameWidthForColumn(colWidth, showIcons)
	display := entryDisplayRunes(entry, nameWidth, showIcons, jobMark, subtree)
	matchStyle := styles.FuzzyHighlight
	if highlightCursor {
		matchStyle = styles.FuzzyHighlightCursor
	}
	spans := make([]primitive.Span, 0, len(ranges))
	for displayIndex, dr := range display {
		if dr.NameIdx < 0 || !rangeContains(ranges, dr.NameIdx) {
			continue
		}
		_, background, _ := nameBGAt(displayIndex).Decompose()
		spans = append(spans, primitive.Span{
			Start: displayIndex,
			End:   displayIndex + 1,
			Style: matchStyle.Background(background),
		})
	}
	return spans
}

func rangeContains(ranges []search.Range, index int) bool {
	for _, r := range ranges {
		if index >= r.Start && index < r.End {
			return true
		}
	}
	return false
}
