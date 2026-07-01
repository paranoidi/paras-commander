package previewpanel

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// Rect is a screen rectangle for preview painting.
type Rect struct {
	X, Y, Width, Height int
}

// DrawParams configures preview panel chrome and body styling.
type DrawParams struct {
	Theme           theme.Theme
	ChromeBlocked   bool
	PreviewFocused  bool
	QuickViewChrome bool
	Embedded        bool
	PanelPath       string
	UserHomeDir     string
	BodyStyle       tcell.Style
	// FrameStyle is the border/box-drawing style; when zero, theme panel frame is used.
	FrameStyle tcell.Style
}

const gapBeforePanelTitleEnd = 2

// BodyStyle is the base text style for preview body rows.
func BodyStyle(styles theme.Theme, chromeBlocked bool) tcell.Style {
	bg := auxPanelContentBG(styles, chromeBlocked)
	if chromeBlocked {
		return styles.PanelBlockedText.Background(bg)
	}
	return styles.PanelText.Background(bg)
}

func auxPanelContentBG(styles theme.Theme, chromeBlocked bool) tcell.Color {
	if chromeBlocked {
		_, bg, _ := styles.PanelBlockedSurface.Decompose()
		return bg
	}
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	return bg
}

// Draw paints a scrollable file preview panel.
func Draw(screen tcell.Screen, rect Rect, st State, p DrawParams) {
	chrome := p.Theme.PanelChrome(p.PreviewFocused, p.ChromeBlocked)
	embeddedChrome := chrome
	if p.Embedded {
		embeddedChrome = p.Theme.PanelChrome(true, p.ChromeBlocked)
	}
	_, bg, _ := p.Theme.PanelActiveSurface.Decompose()
	if p.ChromeBlocked {
		_, bg, _ = p.Theme.PanelBlockedSurface.Decompose()
	}
	borderStyle := p.FrameStyle
	if borderStyle == (tcell.Style{}) {
		borderStyle = embeddedChrome.Frame
	}
	titleStyle := chrome.Title
	if p.Embedded {
		titleStyle = embeddedChrome.HeaderCarousel
		for x := rect.X; x < rect.X+rect.Width; x++ {
			screen.SetContent(x, rect.Y, ' ', nil, titleStyle)
		}
	} else {
		primitive.Box(screen, primitive.Rect(rect), borderStyle)
	}
	titleX := rect.X + 2
	innerRight := rect.X + rect.Width - 2
	if p.Embedded {
		titleX = rect.X + 1
		innerRight = rect.X + rect.Width - 2
	}
	contentCols := innerRight - titleX + 1
	if contentCols < 1 {
		contentCols = 1
	}
	if p.QuickViewChrome {
		endLabel := ""
		if tb := strings.TrimSpace(st.TitleBase); tb != "" {
			endLabel = " " + tb + " "
		}
		paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, rect.Y,
			p.PanelPath, p.UserHomeDir, titleStyle, endLabel, titleStyle, borderStyle)
	} else {
		title := " Preview "
		if tb := strings.TrimSpace(st.TitleBase); tb != "" {
			if st.BodyHeld {
				title = " " + tb + "… "
			} else {
				title = " " + tb + " "
			}
		}
		titleWidth := rect.Width - 4
		if p.Embedded {
			titleWidth = rect.Width - 2
		}
		primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, titleStyle)
	}

	body := p.BodyStyle
	contentTop := rect.Y + 1
	contentH := rect.Height - 1
	if !p.Embedded {
		contentH = geom.JobsPanelContentRows(geom.Rect(rect))
	}
	if contentH <= 0 {
		return
	}
	textX := rect.X + 2
	textW := rect.Width - 4
	if p.Embedded {
		textX = rect.X + 1
		textW = rect.Width - 2
	}
	if textW < 1 {
		textW = 1
	}

	// Compute margin column positions and style once.
	// Non-embedded: margins at X+1 and X+Width-2 (inside the box border).
	// Embedded: margins at X and X+Width-1 (outer edges; title row is skipped since contentTop=Y+1).
	_, borderBG, _ := borderStyle.Decompose()
	_, surfaceBG, _ := chrome.Surface.Decompose()
	marginStyle := chrome.Surface
	if borderBG != surfaceBG {
		marginStyle = borderStyle
	}
	var leftMarginX, rightMarginX int
	paintMargins := false
	if p.Embedded {
		leftMarginX, rightMarginX = rect.X, rect.X+rect.Width-1
		paintMargins = rect.Width >= 2
	} else {
		leftMarginX, rightMarginX = rect.X+1, rect.X+rect.Width-2
		paintMargins = rect.Width >= 4
	}

	if msg := strings.TrimSpace(st.ErrorMsg); msg != "" {
		errSt := p.Theme.MessageError.Background(bg)
		if p.ChromeBlocked {
			_, efg, _ := p.Theme.MessageError.Decompose()
			errSt = p.Theme.PanelBlockedText.Foreground(efg)
		}
		drawMessageContent(screen, rect, p.Embedded, contentTop, contentH, textX, textW, msg, errSt, body)
		return
	}

	if st.ExitCode != 0 && !hasDrawableBody(st) {
		line := filepath.Base(st.Path) + ": exit " + itoa(st.ExitCode)
		drawMessageContent(screen, rect, p.Embedded, contentTop, contentH, textX, textW, line, body, body)
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
	padStyle := contentPadStyle(borderStyle, chrome.Surface, body)
	for row := 0; row < contentH; row++ {
		y := contentTop + row
		if paintMargins {
			screen.SetContent(leftMarginX, y, ' ', nil, marginStyle)
		}
		idx := scroll + row
		if idx >= len(lines) {
			fillContentRow(screen, textX, y, textW, padStyle)
		} else {
			drawLine(screen, textX, y, textW, lines[idx], padStyle)
		}
		if paintMargins {
			screen.SetContent(rightMarginX, y, ' ', nil, marginStyle)
		}
	}
}

