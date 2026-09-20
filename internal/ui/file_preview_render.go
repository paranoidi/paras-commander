package ui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// fullscreenPreviewRects derives the fullscreen preview panel and (when open) theme-picker
// rects from layout+model. Shared by Render's ViewFilePreview branch and the partial spinner
// painter so both agree on where row 0's trailing edge is.
func fullscreenPreviewRects(layout Layout, model Model) (previewRect, pickerRect Rect) {
	union := MergeTwinPanelRects(layout.Primary, layout.Secondary, model.SplitOrientation)
	return SplitFullscreenPreviewRects(union, model.FilePreviewThemePicker.Open, model.FilePreviewThemePicker.Choices)
}

// filePreviewChromaStyleName returns st's Chroma style name, blanked when the preview shows an
// error message (which has no syntax-highlighted body, so border/spinner chrome tint is suppressed).
func filePreviewChromaStyleName(st FilePreviewState) string {
	if strings.TrimSpace(st.ErrorMsg) != "" {
		return ""
	}
	return st.ChromaStyle
}

// drawFullscreenPreviewSpinner draws the activity spinner at the trailing edge of the fullscreen
// preview's filename row — the only menu-bar element that survives in ViewFilePreview, since
// MenuBarLayoutReserved is false there and no other menu-bar content is painted. Returns false
// (nothing drawn) when the spinner isn't active or the F9 theme picker is open.
func drawFullscreenPreviewSpinner(screen tcell.Screen, layout Layout, model Model, styles theme.Theme) bool {
	if model.ViewMode != ViewFilePreview || !model.MenuBarActivitySpinner || model.FilePreviewThemePicker.Open {
		return false
	}
	previewRect, _ := fullscreenPreviewRects(layout, model)
	x, ok := menuBarSpinnerX(previewRect)
	if !ok {
		return false
	}
	chromeBlocked := model.PanelsChromeBlocked()
	chromaStyleName := filePreviewChromaStyleName(model.FullscreenFilePreviewDraw)
	frame := filePreviewFrameStyle(styles, true, chromeBlocked, false, chromaStyleName)
	header := previewpanel.ContentPadStyle(frame, styles.PanelChrome(true, chromeBlocked).Surface, FilePreviewBodyStyle(styles, chromeBlocked))
	fg, _, attrs := styles.MenuSpinner.Decompose()
	style := chromaformat.TokenFrameStyle(header.Foreground(fg).Attributes(attrs), chromaStyleName, chroma.LiteralNumber)
	screen.SetContent(x, previewRect.Y, MenuBarSpinnerIcon(model.SpinPhase), nil, style)
	return true
}

// drawFilePreviewPanel paints a file preview panel (quick view, fullscreen, or carousel child).
// scrollGutterX overrides the scrollbar's target column when >= 0 (see previewpanel.DrawParams);
// pass -1 to let previewpanel derive it from rect/mode. scrollbarRailStyle overrides the
// scrollbar's non-thumb rail color when non-zero (e.g. the carousel child preview passes the
// enclosing panel's own border color, since its embedded chrome is otherwise Chroma-tinted and
// would otherwise mismatch the border column the rail is painted on); pass a zero tcell.Style
// to use the mode's default (frame color, or a dimmed Chroma "Comment" tint in fullscreen).
func drawFilePreviewPanel(screen tcell.Screen, rect Rect, st FilePreviewState, styles theme.Theme, chromeBlocked, previewFocused, quickViewChrome, embedded, borderless bool, panelPath, userHomeDir string, scrollbarStyle uiscrollbar.Style, scrollGutterX int, scrollbarRailStyle tcell.Style) {
	// Use the style stored with the content so border and body always match.
	chromaStyleName := filePreviewChromaStyleName(st)
	frame := filePreviewFrameStyle(styles, previewFocused, chromeBlocked, embedded, chromaStyleName)
	// Fullscreen's scrollbar rail reads as a dimmed Chroma "Comment" tint rather than the
	// full frame color, so it doesn't compete visually with the syntax-highlighted body.
	railStyle := frame
	if borderless && chromaStyleName != "" && !chromeBlocked {
		railStyle = chromaformat.TokenFrameStyle(frame, chromaStyleName, chroma.Comment)
	}
	if scrollbarRailStyle != (tcell.Style{}) {
		railStyle = scrollbarRailStyle
	}
	previewpanel.Draw(screen, previewpanel.Rect(rect), st, previewpanel.DrawParams{
		Theme:              styles,
		ChromeBlocked:      chromeBlocked,
		PreviewFocused:     previewFocused,
		QuickViewChrome:    quickViewChrome,
		Embedded:           embedded,
		Borderless:         borderless,
		PanelPath:          panelPath,
		UserHomeDir:        userHomeDir,
		BodyStyle:          FilePreviewBodyStyle(styles, chromeBlocked),
		FrameStyle:         frame,
		ScrollbarStyle:     scrollbarStyle,
		HasScrollGutterX:   scrollGutterX >= 0,
		ScrollGutterX:      scrollGutterX,
		ScrollbarRailStyle: railStyle,
	})
}

func filePreviewFrameStyle(styles theme.Theme, previewFocused, chromeBlocked, embedded bool, chromaStyleName string) tcell.Style {
	chrome := styles.PanelChrome(previewFocused, chromeBlocked)
	frame := chrome.Frame
	if embedded {
		frame = styles.PanelChrome(true, chromeBlocked).Frame
	}
	// Applied regardless of chromeBlocked: the body's already-rendered chroma cells keep
	// their real background even while a dialog blocks the panel (FilePreviewBodyStyle/
	// pad/margin styles don't re-derive it), so suppressing it here made the border/empty
	// rows/margins fall back to the plain theme background and visibly mismatch the
	// still-chroma-tinted text underneath.
	if chromaStyleName != "" {
		frame = chromaformat.FrameStyleFromChroma(frame, chromaStyleName)
	}
	return frame
}
