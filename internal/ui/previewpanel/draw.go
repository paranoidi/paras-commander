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
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
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
	// Borderless draws no box and no styled title bar: the filename sits as plain
	// text on the first row (fullscreen F3 viewer). Mutually exclusive with Embedded/QuickViewChrome.
	Borderless  bool
	PanelPath   string
	UserHomeDir string
	BodyStyle   tcell.Style
	// FrameStyle is the border/box-drawing style; when zero, theme panel frame is used.
	FrameStyle tcell.Style
	// ScrollbarStyle selects the scroll indicator painted on the panel's right edge.
	// Empty or StyleNone paints nothing.
	ScrollbarStyle uiscrollbar.Style
	// ScrollGutterX, when HasScrollGutterX is set, overrides the scrollbar's target column
	// instead of deriving it from rect/mode. Carousel child previews pass the enclosing
	// panel's real border column here, since their own rect stops one column short of it.
	HasScrollGutterX bool
	ScrollGutterX    int
	// ScrollbarRailStyle, when non-zero, styles the scrollbar's non-thumb rail glyph
	// (e.g. a Chroma Comment-token tint), overriding the plain border/frame style.
	ScrollbarRailStyle tcell.Style
}

const gapBeforePanelTitleEnd = 2

// ImageProtocol identifies the terminal graphics protocol used for an image payload.
type ImageProtocol int

const (
	// ImageProtocolNone means no graphics payload (metadata/text fallback).
	ImageProtocolNone ImageProtocol = iota
	// ImageProtocolSixel is DEC sixel (DCS).
	ImageProtocolSixel
	// ImageProtocolKitty is the Kitty graphics protocol (APC _G).
	ImageProtocolKitty
)

// KittyGraphicsImageID is the fixed Kitty image id used for the single on-screen preview.
const KittyGraphicsImageID = 1

// ImagePlacement records where a terminal image should be emitted after Show().
// Draw records at most one per frame; TakeFrameImage reads and clears it.
type ImagePlacement struct {
	X, Y, MaxCols, MaxRows int
	PxW, PxH               int
	Payload, Path          string
	Protocol               ImageProtocol
	// UnicodePlaceholder is true when Draw already painted this image as Kitty Unicode
	// placeholder cells (X/Y/MaxCols/MaxRows are then unused) instead of recording an
	// out-of-band cursor-relative placement; the app layer only needs to (re)transmit
	// Payload in that case, not track a screen position for it.
	UnicodePlaceholder bool
}

// frameImage is the placement recorded by the most recent Draw call in this frame.
// Render is single-goroutine, so a package var avoids threading an out-param through DrawParams.
var frameImage *ImagePlacement

