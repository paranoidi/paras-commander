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
	return ListViewOffset(it.Cursor, it.Offset, it.Height, it.rowCount(), it.CenterScroll)
}

// rowCount is the number of rows of the list: RowCount for a windowed list, else len(Rows).
func (it Item) rowCount() int {
	if it.RowCount > 0 {
		return it.RowCount
	}
	return len(it.Rows)
}

// row returns list row i (a zero row outside a windowed list's Rows).
func (it Item) row(i int) ListRow {
	if i -= it.RowsFrom; i >= 0 && i < len(it.Rows) {
		return it.Rows[i]
	}
	return ListRow{}
}

// ListWindow is the row range [from, to) a list of n rows shows: the rows a windowed list (see
// Item.RowCount) must build. The list never shows more than height rows from ListViewOffset; a
// placement that shrinks the list shows a subset of them.
func ListWindow(cursor, offset, height, n int) (from, to int) {
	from = ListViewOffset(cursor, offset, height, n, false)
	return from, min(from+max(height, 0), n)
}

// ListViewOffset is Item.ViewOffset for an n-row list known only by its numbers, for callers that
// keep a list's scroll in their own state without building its rows.
func ListViewOffset(cursor, offset, height, n int, center bool) int {
	if n == 0 || height <= 0 {
		return 0
	}
	cursor = min(max(cursor, 0), n-1)
	if center {
		return geom.ScrollOffset(cursor, height, n)
	}
	return geom.ScrollOffsetEdge(cursor, offset, height, n, 0)
}

// cursorRow is the list's Cursor clamped to its rows (0 for an empty list).
func (it Item) cursorRow() int {
	return min(max(it.Cursor, 0), max(it.rowCount()-1, 0))
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
	n := it.rowCount()
	if n == 0 {
		return Event{}, false
	}
	cur := it.cursorRow()
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
	it, ok := leafByID(spec, id)
	return it, ok && it.Kind == KindList
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
			return Event{ID: id, Index: min(max(it.Cursor, 0), it.rowCount()-1)}
		}
	}
	return Event{ID: id}
}
