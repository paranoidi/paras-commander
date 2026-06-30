package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// panelScrollPos groups the four scroll-position integers passed to drawPanelListScrollbar.
type panelScrollPos struct {
	ListTopY, Visible, Total, Offset int
}

func drawPanelListScrollbar(screen tcell.Screen, rect Rect, pos panelScrollPos, style uiscrollbar.Style, show, fileListActive, chromeBlocked bool, frameStyle tcell.Style, styles theme.Theme) {
	if !show || style == uiscrollbar.StyleNone {
		return
	}
	metrics, ok := uiscrollbar.ComputeMetrics(pos.Total, pos.Visible, pos.Offset)
	if !ok {
		return
	}
	uiscrollbar.Draw(uiscrollbar.DrawParams{
		Screen:     screen,
		X:          rect.X + rect.Width - 1,
		ListTopY:   pos.ListTopY,
		Visible:    pos.Visible,
		Metrics:    metrics,
		Style:      style,
		Active:     fileListActive,
		Blocked:    chromeBlocked,
		FrameStyle: frameStyle,
		Theme:      styles,
	})
}

func panelScrollbarShow(fileListActive, showInactive bool) bool {
	return fileListActive || showInactive
}
