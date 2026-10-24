package dlg

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// pos locates a widget: group/col/row for an item, or button >= 0 for a button.
type pos struct{ group, col, row, button int }

func (p pos) isButton() bool { return p.button >= 0 }

func find(spec Spec, id ID) (pos, bool) {
	for gi, g := range spec.Groups {
		for ci, col := range g.Columns {
			for ri, it := range col {
				if it.focusable() && it.ID == id {
					return pos{gi, ci, ri, -1}, true
				}
			}
		}
	}
	for bi, b := range spec.Buttons {
		if b.ID == id {
			return pos{button: bi}, true
		}
	}
	return pos{}, false
}

// itemAt returns the item at an item position (not a button).
func itemAt(spec Spec, p pos) Item { return spec.Groups[p.group].Columns[p.col][p.row] }

// isInput reports whether the focused widget p is a text input.
func isInput(spec Spec, p pos) bool { return !p.isButton() && itemAt(spec, p).Kind == KindInput }

// nextFocusable is the first focusable item of col strictly after row from (from -1 starts at the
// top), and prevFocusable the last one strictly before row from.
func nextFocusable(col []Item, from int) (ID, bool) {
	for i := from + 1; i < len(col); i++ {
		if col[i].focusable() {
			return col[i].ID, true
		}
	}
	return "", false
}

func prevFocusable(col []Item, from int) (ID, bool) {
	for i := from - 1; i >= 0; i-- {
		if col[i].focusable() {
			return col[i].ID, true
		}
	}
	return "", false
}

// groupFirst is the first focusable of group g in column-major order.
func groupFirst(g Group) (ID, bool) {
	for _, col := range g.Columns {
		if id, ok := nextFocusable(col, -1); ok {
			return id, true
		}
	}
	return "", false
}

// col0Last is the last focusable item of column 0 of group g.
func col0Last(g Group) (ID, bool) {
	if len(g.Columns) == 0 {
		return "", false
	}
	return prevFocusable(g.Columns[0], len(g.Columns[0]))
}

// groupFocusables lists the focusable items of g in column-major order.
func groupFocusables(g Group) []Item {
	var out []Item
	for _, col := range g.Columns {
		for _, it := range col {
			if it.focusable() {
				out = append(out, it)
			}
		}
	}
	return out
}

// ownSegment reports whether the item is a Tab segment of its own: every text input and list is.
func ownSegment(it Item) bool { return it.Kind == KindInput || it.Kind == KindList }

// segmentTable returns the first focusable of each Tab segment and the segment index of every
// focusable item. A group is one segment, except that every text input and list is a segment of
// its own (also when adjacent) and splits the group around it. The buttons are the last segment.
func segmentTable(spec Spec) (starts []ID, index map[ID]int) {
	index = map[ID]int{}
	for _, g := range spec.Groups {
		prevInput := false
		for i, it := range groupFocusables(g) {
			if i == 0 || ownSegment(it) || prevInput {
				starts = append(starts, it.ID)
			}
			prevInput = ownSegment(it)
			index[it.ID] = len(starts) - 1
		}
	}
	if len(spec.Buttons) > 0 {
		starts = append(starts, spec.Buttons[0].ID)
		for _, b := range spec.Buttons {
			index[b.ID] = len(starts) - 1
		}
	}
	return starts, index
}

func segments(spec Spec) []ID {
	starts, _ := segmentTable(spec)
	return starts
}

// resolve maps an empty or unknown focus to the first focusable.
func resolve(spec Spec, focus ID) ID {
	if _, ok := find(spec, focus); ok {
		return focus
	}
	if segs := segments(spec); len(segs) > 0 {
		return segs[0]
	}
	return focus
}

// nextGroupFirst is the first item of the first non-empty group after g, else the first button.
func nextGroupFirst(spec Spec, g int) (ID, bool) {
	for gi := g + 1; gi < len(spec.Groups); gi++ {
		if id, ok := groupFirst(spec.Groups[gi]); ok {
			return id, true
		}
	}
	if len(spec.Buttons) > 0 {
		return spec.Buttons[0].ID, true
	}
	return "", false
}

