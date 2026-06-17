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
	Active          bool
	ChromeBlocked   bool
	PreviewFocused  bool
	QuickViewChrome bool
	Embedded        bool
	PanelPath       string
	UserHomeDir     string
	BodyStyle       tcell.Style
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
	borderStyle := embeddedChrome.Frame
	titleStyle := chrome.Title
	if p.Embedded {
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
			if st.BodyHeld {
				endLabel = " " + tb + "… "
			} else {
				endLabel = " " + tb + " "
			}
		}
		paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, rect.Y,
			p.PanelPath, p.UserHomeDir, titleStyle, endLabel, titleStyle)
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

	if msg := strings.TrimSpace(st.ErrorMsg); msg != "" {
		errSt := p.Theme.MessageError.Background(bg)
		if p.ChromeBlocked {
			_, efg, _ := p.Theme.MessageError.Decompose()
			errSt = p.Theme.PanelBlockedText.Foreground(efg)
		}
		primitive.Text(screen, textX, contentTop, textW, msg, errSt)
		return
	}

	if st.ExitCode != 0 && !hasDrawableBody(st) {
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
		drawLine(screen, textX, y, textW, lines[idx], body)
	}
}

func paintQuickViewTitleRow(screen tcell.Screen, titleX, innerRight, contentCols, y int,
	panelPath, userHomeDir string, pathStyle tcell.Style, endLabel string, endStyle tcell.Style) {
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
	left := " " + titlePath(panelPath, userHomeDir, pathMax) + " "
	primitive.TextOverlay(screen, titleX, y, pathSlotCols, left, pathStyle)
	if !showEnd {
		return
	}
	primitive.TextOverlay(screen, endStartX, y, endRunes, endLabel, endStyle)
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