func contentPadStyle(borderStyle, surfaceStyle, bodyStyle tcell.Style) tcell.Style {
	_, borderBG, _ := borderStyle.Decompose()
	_, surfaceBG, _ := surfaceStyle.Decompose()
	if borderBG == surfaceBG {
		return bodyStyle
	}
	fg, _, _ := bodyStyle.Decompose()
	return bodyStyle.Foreground(fg).Background(borderBG)
}

func fillContentRow(screen tcell.Screen, x, y, width int, style tcell.Style) {
	for col := 0; col < width; col++ {
		screen.SetContent(x+col, y, ' ', nil, style)
	}
}

// drawMessageContent paints a single status line (e.g. "Not a text file") across an
// otherwise empty preview body. Message states have no chroma-highlighted body, so the
// whole content area — including the side margins — is filled with the panel surface and
// no chroma frame margins are drawn, keeping the message panel fully theme-colored.
func drawMessageContent(screen tcell.Screen, rect Rect, embedded bool, contentTop, contentH, textX, textW int, msg string, msgStyle, surfaceStyle tcell.Style) {
	fillX, fillW := rect.X, rect.Width
	if !embedded {
		fillX, fillW = rect.X+1, rect.Width-2 // stay inside the box border
	}
	if fillW < 0 {
		fillW = 0
	}
	for row := 0; row < contentH; row++ {
		fillContentRow(screen, fillX, contentTop+row, fillW, surfaceStyle)
	}
	primitive.Text(screen, textX, contentTop, textW, msg, msgStyle)
}

func paintQuickViewTitleRow(screen tcell.Screen, titleX, innerRight, contentCols, y int,
	panelPath, userHomeDir string, pathStyle tcell.Style, endLabel string, endStyle, borderStyle tcell.Style) {
	endRunes := utf8.RuneCountInString(endLabel)
	showEnd := endLabel != "" && endRunes > 0 && contentCols >= endRunes+gapBeforePanelTitleEnd+3
	endStartX := 0
	pathSlotCols := contentCols
	if showEnd {
		endStartX = innerRight - endRunes + 1
		pathSlotCols = endStartX - titleX - gapBeforePanelTitleEnd
		if pathSlotCols < 3 {
			showEnd = false
			pathSlotCols = contentCols
		}
	}
	pathMax := pathSlotCols - 2
	if pathMax < 0 {
		pathMax = 0
	}
	left := primitive.TruncateRight(" "+titlePath(panelPath, userHomeDir, pathMax)+" ", pathSlotCols)
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
			st = pathStyle
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

func titlePath(absPath, homeDir string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	display := primitive.PathWithHomeTilde(absPath, homeDir)
	if utf8.RuneCountInString(display) <= maxRunes {
		return display
	}
	return primitive.TruncateRight(display, maxRunes)
}

func drawLine(screen tcell.Screen, x, y, maxW int, cells []AnsiCell, padBase tcell.Style) {
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
