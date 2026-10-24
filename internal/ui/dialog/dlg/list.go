package dlg

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/filterlist"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// ViewOffset is the first visible row of a list, derived from its Cursor (and its sticky Offset
// unless CenterScroll). Draw, Place and every list event use this one function, so the viewport
// the caller stores from an event is the viewport that is painted.
func (it Item) ViewOffset() int {
	n := len(it.Rows)
	if n == 0 || it.Height <= 0 {
		return 0
	}
	cursor := min(max(it.Cursor, 0), n-1)
	if it.CenterScroll {
		return geom.ScrollOffset(cursor, it.Height, n)
	}
	return geom.ScrollOffsetEdge(cursor, it.Offset, it.Height, n, 0)
}

// pageStep is how many rows PgUp/PgDn move.
func (it Item) pageStep() int {
	if it.PageStep > 0 {
		return it.PageStep
	}
	return max(it.Height-1, 1)
}

// listMove reports the list at cursor row next as an EventListMove with the viewport that goes
// with it.
func listMove(it Item, next int) Event {
	it.Cursor = next
	return Event{ID: it.ID, Kind: EventListMove, Index: next, Offset: it.ViewOffset()}
}

// listKey applies a list-motion key to it. A focused list handles Up/Down/PgUp/PgDn and
// Home/End (with or without Ctrl); an input driving the list (driven) only the keys that are not
// text editing: Up/Down/PgUp/PgDn and Ctrl+Home/Ctrl+End. An empty list handles nothing, so the
// key moves focus as usual. ok is false for every other key.
func listKey(it Item, ev *tcell.EventKey, driven bool) (Event, bool) {
	n := len(it.Rows)
	if n == 0 {
		return Event{}, false
	}
	cur := min(max(it.Cursor, 0), n-1)
	page := it.pageStep()
	var next int
	switch ev.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn:
		if it.Wrap {
			delta := map[tcell.Key]int{tcell.KeyUp: -1, tcell.KeyDown: 1, tcell.KeyPgUp: -page, tcell.KeyPgDn: page}[ev.Key()]
			next = ((cur+delta)%n + n) % n
		} else {
			next, _ = filterlist.NavKey(ev.Key(), ev.Modifiers(), cur, n, page)
		}
	case tcell.KeyHome, tcell.KeyEnd:
		mods := ev.Modifiers()
		if !driven {
			mods |= tcell.ModCtrl
		}
		var ok bool
		if next, ok = filterlist.NavKey(ev.Key(), mods, cur, n, page); !ok {
			return Event{}, false
		}
	default:
		return Event{}, false
	}
	return listMove(it, next), true
}

// listByID returns the KindList item named id, focusable or not.
func listByID(spec Spec, id ID) (Item, bool) {
	for _, it := range leaves(spec) {
		if it.Kind == KindList && it.ID == id && id != "" {
			return it, true
		}
	}
	return Item{}, false
}

// driverOf returns the input that Drives the list id.
func driverOf(spec Spec, id ID) (ID, bool) {
	for _, it := range leaves(spec) {
		if it.Kind == KindInput && it.Drives == id && id != "" {
			return it.ID, true
		}
	}
	return "", false
}

// activation is the Event for activating the widget id: for a list the row under the cursor, else
// just the ID.
func activation(spec Spec, id ID) Event {
	if p, ok := find(spec, id); ok && !p.isButton() {
		if it := widgetAt(spec, p); it.Kind == KindList {
			return Event{ID: id, Index: min(max(it.Cursor, 0), len(it.Rows)-1)}
		}
	}
	return Event{ID: id}
}

// Scroll scrolls the list named id by delta rows (positive = down), for the mouse wheel. The
// viewport moves and the cursor is pulled to stay inside it; with CenterScroll, whose viewport
// follows the cursor, the cursor itself moves by delta. Focus is not touched. ok is false when
// id is not a list, the list is empty or the viewport cannot move that way.
func Scroll(spec Spec, id ID, delta int) (Event, bool) {
	it, found := listByID(spec, id)
	n := len(it.Rows)
	if !found || n == 0 || delta == 0 {
		return Event{}, false
	}
	cursor := min(max(it.Cursor, 0), n-1)
	if it.CenterScroll {
		next := filterlist.ClampedDelta(cursor, n, delta)
		return listMove(it, next), next != cursor
	}
	off := it.ViewOffset()
	next := min(max(off+delta, 0), max(n-it.Height, 0))
	if next == off {
		return Event{}, false
	}
	it.Offset = next
	it.Cursor = min(max(cursor, next), min(next+it.Height-1, n-1))
	return Event{ID: id, Kind: EventListMove, Index: it.Cursor, Offset: next}, true
}
