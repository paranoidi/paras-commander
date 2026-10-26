package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// AuxPanelChromeLayout is the shared chrome layout for jobs/commands/messages auxiliary panels.
type AuxPanelChromeLayout struct {
	Inner      Rect
	TitleX     int
	TitleWidth int
	ContentBG  tcell.Color
	Chrome     theme.PanelChrome
}

// drawAuxPanelChrome paints border, surface fill, and title overlay for an auxiliary panel.
// When endLabel is non-empty, the top row uses a split layout (title left, endLabel right).
// viMotion selects the vi-motion accent frame on the active, unblocked panel (selected strip).
func drawAuxPanelChrome(screen tcell.Screen, rect Rect, title, endLabel string, active, blocked, viMotion bool, styles theme.Theme) AuxPanelChromeLayout {
	return drawAuxPanelChromeEndStyled(screen, rect, title, endLabel, nil, active, blocked, viMotion, styles)
}

// drawAuxPanelChromeEndStyled is drawAuxPanelChrome with an optional end-label restyle: endOn
// receives the default end-label style and returns the one to paint. A blocked panel ignores
// it, so the label dims with the rest of the chrome.
func drawAuxPanelChromeEndStyled(screen tcell.Screen, rect Rect, title, endLabel string, endOn func(tcell.Style) tcell.Style, active, blocked, viMotion bool, styles theme.Theme) AuxPanelChromeLayout {
	chrome := styles.PanelChrome(active, blocked)
	borderStyle := chrome.Frame
	if viMotion && active && !blocked {
		borderStyle = styles.PanelViMotionFrame
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle, primitive.SharpBorder)
	inner := Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		primitive.Fill(screen, primitive.Rect(inner), ' ', chrome.Surface)
	}
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2
	contentCols := innerRight - titleX + 1
	if contentCols < 1 {
		contentCols = 1
	}
	if endLabel != "" {
		endStyle := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, active, blocked)
		if endOn != nil && !blocked {
			endStyle = endOn(endStyle)
		}
		paintAuxPanelTopRow(screen, titleX, innerRight, contentCols, rect.Y, title, endLabel, chrome.Title, endStyle, borderStyle)
	} else {
		paintAuxPanelTopRow(screen, titleX, innerRight, contentCols, rect.Y, title, "", chrome.Title, chrome.Title, borderStyle)
	}
	chrome.Frame = borderStyle
	return AuxPanelChromeLayout{
		Inner:      inner,
		TitleX:     titleX,
		TitleWidth: contentCols,
		ContentBG:  auxPanelContentBG(styles, blocked),
		Chrome:     chrome,
	}
}

func auxPanelContentBG(styles theme.Theme, blocked bool) tcell.Color {
	if blocked {
		_, bg, _ := styles.PanelBlockedSurface.Decompose()
		return bg
	}
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	return bg
}

func auxPanelBodyText(styles theme.Theme, blocked bool, contentBG tcell.Color) tcell.Style {
	if blocked {
		return styles.PanelBlockedText
	}
	return styles.PanelText.Background(contentBG)
}

type titleEndLabelSlots = previewpanel.TitleEndLabelSlots

// titleEndLabelLayout is the ui name for previewpanel.TitleEndLabelLayout so
// aux chrome and paintPanelTopTitleRow share that helper without a second copy.
func titleEndLabelLayout(titleX, innerRight, contentCols, endRunes, endRightMargin, gapBeforeEnd int) titleEndLabelSlots {
	return previewpanel.TitleEndLabelLayout(titleX, innerRight, contentCols, endRunes, endRightMargin, gapBeforeEnd)
}

// paintAuxPanelTopRow paints the top border row with a title, optional end label, and frame dashes elsewhere.
func paintAuxPanelTopRow(screen tcell.Screen, titleX, innerRight, contentCols, y int, leftTitle, endLabel string, titleStyle, endStyle, borderStyle tcell.Style) {
	// Leave one frame-dash column between the end label and the right border so the
	// indicator (compare filter / dedup sort) isn't flush against the corner.
	const endRightMargin = 1
	endRunes := utf8.RuneCountInString(endLabel)
	slots := titleEndLabelLayout(titleX, innerRight, contentCols, endRunes, endRightMargin, gapBeforePanelTitleEnd)
	showEnd := slots.ShowEnd
	endStartX := slots.EndStartX
	pathSlotCols := slots.PathSlotCols
	left := primitive.TruncateRight(leftTitle, pathSlotCols)
	leftRunes := []rune(left)
	endLabelRunes := []rune(endLabel)
	endStartCol := endStartX - titleX

	for col := 0; col < contentCols; col++ {
		x := titleX + col
		var ch rune
		var st tcell.Style
		switch {
		case col < pathSlotCols && col < len(leftRunes):
			ch = leftRunes[col]
			st = titleStyle
		case showEnd && col >= endStartCol && col < endStartCol+endRunes:
			ch = endLabelRunes[col-endStartCol]
			st = endStyle
		default:
			ch = '─'
			st = borderStyle
		}
		screen.SetContent(x, y, ch, nil, st)
	}
}

