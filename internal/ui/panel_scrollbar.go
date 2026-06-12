package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func drawPanelListScrollbar(screen tcell.Screen, rect Rect, listTopY, visibleRows, total, offset int, style uiscrollbar.Style, show bool, fileListActive, chromeBlocked bool, frameStyle tcell.Style, styles theme.Theme) {
	if !show || style == uiscrollbar.StyleNone {
		return
	}
	metrics, ok := uiscrollbar.ComputeMetrics(total, visibleRows, offset)
	if !ok {
		return
	}
	uiscrollbar.Draw(uiscrollbar.DrawParams{
		Screen:     screen,
		X:          rect.X + rect.Width - 1,
		ListTopY:   listTopY,
		Visible:    visibleRows,
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
