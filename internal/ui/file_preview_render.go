package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// drawFilePreviewPanel paints a file preview panel (quick view, fullscreen, or carousel child).
// scrollGutterX overrides the scrollbar's target column when >= 0 (see previewpanel.DrawParams);
// pass -1 to let previewpanel derive it from rect/mode. scrollbarRailStyle overrides the
// scrollbar's non-thumb rail color when non-zero (e.g. the carousel child preview passes the
// enclosing panel's own border color, since its embedded chrome is otherwise Chroma-tinted and
// would otherwise mismatch the border column the rail is painted on); pass a zero tcell.Style
// to use the mode's default (frame color, or a dimmed Chroma "Comment" tint in fullscreen).
func drawFilePreviewPanel(screen tcell.Screen, rect Rect, st FilePreviewState, styles theme.Theme, chromeBlocked, previewFocused, quickViewChrome, embedded, borderless bool, panelPath, userHomeDir string, scrollbarStyle uiscrollbar.Style, scrollGutterX int, scrollbarRailStyle tcell.Style) {
	// Use the style stored with the content so border and body always match.
	// ErrorMsg states have no syntax-highlighted body, so suppress chroma border tint.
	chromaStyleName := st.ChromaStyle
	if strings.TrimSpace(st.ErrorMsg) != "" {
		chromaStyleName = ""
	}
	frame := filePreviewFrameStyle(styles, previewFocused, chromeBlocked, embedded, chromaStyleName)
	// Fullscreen's scrollbar rail reads as a dimmed Chroma "Comment" tint rather than the
	// full frame color, so it doesn't compete visually with the syntax-highlighted body.
	railStyle := frame
	if borderless && chromaStyleName != "" && !chromeBlocked {
		railStyle = chromaformat.CommentFrameStyle(frame, chromaStyleName)
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
