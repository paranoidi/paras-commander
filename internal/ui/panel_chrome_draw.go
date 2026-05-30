package ui

import (
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
func drawAuxPanelChrome(screen tcell.Screen, rect Rect, title string, active, blocked bool, styles theme.Theme) AuxPanelChromeLayout {
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
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, chrome.Title)
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

func auxPanelListHeaderStyle(chrome theme.PanelChrome, blocked bool, contentBG tcell.Color) tcell.Style {
	if blocked {
		return chrome.Header
	}
	return chrome.Header.Background(contentBG)
}
