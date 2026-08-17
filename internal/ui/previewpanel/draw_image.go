package previewpanel

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func drawImageBody(screen tcell.Screen, st State, textX, contentTop, textW, contentH int, body tcell.Style,
	paintLeftMargin, paintRightMargin bool, leftMarginX, rightMarginX int, marginStyle, padStyle tcell.Style,
	scrollGutterX int, borderStyle tcell.Style, p DrawParams,
) {
	hasCaption := strings.TrimSpace(st.CombinedText) != ""
	if !hasCaption {
		drawImageOnly(screen, st, textX, contentTop, textW, contentH,
			paintLeftMargin, paintRightMargin, leftMarginX, rightMarginX, marginStyle, padStyle)
		return
	}

	_, cellH := CellPixelDims(screen)
	if cellH < 1 {
		cellH = fallbackCellPxH
	}
	neededImageRows := (st.ImagePxH + cellH - 1) / cellH
	if neededImageRows < 1 {
		neededImageRows = 1
	}

	lines := previewWrappedLines(st, textW, body)
	// One blank row between metadata and the grid when both fit.
	sep := 1
	captionH := len(lines) + sep
	if captionH < 1 {
		captionH = 1
	}
	if captionH+neededImageRows > contentH {
		// Prefer keeping metadata visible; shrink image / separator as needed.
		if len(lines) >= contentH {
			captionH = contentH
			neededImageRows = 0
			sep = 0
		} else {
			captionH = len(lines) + sep
			if captionH >= contentH {
				sep = 0
				captionH = len(lines)
			}
			neededImageRows = contentH - captionH
			if neededImageRows < 0 {
				neededImageRows = 0
			}
		}
	}

	scroll := st.Scroll
	if scroll < 0 {
		scroll = 0
	}
	textRows := captionH - sep
	if textRows < 0 {
		textRows = 0
	}
	maxStart := max(0, len(lines)-textRows)
	if scroll > maxStart {
		scroll = maxStart
	}

	for row := 0; row < contentH; row++ {
		y := contentTop + row
		if paintLeftMargin {
			screen.SetContent(leftMarginX, y, ' ', nil, marginStyle)
		}
		fillContentRow(screen, textX, y, textW, padStyle)
		if paintRightMargin {
			screen.SetContent(rightMarginX, y, ' ', nil, marginStyle)
		}
	}

	for row := 0; row < textRows; row++ {
		y := contentTop + row
		idx := scroll + row
		if idx < len(lines) {
			drawLine(screen, textX, y, textW, lines[idx], padStyle)
		}
	}

	scrollbarActive := p.PreviewFocused || p.Embedded
	if p.ScrollbarStyle != "" && p.ScrollbarStyle != uiscrollbar.StyleNone && textRows > 0 {
		if metrics, show := uiscrollbar.ComputeMetrics(len(lines), textRows, scroll); show {
			railStyle := borderStyle
			if p.ScrollbarRailStyle != (tcell.Style{}) {
				railStyle = p.ScrollbarRailStyle
			}
			uiscrollbar.Draw(uiscrollbar.DrawParams{
				Screen: screen, X: scrollGutterX, ListTopY: contentTop, Visible: textRows,
				Metrics: metrics, Style: p.ScrollbarStyle, Active: scrollbarActive,
				Blocked: p.ChromeBlocked, FrameStyle: railStyle, Theme: p.Theme,
			})
		}
	}

	if neededImageRows < 1 {
		return
	}
	imageTop := contentTop + captionH
	drawImageOnly(screen, st, textX, imageTop, textW, neededImageRows,
		paintLeftMargin, paintRightMargin, leftMarginX, rightMarginX, marginStyle, padStyle)
}

func drawImageOnly(screen tcell.Screen, st State, textX, top, textW, rows int,
	paintLeftMargin, paintRightMargin bool, leftMarginX, rightMarginX int, marginStyle, padStyle tcell.Style,
) {
	if rows < 1 {
		return
	}
	if st.ImageUnicodePlaceholder {
		drawUnicodePlaceholderImage(screen, st, textX, top, textW, rows,
			paintLeftMargin, paintRightMargin, leftMarginX, rightMarginX, marginStyle, padStyle)
		return
	}
	for row := 0; row < rows; row++ {
		y := top + row
		if paintLeftMargin {
			screen.SetContent(leftMarginX, y, ' ', nil, marginStyle)
		}
		fillContentRow(screen, textX, y, textW, padStyle)
		if paintRightMargin {
			screen.SetContent(rightMarginX, y, ' ', nil, marginStyle)
		}
	}
	frameImage.Store(&ImagePlacement{
		X:        textX,
		Y:        top,
		MaxCols:  textW,
		MaxRows:  rows,
		PxW:      st.ImagePxW,
		PxH:      st.ImagePxH,
		Payload:  st.ImagePayload,
		Path:     st.Path,
		Protocol: st.ImageProtocol,
	})
}
