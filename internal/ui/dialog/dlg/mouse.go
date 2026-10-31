package dlg

import (
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// Click applies a left press at screen cell (x, y) to spec placed on a screenW x screenH screen,
// as Draw places it. Focus moves to the widget under the pointer (for a row of a driven list, to
// the input that drives it) and the returned Event is the one the matching key would produce, so
// the caller applies it with the same code as HandleKey's events:
//   - a button or a radio / checkbox: its activation (Event.Enter unset: a click toggles);
//   - an input: an EventCaret with the rune index of the clicked text cell;
//   - a list row: the EventListMove onto that row, or, when double and the row already holds the
//     cursor, the activation Enter produces in that list (or in its driving input).
//
// ok is false on a miss: labels, separators, spacers (reserved regions), bars, disabled widgets,
// ScrollOnly lists, Header rows and list rows past the last row are not targets, and focus is
// left alone.
func Click(spec Spec, screenW, screenH int, focus *ID, x, y int, double bool) (Event, bool) {
	pi, it, ok := hit(spec, screenW, screenH, x, y)
	if !ok {
		return Event{}, false
	}
	row := pi.Offset + (y - pi.Rect.Y)
	if pi.Kind == KindList && (it.ScrollOnly || it.row(row).Header) {
		return Event{}, false
	}
	*focus = pi.ID
	switch pi.Kind {
	case KindList:
		*focus = listFocus(spec, it)
		if double && row == it.cursorRow() {
			e := activation(spec, *focus)
			e.Enter = true
			return e, true
		}
		return listMove(it, row), true
	case KindInput:
		col := x - pi.Rect.X
		if !it.Simple {
			col -= draw.LeadingIconCols(it.LeadingIcon)
		}
		return Event{ID: pi.ID, Kind: EventCaret, Index: caretIndex(it, col)}, true
	}
	return Event{ID: pi.ID}, true
}

// Wheel applies delta rows of mouse wheel at (x, y) to the list under the pointer: focus moves to
// the list (or its driving input) and the result is the list's EventListMove, clamped to its
// rows. A ScrollOnly list scrolls by delta rows and focus stays. ok is false when no list with
// rows is under the pointer.
func Wheel(spec Spec, screenW, screenH int, focus *ID, x, y, delta int) (Event, bool) {
	pi, it, ok := hit(spec, screenW, screenH, x, y)
	if !ok || pi.Kind != KindList || it.rowCount() == 0 {
		return Event{}, false
	}
	if it.ScrollOnly {
		return scrollTo(it, it.ViewOffset()+delta), true
	}
	*focus = listFocus(spec, it)
	next := min(max(it.cursorRow()+delta, 0), it.rowCount()-1)
	return listMove(it, it.landOnRow(next, delta)), true
}

// hit places spec and returns the placed widget under (x, y) and its spec item (zero for a button).
func hit(spec Spec, screenW, screenH, x, y int) (PlacedItem, Item, bool) {
	p, placed := Place(screenW, screenH, spec)
	if !placed {
		return PlacedItem{}, Item{}, false
	}
	id, ok := HitTest(p, x, y)
	if !ok {
		return PlacedItem{}, Item{}, false
	}
	pi, _ := p.Item(id)
	it, _ := spec.Item(id)
	return pi, it, true
}

// listFocus is the widget that takes focus for a press on list it: the list, or for a driven
// list (which has no focus of its own) the input that drives it.
func listFocus(spec Spec, it Item) ID {
	if driver, ok := driverOf(spec, it.ID); ok && it.Driven {
		return driver
	}
	return it.ID
}

// caretIndex is the rune index of input it's text under cell col of its text area (0 = the first
// visible cell, Scroll runes into the value), clamped to the end of the value.
func caretIndex(it Item, col int) int {
	runes := []rune(it.Value)
	i := min(max(it.Scroll, 0), len(runes))
	for w := 0; i < len(runes); i++ {
		rw := max(runewidth.RuneWidth(runes[i]), 1)
		if w+rw > col {
			break
		}
		w += rw
	}
	return i
}

// ButtonHover is the strip button under the mouse pointer, as Place laid it out. Active is false
// when the pointer is on no button.
type ButtonHover struct {
	Active      bool
	ID          ID
	Rect        Rect
	Label       string
	Mnemonic    rune
	Destructive bool
}

// HoveredButton returns the strip button of spec under (x, y), placed as Draw places it.
func HoveredButton(spec Spec, screenW, screenH, x, y int) ButtonHover {
	pi, _, ok := hit(spec, screenW, screenH, x, y)
	if !ok {
		return ButtonHover{}
	}
	for _, b := range spec.Buttons {
		if b.ID == pi.ID {
			return ButtonHover{Active: true, ID: b.ID, Rect: pi.Rect, Label: b.Label, Mnemonic: b.Mnemonic, Destructive: b.Destructive}
		}
	}
	return ButtonHover{}
}

// DrawButtonHover repaints the hovered button h in the focused button style, over the strip Draw
// painted; focus itself does not move.
func DrawButtonHover(screen tcell.Screen, h ButtonHover, styles theme.Theme) {
	if !h.Active {
		return
	}
	draw.DrawDialogButton(screen, h.Rect.X, h.Rect.Y, h.Label, h.Mnemonic, true, h.Destructive, false, styles)
}
