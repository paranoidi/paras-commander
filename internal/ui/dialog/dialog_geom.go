package dialog

import "github.com/paranoidi/paras-commander/internal/ui/geom"

// Layout and Rect name the shared terminal geometry types for dialog drawing.
// Aliased from internal/ui/geom (same as package ui) so modal entrypoints accept
// the main renderer layout without a distinct type path via internal/draw.
type (
	Layout = geom.Layout
	Rect   = geom.Rect
)

// PaintedArea is the screen area a dialog overlay painted: its frame and a popup drawn outside
// it (a path-completion dropdown; zero when none). A partial repaint that would paint a
// different area than the screen shows must give way to a full render, or what the old area
// covered stays on screen.
type PaintedArea struct {
	Frame, Popup Rect
}
