package previewpanel

import "github.com/gdamore/tcell/v2"

const (
	fallbackCellPxW = 10
	fallbackCellPxH = 20
)

// CellPixelDims returns the terminal cell size in pixels (ioctl), with a 10×20 fallback
// when the screen has no tty or dimensions are unavailable.
func CellPixelDims(screen tcell.Screen) (cw, ch int) {
	tty, ok := screen.Tty()
	if !ok {
		return fallbackCellPxW, fallbackCellPxH
	}
	ws, err := tty.WindowSize()
	if err != nil {
		return fallbackCellPxW, fallbackCellPxH
	}
	cw, ch = ws.CellDimensions()
	if cw <= 0 || ch <= 0 {
		return fallbackCellPxW, fallbackCellPxH
	}
	return cw, ch
}
