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

// footerHintSpan is one drawn footer entry: the key label, then a space and the hint.
type footerHintSpan struct {
	X, Width int
	Item     menu.FunctionKey
	layout   footerHintLayout
}

// footerHintSpans lays out the entries of fkeys that have a hint across rect, spreading the spare
// width evenly into the gaps between them.
func footerHintSpans(rect Rect, fkeys []menu.FunctionKey) []footerHintSpan {
	visible := make([]menu.FunctionKey, 0, len(fkeys))
	for _, item := range fkeys {
		if item.FullHint() != "" {
			visible = append(visible, item)
		}
	}
	n := len(visible)
	if n == 0 || rect.Width <= 0 {
		return nil
	}
	layouts, sumW := footerHintStringsFittingWidth(visible, rect.Width)
	remaining := rect.Width - sumW
	gapsBetween := n - 1
	spans := make([]footerHintSpan, 0, n)
	x := rect.X
	for i, item := range visible {
		w := footerItemWidth(item, layouts[i])
		spans = append(spans, footerHintSpan{X: x, Width: w, Item: item, layout: layouts[i]})
		x += w
		if i < n-1 && gapsBetween > 0 {
			gap := remaining / gapsBetween
			if i < remaining%gapsBetween {
				gap++
			}
			x += gap
		}
	}
	return spans
}

// shiftStart is the column where the span's Shift-alternative word starts; ok is false when the
// word is not drawn.
func (span footerHintSpan) shiftStart() (int, bool) {
	if !span.layout.showPrefix {
		return 0, false
	}
	return span.X + span.Width - utf8.RuneCountInString(span.Item.HintShiftPrefix), true
}

// footerHit finds the footer entry drawn at (x, y) (its key label or its hint, not the gaps) and
// whether x lies on its Shift-alternative word with a binding of its own (ShiftKeyLabel). An
// informational suffix belongs to the entry's main part.
func footerHit(rect Rect, fkeys []menu.FunctionKey, x, y int) (FooterHover, menu.FunctionKey) {
	if rect.Height <= 0 || y != rect.Y {
		return FooterHover{}, menu.FunctionKey{}
	}
	for i, span := range footerHintSpans(rect, fkeys) {
		if x < span.X || x >= span.X+span.Width {
			continue
		}
		start, drawn := span.shiftStart()
		shift := drawn && span.Item.ShiftKeyLabel != "" && x >= start
		return FooterHover{Active: true, Index: i, Shift: shift}, span.Item
	}
	return FooterHover{}, menu.FunctionKey{}
}

// drawFooter paints the footer row. The entry under the mouse pointer (hover) is drawn in
// footer.hover, and the part a click there would press in footer.hover.active: the
// Shift-alternative word, or the key label and main hint.
func drawFooter(screen tcell.Screen, rect Rect, styles theme.Theme, fkeys []menu.FunctionKey, hover FooterHover) {
	for col := rect.X; col < rect.X+rect.Width; col++ {
		screen.SetContent(col, rect.Y, ' ', nil, styles.FooterLabel)
	}
	for i, span := range footerHintSpans(rect, fkeys) {
		item, layout, x := span.Item, span.layout, span.X
		keyStyle, labelStyle, shiftStyle := styles.FooterKey, styles.FooterLabel, styles.FooterLabelShift
		if hover.Active && hover.Index == i {
			keyStyle, labelStyle, shiftStyle = styles.FooterHoverActive, styles.FooterHoverActive, styles.FooterHover
			if hover.Shift {
				keyStyle, labelStyle, shiftStyle = styles.FooterHover, styles.FooterHover, styles.FooterHoverActive
			}
		}
		keyRunes := utf8.RuneCountInString(item.KeyLabel)
		prefix := ""
		if layout.showPrefix {
			prefix = item.HintShiftPrefix
		}
		prefixRunes := utf8.RuneCountInString(prefix)
		primaryRunes := utf8.RuneCountInString(layout.primary)
		primitive.TextOverlay(screen, x, rect.Y, keyRunes, item.KeyLabel, keyStyle)
		x += keyRunes
		if prefixRunes+primaryRunes == 0 {
			continue
		}
		screen.SetContent(x, rect.Y, ' ', nil, labelStyle)
		x++
		if primaryRunes > 0 {
			primitive.TextOverlay(screen, x, rect.Y, primaryRunes, layout.primary, labelStyle)
			x += primaryRunes
		}
		if prefixRunes > 0 {
			primitive.TextOverlay(screen, x, rect.Y, prefixRunes, prefix, shiftStyle)
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
