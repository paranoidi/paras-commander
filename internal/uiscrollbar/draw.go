package uiscrollbar

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DrawParams configures vertical scrollbar painting on a list column edge.
type DrawParams struct {
	Screen     tcell.Screen
	X          int
	ListTopY   int
	Visible    int
	Metrics    Metrics
	Style      Style
	Active     bool
	Blocked    bool
	FrameStyle tcell.Style
	Theme      theme.Theme
}

// Draw paints a vertical scroll indicator on column x for rows [listTopY, listTopY+visible).
func Draw(p DrawParams) {
	style := EffectiveStyle(p.Style)
	if style == StyleNone {
		return
	}
	track, thumb := p.Theme.PanelScrollbarStyles(p.Active, p.Blocked)
	switch style {
	case StyleThumb:
		drawThumb(p, thumb, p.FrameStyle)
	case StyleBar:
		drawBar(p, track, thumb)
	}
}

func drawThumb(p DrawParams, thumbStyle, frameStyle tcell.Style) {
	m := p.Metrics
	thumbRow := m.ThumbStart + m.ThumbSize/2
	if thumbRow < 0 {
		thumbRow = 0
	}
	if thumbRow >= m.Visible {
		thumbRow = m.Visible - 1
	}
	for row := 0; row < m.Visible; row++ {
		y := p.ListTopY + row
		if row == thumbRow {
			p.Screen.SetContent(p.X, y, p.Theme.SymbolScrollbarThumb(), nil, thumbStyle)
		} else {
			p.Screen.SetContent(p.X, y, '│', nil, frameStyle)
		}
	}
}

func drawBar(p DrawParams, trackStyle, thumbStyle tcell.Style) {
	m := p.Metrics
	thumbEnd := m.ThumbStart + m.ThumbSize
	for row := 0; row < m.Visible; row++ {
		y := p.ListTopY + row
		if row >= m.ThumbStart && row < thumbEnd {
			p.Screen.SetContent(p.X, y, '█', nil, thumbStyle)
		} else {
			p.Screen.SetContent(p.X, y, '░', nil, trackStyle)
		}
	}
}
