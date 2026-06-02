package draw

import "github.com/paranoidi/paras-commander/internal/ui/geom"

// Layout and Rect name the shared terminal geometry types for dialog drawing.
type (
	Layout = geom.Layout
	Rect   = geom.Rect
)

// DialogTextX is the screen column for labels, hints, and plain text rows inside a dialog.
// rect.X is the left border; rect.X+1 is the one-space inner margin; text starts at rect.X+2.
func DialogTextX(rect Rect) int { return rect.X + 2 }

// DialogOptionX is the x passed to DrawDialogCheckbox / DrawDialogRadio. Markers include a
// leading space that occupies the inner margin cell so "(" or "[" aligns with DialogTextX.
func DialogOptionX(rect Rect) int { return rect.X + 1 }

// DialogContentWidth is the usable width between the left and right one-space margins.
func DialogContentWidth(rect Rect) int { return rect.Width - 4 }