// prevGroupLast is the last item of column 0 of the closest non-empty group before g.
func prevGroupLast(spec Spec, g int) (ID, bool) {
	for gi := g - 1; gi >= 0; gi-- {
		if id, ok := col0Last(spec.Groups[gi]); ok {
			return id, true
		}
	}
	return "", false
}

// HandleKey applies one key to focus. Navigation (arrows, Tab/Backtab) returns (Event{}, true).
// Space and Enter return Event{ID: focus}; Alt+mnemonic focuses the widget and returns its Event;
// with Spec.PlainButtonKeys a plain button mnemonic letter returns that button's Event without
// moving focus. Anything else returns (Event{}, false). OK / Cancel / Esc chords are resolved by
// the application before dialog keys reach here and are not handled.
//
// Text inputs (KindInput) are edited by the caller, not the engine. While an input is focused:
//   - Up/Down and Tab/Backtab move focus as usual, and every input is a Tab segment of its own,
//     also when inputs are adjacent;
//   - Enter returns Event{ID: input}, which the caller treats as submit / OK;
//   - Alt+mnemonic of a checkbox or radio returns that widget's Event but leaves focus in the
//     input, so the user keeps typing (a button mnemonic still focuses the button);
//   - Space, plain letters (also PlainButtonKeys letters), Left/Right, Home/End, Backspace,
//     Delete and every other key return (Event{}, false): the caller feeds them to its text
//     editing code (lineedit, scrollquery, path completion...) and keeps Value/Cursor/Scroll in its
//     own state.
//
// Lists (KindList) own their motion, so the keyboard and Scroll (the mouse wheel) share one
// implementation, and report it as EventListMove events carrying the new cursor row (Index) and
// first visible row (Offset), which the caller stores. While a list is focused:
//   - Up/Down move the cursor one row and PgUp/PgDn by Item.PageStep (default Height-1); Home/End
//     (with or without Ctrl) jump to the first / last row. The cursor stops at the ends (or wraps
//     when Item.Wrap), and the key is consumed even there: Up on the first row never leaves the
//     list. An empty list is never focused (it is skipped by every focus move);
//   - Enter and Space return an EventActivate event for the list with Index = the cursor row;
//   - Tab/Backtab leave the list (every list is a Tab segment of its own); Left/Right do nothing;
//   - plain letters return (Event{}, false) unless Spec.PlainButtonKeys, so a type-to-jump
//     handler of the caller keeps working; Alt+mnemonic focuses and activates its widget.
//
// An input with Item.Drives naming a list steers it while the input is focused and the list has
// rows: Up/Down/PgUp/PgDn and Ctrl+Home/Ctrl+End return the list's EventListMove and focus stays in
// the input (plain Home/End stay text editing keys, so they return (Event{}, false)). With an
// empty list those keys move focus like for any input. Enter on the input still returns
// Event{ID: input}; the caller reads the list cursor from its own state.
func HandleKey(spec Spec, focus *ID, ev *tcell.EventKey) (Event, bool) {
	*focus = resolve(spec, *focus)
	cur, ok := find(spec, *focus)
	if !ok {
		return Event{}, false
	}
	move := func(id ID, found bool) (Event, bool) {
		if found {
			*focus = id
		}
		return Event{}, true
	}
	if !cur.isButton() {
		switch it := itemAt(spec, cur); {
		case it.Kind == KindList:
			if e, ok := listKey(it, ev, false); ok {
				return e, true
			}
		case it.Kind == KindInput && it.Drives != "":
			if list, ok := listByID(spec, it.Drives); ok {
				if e, ok := listKey(list, ev, true); ok {
					return e, true
				}
			}
		}
	}
	switch ev.Key() {
	case tcell.KeyEnter:
		return activation(spec, *focus), true
	case tcell.KeyTab, tcell.KeyBacktab:
		segs := segments(spec)
		step := 1
		if ev.Key() == tcell.KeyBacktab {
			step = -1
		}
		_, index := segmentTable(spec)
		i := index[*focus]
		return move(segs[(i+step+len(segs))%len(segs)], true)
	case tcell.KeyDown:
		if cur.isButton() {
			return Event{}, true
		}
		if id, ok := nextFocusable(spec.Groups[cur.group].Columns[cur.col], cur.row); ok {
			return move(id, true)
		}
		return move(nextGroupFirst(spec, cur.group))
	case tcell.KeyUp:
		if cur.isButton() {
			return move(prevGroupLast(spec, len(spec.Groups)))
		}
		if id, ok := prevFocusable(spec.Groups[cur.group].Columns[cur.col], cur.row); ok {
			return move(id, true)
		}
		if cur.col == 0 {
			return move(prevGroupLast(spec, cur.group))
		}
		return Event{}, true
	case tcell.KeyLeft, tcell.KeyRight:
		if isInput(spec, cur) {
			return Event{}, false // the caller moves the text cursor
		}
		d := 1
		if ev.Key() == tcell.KeyLeft {
			d = -1
		}
		if cur.isButton() {
			if b := cur.button + d; b >= 0 && b < len(spec.Buttons) {
				*focus = spec.Buttons[b].ID
			}
			return Event{}, true
		}
		cols := spec.Groups[cur.group].Columns
		if c := cur.col + d; c >= 0 && c < len(cols) && cur.row < len(cols[c]) && cols[c][cur.row].focusable() {
			*focus = cols[c][cur.row].ID
		}
		return Event{}, true
	case tcell.KeyRune:
		return handleRune(spec, focus, isInput(spec, cur), ev)
	}
	return Event{}, false
}

