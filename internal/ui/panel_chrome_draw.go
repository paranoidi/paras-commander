package ui

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
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
func drawAuxPanelChrome(screen tcell.Screen, rect Rect, title, endLabel string, active, blocked bool, styles theme.Theme) AuxPanelChromeLayout {
	chrome := styles.PanelChrome(active, blocked)
	primitive.Box(screen, primitive.Rect(rect), chrome.Frame, primitive.SharpBorder)
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
		paintAuxPanelTopRow(screen, titleX, innerRight, contentCols, rect.Y, title, endLabel, chrome.Title, endStyle, chrome.Frame)
	} else {
		paintAuxPanelTopRow(screen, titleX, innerRight, contentCols, rect.Y, title, "", chrome.Title, chrome.Title, chrome.Frame)
	}
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

// paintAuxPanelTopRow paints the top border row with a title, optional end label, and frame dashes elsewhere.
func paintAuxPanelTopRow(screen tcell.Screen, titleX, innerRight, contentCols, y int, leftTitle, endLabel string, titleStyle, endStyle, borderStyle tcell.Style) {
	// Leave one frame-dash column between the end label and the right border so the
	// indicator (compare filter / dedup sort) isn't flush against the corner.
	const endRightMargin = 1
	endRunes := utf8.RuneCountInString(endLabel)
	showEnd := endLabel != "" && endRunes > 0 && contentCols >= endRunes+gapBeforePanelTitleEnd+endRightMargin+3
	endStartX := 0
	pathSlotCols := contentCols
	if showEnd {
		endStartX = innerRight - endRunes + 1 - endRightMargin
		pathSlotCols = endStartX - titleX - gapBeforePanelTitleEnd
		if pathSlotCols < 3 {
			showEnd = false
			pathSlotCols = contentCols
		}
	}
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

// drawAuxPanelBottomCenterLabel paints centered text on the bottom border interior row.
func drawAuxPanelBottomCenterLabel(screen tcell.Screen, rect Rect, label string, style tcell.Style) {
	if label == "" {
		return
	}
	innerLeft := rect.X + 1
	innerRight := rect.X + rect.Width - 2
	innerW := innerRight - innerLeft + 1
	runes := len([]rune(label))
	if runes > innerW {
		label = primitive.TruncateRight(label, innerW)
		runes = len([]rune(label))
	}
	if runes <= 0 || innerW <= 0 {
		return
	}
	x := innerLeft + (innerW-runes)/2
	y := rect.Y + rect.Height - 1
	primitive.TextOverlay(screen, x, y, runes, label, style)
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
