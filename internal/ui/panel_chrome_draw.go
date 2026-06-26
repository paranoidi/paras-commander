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
	primitive.Box(screen, primitive.Rect(rect), chrome.Frame)
	inner := Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		primitive.Fill(screen, primitive.Rect(inner), ' ', chrome.Surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	if titleWidth < 1 {
		titleWidth = 1
	}
	innerRight := rect.X + rect.Width - 2
	contentCols := innerRight - titleX + 1
	if endLabel != "" {
		endStyle := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, active, blocked)
		paintAuxPanelTopRowSplit(screen, titleX, innerRight, contentCols, rect.Y, title, endLabel, chrome.Title, endStyle)
	} else {
		primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, chrome.Title)
	}
	return AuxPanelChromeLayout{
		Inner:      inner,
		TitleX:     titleX,
		TitleWidth: titleWidth,
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

// paintAuxPanelTopRowSplit paints a top border row with a start title and optional end label (frame dashes between).
func paintAuxPanelTopRowSplit(screen tcell.Screen, titleX, innerRight, contentCols, y int, leftTitle, endLabel string, titleStyle, endStyle tcell.Style) {
	endRunes := utf8.RuneCountInString(endLabel)
	showEnd := endLabel != "" && endRunes > 0 && contentCols >= endRunes+gapBeforePanelTitleEnd+3
	endStartX := 0
	pathSlotCols := contentCols
	if showEnd {
		endStartX = innerRight - endRunes
		pathSlotCols = endStartX - titleX - gapBeforePanelTitleEnd
		if pathSlotCols < 3 {
			showEnd = false
			pathSlotCols = contentCols
		}
	}
	primitive.TextOverlay(screen, titleX, y, pathSlotCols, leftTitle, titleStyle)
	if !showEnd {
		return
	}
	primitive.TextOverlay(screen, endStartX, y, endRunes, endLabel, endStyle)
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
