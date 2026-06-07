package ui

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

type footerHintLayout struct {
	primary    string
	showPrefix bool
}

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
		layouts, sumW := footerHintStringsFittingWidth(visible, keyRegionWidth)
		remaining := keyRegionWidth - sumW
		x := rect.X
		gapsBetween := n - 1
		for i, item := range visible {
			layout := layouts[i]
			keyRunes := utf8.RuneCountInString(item.KeyLabel)
			prefix := ""
			if layout.showPrefix {
				prefix = item.HintShiftPrefix
			}
			prefixRunes := utf8.RuneCountInString(prefix)
			primaryRunes := utf8.RuneCountInString(layout.primary)
			hintRunes := prefixRunes + primaryRunes
			primitive.TextOverlay(screen, x, rect.Y, keyRunes, item.KeyLabel, styles.FooterKey)
			x += keyRunes
			if hintRunes > 0 {
				screen.SetContent(x, rect.Y, ' ', nil, styles.FooterLabel)
				x++
				if primaryRunes > 0 {
					primitive.TextOverlay(screen, x, rect.Y, primaryRunes, layout.primary, styles.FooterLabel)
					x += primaryRunes
				}
				if prefixRunes > 0 {
					primitive.TextOverlay(screen, x, rect.Y, prefixRunes, prefix, styles.FooterLabelShift)
					x += prefixRunes
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

// footerHintStringsFittingWidth returns per-item hint layouts (possibly without shift
// suffixes and/or truncated primary text) and total width (key + space + hint for each
// item). Extra horizontal space is for gaps between items, not padding inside columns.
func footerHintStringsFittingWidth(visible []menu.FunctionKey, keyRegionWidth int) ([]footerHintLayout, int) {
	layouts, sum := footerMeasureLayouts(visible, true, -1)
	if sum <= keyRegionWidth {
		return layouts, sum
	}
	layouts, sum = footerMeasureLayouts(visible, false, -1)
	if sum <= keyRegionWidth {
		return layouts, sum
	}
	hi := maxFooterPrimaryRunes(visible)
	lo := 0
	for lo < hi {
		mid := (lo + hi + 1) / 2
		_, s := footerMeasureLayouts(visible, false, mid)
		if s <= keyRegionWidth {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return footerMeasureLayouts(visible, false, lo)
}

func maxFooterPrimaryRunes(visible []menu.FunctionKey) int {
	m := 0
	for _, v := range visible {
		if n := utf8.RuneCountInString(v.Hint); n > m {
			m = n
		}
	}
	return m
}

func footerMeasureLayouts(visible []menu.FunctionKey, showPrefix bool, maxPrimaryRunes int) ([]footerHintLayout, int) {
	layouts := make([]footerHintLayout, len(visible))
	sum := 0
	for i, item := range visible {
		layout := footerHintLayoutForItem(item, showPrefix, maxPrimaryRunes)
		layouts[i] = layout
		sum += footerItemWidth(item, layout)
	}
	return layouts, sum
}

func footerHintLayoutForItem(item menu.FunctionKey, showPrefix bool, maxPrimaryRunes int) footerHintLayout {
	primary := item.Hint
	if maxPrimaryRunes >= 0 && utf8.RuneCountInString(primary) > maxPrimaryRunes {
		primary = primitive.TruncateRight(primary, maxPrimaryRunes)
	}
	return footerHintLayout{
		primary:    primary,
		showPrefix: showPrefix && item.HintShiftPrefix != "",
	}
}

func footerItemWidth(item menu.FunctionKey, layout footerHintLayout) int {
	keyRunes := utf8.RuneCountInString(item.KeyLabel)
	hintRunes := utf8.RuneCountInString(layout.primary)
	if layout.showPrefix {
		hintRunes += utf8.RuneCountInString(item.HintShiftPrefix)
	}
	innerSpace := 0
	if hintRunes > 0 {
		innerSpace = 1
	}
	return keyRunes + innerSpace + hintRunes
}
