package ui

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// drawFilePreviewPanel paints the inactive-column file preview (ANSI output).
// previewFocused highlights the preview title when keyboard focus is on the preview pane.
func drawFilePreviewPanel(screen tcell.Screen, rect Rect, st FilePreviewState, styles theme.Theme, chromeBlocked, previewFocused bool) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	var borderStyle tcell.Style
	var titleStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if previewFocused {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if previewFocused {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	title := " Preview "
	if tb := strings.TrimSpace(st.TitleBase); tb != "" {
		title = " " + tb + " "
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, titleStyle)

	body := styles.PanelText.Background(bg)
	if chromeBlocked {
		body = styles.PanelBlockedText
	}
	contentTop := rect.Y + 1
	contentH := JobsPanelContentRows(rect)
	if contentH <= 0 {
		return
	}
	textX := rect.X + 2
	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}

	switch st.Phase {
	case FilePreviewPhasePending, FilePreviewPhaseRunning:
		primitive.Text(screen, textX, contentTop, textW, "Loading…", body)
		return
	}

	if msg := strings.TrimSpace(st.ErrorMsg); msg != "" {
		errSt := styles.MessageError.Background(bg)
		if chromeBlocked {
			_, efg, _ := styles.MessageError.Decompose()
			errSt = styles.PanelBlockedText.Foreground(efg)
		}
		primitive.Text(screen, textX, contentTop, textW, msg, errSt)
		return
	}

	if st.ExitCode != 0 && strings.TrimSpace(st.CombinedText) == "" {
		line := filepath.Base(st.Path) + ": exit " + itoa(st.ExitCode)
		primitive.Text(screen, textX, contentTop, textW, line, body)
		return
	}

	cells := AnsiStyledCells(st.CombinedText, body)
	lines := WrapAnsiCells(cells, textW)
	scroll := st.Scroll
	if scroll < 0 {
		scroll = 0
	}
	maxStart := max(0, len(lines)-contentH)
	if scroll > maxStart {
		scroll = maxStart
	}
	for row := 0; row < contentH; row++ {
		y := contentTop + row
		idx := scroll + row
		if idx >= len(lines) {
			break
		}
		drawPreviewLine(screen, textX, y, textW, lines[idx], body)
	}
}

func drawPreviewLine(screen tcell.Screen, x, y, maxW int, cells []AnsiCell, padBase tcell.Style) {
	col := 0
	for _, c := range cells {
		rw := runewidth.RuneWidth(c.R)
		if rw < 1 {
			rw = 1
		}
		if col+rw > maxW {
			break
		}
		screen.SetContent(x+col, y, c.R, nil, c.St)
		col += rw
	}
	pad := padBase
	if len(cells) > 0 {
		pad = cells[len(cells)-1].St
	}
	for col < maxW {
		screen.SetContent(x+col, y, ' ', nil, pad)
		col++
	}
}

// FilePreviewTotalLines returns how many wrapped lines the preview body would use for vertical scrolling.
func FilePreviewTotalLines(combinedText string, textWidth int) int {
	if textWidth < 1 {
		textWidth = 1
	}
	t := strings.TrimSpace(combinedText)
	if t == "" {
		return 0
	}
	cells := AnsiStyledCells(t, tcell.StyleDefault)
	return len(WrapAnsiCells(cells, textWidth))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
