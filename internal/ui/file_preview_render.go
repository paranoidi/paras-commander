package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// drawFilePreviewPanel paints a file preview panel (quick view, fullscreen, or carousel child).
func drawFilePreviewPanel(screen tcell.Screen, rect Rect, st FilePreviewState, styles theme.Theme, chromeBlocked, previewFocused, quickViewChrome, embedded bool, panelPath, userHomeDir, chromaStyleName string) {
	previewpanel.Draw(screen, previewpanel.Rect(rect), st, previewpanel.DrawParams{
		Theme:           styles,
		ChromeBlocked:   chromeBlocked,
		PreviewFocused:  previewFocused,
		QuickViewChrome: quickViewChrome,
		Embedded:        embedded,
		PanelPath:       panelPath,
		UserHomeDir:     userHomeDir,
		BodyStyle:       FilePreviewBodyStyle(styles, chromeBlocked),
		FrameStyle:      filePreviewFrameStyle(styles, previewFocused, chromeBlocked, embedded, chromaStyleName),
	})
}

func filePreviewFrameStyle(styles theme.Theme, previewFocused, chromeBlocked, embedded bool, chromaStyleName string) tcell.Style {
	chrome := styles.PanelChrome(previewFocused, chromeBlocked)
	frame := chrome.Frame
	if embedded {
		frame = styles.PanelChrome(true, chromeBlocked).Frame
	}
	if chromaStyleName != "" && !chromeBlocked {
		frame = chromaformat.FrameStyleFromChroma(frame, chromaStyleName)
	}
	return frame
}
