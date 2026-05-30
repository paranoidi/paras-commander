package ui

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func drawFooter(screen tcell.Screen, rect Rect, styles theme.Theme, fkeys []menu.FunctionKey) {
	keyRegionWidth := rect.Width

	for col := rect.X; col < rect.X+rect.Width; col++ {
		screen.SetContent(col, rect.Y, ' ', nil, styles.FooterLabel)
	}

	visible := make([]menu.FunctionKey, 0, len(fkeys))
	for _, item := range fkeys {
		if item.FullHint() != "" {
			visible = append(visible, item)
		}
	}
	n := len(visible)
	if n > 0 && keyRegionWidth > 0 {
		hints, sumW := footerHintStringsFittingWidth(visible, keyRegionWidth)
		remaining := keyRegionWidth - sumW
		x := rect.X
		gapsBetween := n - 1
		for i, item := range visible {
			keyRunes := utf8.RuneCountInString(item.KeyLabel)
			hintPrimary := hints[i]
			prefixRunes := utf8.RuneCountInString(item.HintShiftPrefix)
			primaryRunes := utf8.RuneCountInString(hintPrimary)
			hintRunes := prefixRunes + primaryRunes
			primitive.TextOverlay(screen, x, rect.Y, keyRunes, item.KeyLabel, styles.FooterKey)
			x += keyRunes
			if hintRunes > 0 {
				screen.SetContent(x, rect.Y, ' ', nil, styles.FooterLabel)
				x++
				if prefixRunes > 0 {
					primitive.TextOverlay(screen, x, rect.Y, prefixRunes, item.HintShiftPrefix, styles.FooterLabelShift)
					x += prefixRunes
				}
				if primaryRunes > 0 {
					primitive.TextOverlay(screen, x, rect.Y, primaryRunes, hintPrimary, styles.FooterLabel)
					x += primaryRunes
				}
			}
			if i < n-1 && gapsBetween > 0 {
				gap := remaining / gapsBetween
				if i < remaining%gapsBetween {
					gap++
				}
				x += gap
			}
		}
	}
}

// footerHintStringsFittingWidth returns per-item primary hint strings (possibly truncated) and total width
// (key + space + hint for each item). Extra horizontal space is for gaps between items, not padding
// inside columns. maxHintWidth -1 means do not truncate.
func footerHintStringsFittingWidth(visible []menu.FunctionKey, keyRegionWidth int) ([]string, int) {
	_, sumFull := footerMeasureWithMaxHint(visible, -1)
	if sumFull <= keyRegionWidth {
		return footerMeasureWithMaxHint(visible, -1)
	}
	hi := maxFooterHintRunes(visible)
	lo := 0
	for lo < hi {
		mid := (lo + hi + 1) / 2
		_, s := footerMeasureWithMaxHint(visible, mid)
		if s <= keyRegionWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return footerMeasureWithMaxHint(visible, lo)
}

func maxFooterHintRunes(visible []menu.FunctionKey) int {
	m := 0
	for _, v := range visible {
		if n := utf8.RuneCountInString(v.FullHint()); n > m {
			m = n
		}
	}
	return m
}

func footerMeasureWithMaxHint(visible []menu.FunctionKey, maxHintRunes int) ([]string, int) {
	hints := make([]string, len(visible))
	sum := 0
	for i, item := range visible {
		keyRunes := utf8.RuneCountInString(item.KeyLabel)
		hintPrimary := footerHintPrimaryTruncated(item, maxHintRunes)
		hintRunes := utf8.RuneCountInString(item.HintShiftPrefix) + utf8.RuneCountInString(hintPrimary)
		innerSpace := 0
		if hintRunes > 0 {
			innerSpace = 1
		}
		hints[i] = hintPrimary
		sum += keyRunes + innerSpace + hintRunes
	}
	return hints, sum
}

func footerHintPrimaryTruncated(item menu.FunctionKey, maxHintRunes int) string {
	prefix := item.HintShiftPrefix
	primary := item.Hint
	full := prefix + primary
	if maxHintRunes < 0 || utf8.RuneCountInString(full) <= maxHintRunes {
		return primary
	}
	prefixRunes := utf8.RuneCountInString(prefix)
	if maxHintRunes <= prefixRunes {
		return ""
	}
	return primitive.TruncateRight(primary, maxHintRunes-prefixRunes)
}
