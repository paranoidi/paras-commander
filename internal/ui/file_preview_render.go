package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// drawFilePreviewPanel paints a file preview panel (quick view, fullscreen, or carousel child).
func drawFilePreviewPanel(screen tcell.Screen, rect Rect, st FilePreviewState, styles theme.Theme, chromeBlocked, previewFocused, quickViewChrome, embedded bool, panelPath, userHomeDir string) {
	previewpanel.Draw(screen, previewpanel.Rect(rect), st, previewpanel.DrawParams{
		Theme:           styles,
		ChromeBlocked:   chromeBlocked,
		PreviewFocused:  previewFocused,
		QuickViewChrome: quickViewChrome,
		Embedded:        embedded,
		PanelPath:       panelPath,
		UserHomeDir:     userHomeDir,
		BodyStyle:       FilePreviewBodyStyle(styles, chromeBlocked),
	})
}
