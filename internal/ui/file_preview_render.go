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
// When quickViewChrome is true, the title shows panelPath on the start and TitleBase on the end (no volume stats).
// When embedded is true, the preview is painted inside a carousel child column (no panel border box).
func drawFilePreviewPanel(screen tcell.Screen, rect Rect, st FilePreviewState, styles theme.Theme, chromeBlocked, previewFocused, quickViewChrome, embedded bool, panelPath, userHomeDir string) {
	chrome := styles.PanelChrome(previewFocused, chromeBlocked)
	embeddedChrome := chrome
	if embedded {
		// Carousel child column lives inside the active panel; match DrawBody side-column chrome.
		embeddedChrome = styles.PanelChrome(true, chromeBlocked)
	}
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	borderStyle := embeddedChrome.Frame
	titleStyle := chrome.Title
	if embedded {
		titleStyle = embeddedChrome.HeaderCarousel
		for x := rect.X; x < rect.X+rect.Width; x++ {
			screen.SetContent(x, rect.Y, ' ', nil, titleStyle)
		}
	} else {
		primitive.Box(screen, primitive.Rect(rect), borderStyle)
		inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
		if inner.Width > 0 && inner.Height > 0 {
			primitive.Fill(screen, inner, ' ', chrome.Surface)
		}
	}
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2
	if embedded {
		titleX = rect.X + 1
		innerRight = rect.X + rect.Width - 2
	}
	contentCols := innerRight - titleX + 1
	if contentCols < 1 {
		contentCols = 1
	}
	if quickViewChrome {
		endLabel := ""
		if tb := strings.TrimSpace(st.TitleBase); tb != "" {
			endLabel = " " + tb + " "
		}
		paintPanelTopTitleRow(screen, titleX, innerRight, contentCols, rect.Y,
			panelPath, userHomeDir, titleStyle, endLabel, titleStyle, borderStyle, false)
	} else {
		title := " Preview "
		if tb := strings.TrimSpace(st.TitleBase); tb != "" {
			title = " " + tb + " "
		}
		titleWidth := rect.Width - 4
		if embedded {
			titleWidth = rect.Width - 2
		}
		primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, titleStyle)
	}

	body := auxPanelBodyText(styles, chromeBlocked, bg)
	contentTop := rect.Y + 1
	contentH := rect.Height - 1
	if !embedded {
		contentH = JobsPanelContentRows(rect)
	}
	if contentH <= 0 {
		return
	}
	textX := rect.X + 2
	textW := rect.Width - 4
	if embedded {
		textX = rect.X + 1
		textW = rect.Width - 2
	}
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

	lines := previewWrappedLines(st, textW, body)
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
	if strings.TrimSpace(combinedText) == "" {
		return 0
	}
	cells := AnsiStyledCells(combinedText, tcell.StyleDefault)
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
