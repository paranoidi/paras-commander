package dialog

import "github.com/paranoidi/paras-commander/internal/ui/geom"

// Layout and Rect name the shared terminal geometry types for dialog drawing.
// Aliased from internal/ui/geom (same as package ui) so modal entrypoints accept
// the main renderer layout without a distinct type path via internal/draw.
type (
	Layout = geom.Layout
	Rect   = geom.Rect
)