// TakeFrameImage returns and clears the placement recorded by Draw, if any.
func TakeFrameImage() *ImagePlacement {
	p := frameImage
	frameImage = nil
	return p
}

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
	switch {
	case p.Embedded:
		titleStyle = embeddedChrome.HeaderCarousel
		for x := rect.X; x < rect.X+rect.Width; x++ {
			screen.SetContent(x, rect.Y, ' ', nil, titleStyle)
		}
	case p.Borderless:
		// No box; the filename is drawn plain on the first row (see below).
	default:
		primitive.Box(screen, primitive.Rect(rect), borderStyle, primitive.SharpBorder)
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
	} else if p.Borderless {
		name := strings.TrimSpace(st.TitleBase)
		if name == "" {
			name = filepath.Base(st.Path)
		}
		if st.BodyHeld {
			name += string(primitive.Ellipsis)
		}
		statusSuffix := ""
		if st.GitStatusText != "" {
			statusSuffix = " (" + st.GitStatusText + ")"
		}
		// Fill the row with the preview theme (syntax) background, filename centered.
		headerStyle := contentPadStyle(borderStyle, chrome.Surface, p.BodyStyle)
		primitive.Text(screen, rect.X, rect.Y, rect.Width, "", headerStyle)
		full := name + statusSuffix
		start := rect.X + (rect.Width-runewidth.StringWidth(full))/2
		if start < rect.X {
			start = rect.X
		}
		primitive.TextOverlay(screen, start, rect.Y, rect.X+rect.Width-start, name, headerStyle)
		if statusSuffix != "" {
			statusFg, _, _ := p.Theme.PanelGitStyle(st.GitStatusThemeKey).Decompose()
			suffixStyle := headerStyle.Foreground(statusFg)
			suffixX := start + runewidth.StringWidth(name)
			primitive.TextOverlay(screen, suffixX, rect.Y, rect.X+rect.Width-suffixX, statusSuffix, suffixStyle)
		}
	} else {
		title := " Preview "
		if tb := strings.TrimSpace(st.TitleBase); tb != "" {
			if st.BodyHeld {
				title = " " + tb + string(primitive.Ellipsis) + " "
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
	if !p.Embedded && !p.Borderless {
		contentH = geom.JobsPanelContentRows(geom.Rect(rect))
	}
	if contentH <= 0 {
		return
	}
	// Borderless rendered-markdown content gets a 1-space left/right screen margin
	// (matches the width preview.WillRenderMarkdown-driven callers already reserved).
	borderlessMarkdown := p.Borderless && st.IsMarkdown
	textX := rect.X + 2
	textW := rect.Width - 4
	switch {
	case p.Embedded:
		textX = rect.X + 1
		textW = rect.Width - 2
	case borderlessMarkdown:
		textX = rect.X + 1
		textW = rect.Width - 2
	case p.Borderless:
		textX = rect.X
		textW = rect.Width - 1
	}
	if textW < 1 {
		textW = 1
	}

	// Compute margin column positions and style once.
	// Non-embedded: margins at X+1 and X+Width-2 (inside the box border).
	// Embedded: margins at X and X+Width-1 (outer edges; title row is skipped since contentTop=Y+1).
	_, borderBG, _ := borderStyle.Decompose()
	_, surfaceBG, _ := chrome.Surface.Decompose()
	padStyle := contentPadStyle(borderStyle, chrome.Surface, body)
	marginStyle := chrome.Surface
	if borderBG != surfaceBG {
		marginStyle = borderStyle
	}
	var leftMarginX, rightMarginX int
	paintLeftMargin, paintRightMargin := false, false
	if p.Embedded {
		leftMarginX, rightMarginX = rect.X, rect.X+rect.Width-1
		paintLeftMargin, paintRightMargin = rect.Width >= 2, rect.Width >= 2
	} else if borderlessMarkdown {
		// Margin must match the text row background exactly (contentPadStyle), not the
		// chrome/frame color: borderless has no visible frame, so a mismatched fill here
		// reads as a stray border where none is drawn.
		leftMarginX, rightMarginX = rect.X, rect.X+rect.Width-1
		paintLeftMargin, paintRightMargin = rect.Width >= 2, rect.Width >= 2
		marginStyle = padStyle
	} else if p.Borderless {
		// Plain (non-markdown) borderless: reserve a right-only gutter column for the
		// scrollbar; no left margin since the un-boxed text already starts flush at rect.X.
		rightMarginX = rect.X + rect.Width - 1
		paintRightMargin = rect.Width >= 2
		marginStyle = padStyle
	} else {
		leftMarginX, rightMarginX = rect.X+1, rect.X+rect.Width-2
		paintLeftMargin, paintRightMargin = rect.Width >= 4, rect.Width >= 4
	}
	scrollGutterX := rect.X + rect.Width - 1
	if p.Embedded || borderlessMarkdown || p.Borderless {
		scrollGutterX = rightMarginX
	}
	if p.HasScrollGutterX {
		// Caller knows better: e.g. the carousel child preview's own rect stops one
		// column short of the panel's real border (a blank margin column sits between
		// them), so the scrollbar must target the panel's border column instead.
		scrollGutterX = p.ScrollGutterX
	}

	if msg := strings.TrimSpace(st.ErrorMsg); msg != "" {
		errSt := p.Theme.MessageError.Background(bg)
		if p.ChromeBlocked {
			_, efg, _ := p.Theme.MessageError.Decompose()
			errSt = p.Theme.PanelBlockedText.Foreground(efg)
		}
		drawMessageContent(screen, rect, p.Embedded || p.Borderless, contentTop, contentH, textX, textW, msg, errSt, body)
		return
	}

	if st.ExitCode != 0 && !hasDrawableBody(st) {
		line := filepath.Base(st.Path) + ": exit " + itoa(st.ExitCode)
		drawMessageContent(screen, rect, p.Embedded || p.Borderless, contentTop, contentH, textX, textW, line, body, body)
		return
	}

	if st.ImagePayload != "" {
		drawImageBody(screen, st, textX, contentTop, textW, contentH, body,
			paintLeftMargin, paintRightMargin, leftMarginX, rightMarginX, marginStyle, padStyle,
			scrollGutterX, borderStyle, p)
		if !p.Embedded && !p.Borderless {
			paintImageProtocolIndicator(screen, rect, st.ImageProtocol, st.ImageInTmux, p.Theme)
		}
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
		if paintLeftMargin {
			screen.SetContent(leftMarginX, y, ' ', nil, marginStyle)
		}
		idx := scroll + row
		if idx >= len(lines) {
			fillContentRow(screen, textX, y, textW, padStyle)
		} else {
			drawLine(screen, textX, y, textW, lines[idx], padStyle)
		}
		if paintRightMargin {
			screen.SetContent(rightMarginX, y, ' ', nil, marginStyle)
		}
	}

	scrollbarActive := p.PreviewFocused || p.Embedded
	if p.ScrollbarStyle != "" && p.ScrollbarStyle != uiscrollbar.StyleNone {
		if metrics, show := uiscrollbar.ComputeMetrics(len(lines), contentH, scroll); show {
			railStyle := borderStyle
			if p.ScrollbarRailStyle != (tcell.Style{}) {
				railStyle = p.ScrollbarRailStyle
			}
			uiscrollbar.Draw(uiscrollbar.DrawParams{
				Screen: screen, X: scrollGutterX, ListTopY: contentTop, Visible: contentH,
				Metrics: metrics, Style: p.ScrollbarStyle, Active: scrollbarActive,
				Blocked: p.ChromeBlocked, FrameStyle: railStyle, Theme: p.Theme,
			})
		}
	}
}

// paintImageProtocolIndicator overlays a bottom-right label on the panel's border row naming
// the active graphics protocol, but only for Sixel — Kitty and None get no indicator. Sixel
// under tmux has a known display gap Kitty doesn't share (WezTerm's fix for images surviving
// an unrelated tmux redraw sweep currently excludes Sixel — see
// llm-docs/graphics-implementation-lessons.md lesson 11), so that combination gets its own
// louder "Sixel+Tmux" label (styles.PanelStatusImageSixelTmux, red by default) instead of the
// plain "Sixel" one (styles.PanelStatusImageSixel), making a known risk visible instead of
// silent. Caller only invokes this for a boxed panel (not Embedded/Borderless): those have no
// border row outside the image's own drawn area to paint into without overlapping the image.
func paintImageProtocolIndicator(screen tcell.Screen, rect Rect, protocol ImageProtocol, inTmux bool, th theme.Theme) {
	if protocol != ImageProtocolSixel {
		return
	}
	label := " Sixel "
	style := th.PanelStatusImageSixel
	if inTmux {
		label = " Sixel+Tmux "
		style = th.PanelStatusImageSixelTmux
	}
	y := rect.Y + rect.Height - 1
	innerRight := rect.X + rect.Width - 2
	const rightMargin = 1
	runes := []rune(label)
	n := len(runes)
	startX := innerRight - n + 1 - rightMargin
	if startX <= rect.X {
		return
	}
	for i, r := range runes {
		screen.SetContent(startX+i, y, r, nil, style)
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
	// Leave one frame-dash column between the end label and the right border (same as
	// paintAuxPanelTopRow / plain panel title end labels).
	const endRightMargin = 1
	endRunes := utf8.RuneCountInString(endLabel)
	showEnd := endLabel != "" && endRunes > 0 && contentCols >= endRunes+gapBeforePanelTitleEnd+endRightMargin+3
	endStartX := 0
	pathSlotCols := contentCols
	if showEnd {
		endStartX = innerRight - endRunes + 1 - endRightMargin
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