func handleRune(spec Spec, focus *ID, inInput bool, ev *tcell.EventKey) (Event, bool) {
	r := unicode.ToLower(ev.Rune())
	if keymap.AltLetterModifiers(ev.Modifiers()) {
		for _, g := range spec.Groups {
			for _, col := range g.Columns {
				for _, it := range col {
					if it.Mnemonic != 0 && unicode.ToLower(it.Mnemonic) == r {
						if !inInput { // a flipped option never pulls focus out of a text input
							*focus = it.ID
						}
						return Event{ID: it.ID}, true
					}
				}
			}
		}
		for _, b := range spec.Buttons {
			if b.Mnemonic != 0 && unicode.ToLower(b.Mnemonic) == r {
				*focus = b.ID
				return Event{ID: b.ID}, true
			}
		}
		return Event{}, false
	}
	if ev.Modifiers() != tcell.ModNone {
		return Event{}, false
	}
	if inInput {
		return Event{}, false // plain letters and Space are text
	}
	if ev.Rune() == ' ' {
		return activation(spec, *focus), true
	}
	if spec.PlainButtonKeys {
		for _, b := range spec.Buttons {
			if b.Mnemonic != 0 && unicode.ToLower(b.Mnemonic) == r {
				return Event{ID: b.ID}, true
			}
		}
	}
	return Event{}, false
}

// Click focuses the widget under (x, y) and activates it like Space. ok is false on a miss. A click
// on a list row focuses the list and returns an EventListMove to that row (rows past the last
// row are misses); on an input it returns the clicked column.
func Click(spec Spec, screenW, screenH int, focus *ID, x, y int) (Event, bool) {
	p, placed := Place(screenW, screenH, spec)
	if !placed {
		return Event{}, false
	}
	id, hit := HitTest(p, x, y)
	if !hit {
		return Event{}, false
	}
	*focus = id
	ev := Event{ID: id}
	if it, ok := p.Item(id); ok {
		switch it.Kind {
		case KindInput:
			ev.Col = x - it.Rect.X
		case KindList:
			// The caller decides whether a click also activates the row.
			if list, ok := listByID(spec, id); ok {
				if driver, ok := driverOf(spec, id); ok && list.Driven {
					*focus = driver // a driven list has no focus of its own
				}
				ev = listMove(list, it.Offset+(y-it.Rect.Y))
			}
		}
	}
	return ev, true
}