// drawAuxPanelBottomCenterLabel paints raw label centered on the bottom border interior row,
// adding the one-space border padding itself (callers pass unpadded text).
func drawAuxPanelBottomCenterLabel(screen tcell.Screen, rect Rect, label string, style tcell.Style) {
	if label == "" {
		return
	}
	innerLeft := rect.X + 1
	innerRight := rect.X + rect.Width - 2
	innerW := innerRight - innerLeft + 1
	if innerW <= 2 {
		return
	}
	if len([]rune(label)) > innerW-2 {
		label = primitive.TruncateRight(label, innerW-2)
	}
	label = panelSelectionSizePadded(label)
	runes := len([]rune(label))
	x := innerLeft + (innerW-runes)/2
	y := rect.Y + rect.Height - 1
	primitive.TextOverlay(screen, x, y, runes, label, style)
}

// drawAuxPanelBottomCenterSpans is drawAuxPanelBottomCenterLabel for a label made of
// differently styled segments; the one-space padding on each side takes the first segment's style.
func drawAuxPanelBottomCenterSpans(screen tcell.Screen, rect Rect, segs []styledSegment) {
	var text strings.Builder
	var spans []primitive.Span
	for _, sg := range segs {
		start := utf8.RuneCountInString(text.String())
		text.WriteString(sg.Text)
		spans = append(spans, primitive.Span{Start: start + 1, End: start + 1 + utf8.RuneCountInString(sg.Text), Style: sg.Style})
	}
	if text.Len() == 0 {
		return
	}
	innerLeft := rect.X + 1
	innerW := rect.Width - 2
	if innerW <= 2 {
		return
	}
	label := panelSelectionSizePadded(primitive.TruncateRight(text.String(), innerW-2))
	runes := utf8.RuneCountInString(label)
	x := innerLeft + (innerW-runes)/2
	primitive.StyledText(screen, x, rect.Y+rect.Height-1, runes, label, segs[0].Style, spans)
}

// styledSegment is one differently styled piece of a border label.
type styledSegment struct {
	Text  string
	Style tcell.Style
}

func auxPanelListHeaderStyle(chrome theme.PanelChrome, blocked bool, contentBG tcell.Color) tcell.Style {
	if blocked {
		return chrome.Header
	}
	return chrome.Header.Background(contentBG)
}

// auxPanelListRowStyle resolves a jobs/commands/messages list row's style: rowBase normally,
// or the shared cursor style (theme.Theme.PanelListingCursorStyle) when the row is selected.
func auxPanelListRowStyle(styles theme.Theme, rowBase tcell.Style, selected, chromeBlocked, active bool) tcell.Style {
	if !selected {
		return rowBase
	}
	return styles.PanelListingCursorStyle(rowBase, theme.PanelListingCursorOpts{
		ChromeBlocked:  chromeBlocked,
		FileListActive: active,
	})
}

// paintAuxPanelRowMargin fills the single-cell margin column at x (e.g. the row's left
// or right edge, just inside the panel border) with style.
func paintAuxPanelRowMargin(screen tcell.Screen, x, y int, style tcell.Style) {
	primitive.Text(screen, x, y, 1, "", style)
}

// paintAuxPanelRowMargins fills both the left margin (rect.X+1) and right margin
// (contentX+contentW, just inside the right border) of a jobs/commands/messages list row.
func paintAuxPanelRowMargins(screen tcell.Screen, rect Rect, contentX, contentW, y int, style tcell.Style) {
	paintAuxPanelRowMargin(screen, rect.X+1, y, style)
	paintAuxPanelRowMargin(screen, contentX+contentW, y, style)
}
