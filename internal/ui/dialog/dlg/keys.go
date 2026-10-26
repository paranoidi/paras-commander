package dlg

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

// pos locates a widget: group/col/row for an item, plus cell >= 0 when it is a cell of a KindRow
// item (row is then the row's index in the column), or button >= 0 for a button.
type pos struct{ group, col, row, cell, button int }

func (p pos) isButton() bool { return p.button >= 0 }

func itemPos(group, col, row, cell int) pos {
	return pos{group: group, col: col, row: row, cell: cell, button: -1}
}

func find(spec Spec, id ID) (pos, bool) {
	for gi, g := range spec.Groups {
		for ci, col := range g.Columns {
			for ri, it := range col {
				if it.Kind == KindRow {
					for ki, c := range it.Cells {
						if c.Focusable() && c.ID == id {
							return itemPos(gi, ci, ri, ki), true
						}
					}
					continue
				}
				if it.Focusable() && it.ID == id {
					return itemPos(gi, ci, ri, -1), true
				}
			}
		}
	}
	for bi, b := range spec.Buttons {
		if b.ID == id {
			return pos{cell: -1, button: bi}, true
		}
	}
	return pos{}, false
}

// itemAt returns the item at an item position (not a button): for a row cell, the row.
func itemAt(spec Spec, p pos) Item { return spec.Groups[p.group].Columns[p.col][p.row] }

// widgetAt returns the widget at an item position: the cell for a row cell, else the item.
func widgetAt(spec Spec, p pos) Item {
	it := itemAt(spec, p)
	if p.cell >= 0 {
		return it.Cells[p.cell]
	}
	return it
}

// isInput reports whether the focused widget p is a text input.
func isInput(spec Spec, p pos) bool { return !p.isButton() && widgetAt(spec, p).Kind == KindInput }

// enterRow returns the focusable cell of a row that focus lands on when it moves onto the row.
// idx is the cell index of the widget focus leaves (-1 when that is not a row cell): the cell with
// that index, clamped to the row, else the nearest focusable cell to its left, else to its right
// (with FallbackFirst, else the row's first focusable cell).
// From a widget that is not a row cell, the first focusable cell, or the last one for Up into a
// row with EntryLast.
func enterRow(row Item, idx int, up bool) (ID, bool) {
	n := len(row.Cells)
	if idx < 0 {
		if up && row.EntryLast {
			idx = n - 1
		} else {
			for _, c := range row.Cells {
				if c.Focusable() {
					return c.ID, true
				}
			}
			return "", false
		}
	}
	idx = min(idx, n-1)
	if row.FallbackFirst && !row.Cells[idx].Focusable() {
		idx = 0
	}
	for i := idx; i >= 0; i-- {
		if row.Cells[i].Focusable() {
			return row.Cells[i].ID, true
		}
	}
	for i := idx + 1; i < n; i++ {
		if row.Cells[i].Focusable() {
			return row.Cells[i].ID, true
		}
	}
	return "", false
}

// entry is the ID focus lands on when it moves onto item it (see enterRow for idx and up).
func entry(it Item, idx int, up bool) (ID, bool) {
	if it.Kind == KindRow {
		return enterRow(it, idx, up)
	}
	return it.ID, it.Focusable()
}

// nextFocusable is where focus lands on the first focusable item of col strictly after row from
// (from -1 starts at the top), and prevFocusable on the last one strictly before row from; idx is
// the cell index of the widget focus leaves, or -1.
func nextFocusable(col []Item, from, idx int) (ID, bool) {
	for i := from + 1; i < len(col); i++ {
		if col[i].Focusable() {
			return entry(col[i], idx, false)
		}
	}
	return "", false
}

func prevFocusable(col []Item, from, idx int) (ID, bool) {
	for i := from - 1; i >= 0; i-- {
		if col[i].Focusable() {
			return entry(col[i], idx, true)
		}
	}
	return "", false
}

// groupFirst is the first focusable of group g in column-major order.
func groupFirst(g Group, idx int) (ID, bool) {
	for _, col := range g.Columns {
		if id, ok := nextFocusable(col, -1, idx); ok {
			return id, true
		}
	}
	return "", false
}

// col0Last is the last focusable item of column 0 of group g.
func col0Last(g Group, idx int) (ID, bool) {
	if len(g.Columns) == 0 {
		return "", false
	}
	return prevFocusable(g.Columns[0], len(g.Columns[0]), idx)
}

// groupFocusables lists the focusable widgets of g in column-major order, a row's cells in place
// of the row.
func groupFocusables(g Group) []Item {
	var out []Item
	for _, col := range g.Columns {
		for _, it := range col {
			if it.Kind == KindRow {
				for _, c := range it.Cells {
					if c.Focusable() {
						out = append(out, c)
					}
				}
				continue
			}
			if it.Focusable() {
				out = append(out, it)
			}
		}
	}
	return out
}

// ownSegment reports whether the item is a Tab segment of its own: every text input and list is.
func ownSegment(it Item) bool { return it.Kind == KindInput || it.Kind == KindList }

// segmentTable returns the first focusable of each Tab segment and the segment index of every
// focusable item. A group is one segment (Item.Seg splits it further: items of the group sharing
// a Seg are one segment wherever they sit), except that every text input and list is a segment of
// its own (also when adjacent) and splits the group around it. The buttons are the last segment.
func segmentTable(spec Spec) (starts []ID, index map[ID]int) {
	index = map[ID]int{}
	for _, g := range spec.Groups {
		seen := map[int]int{} // Seg value -> segment index, since the last input or list
		afterOwn := false
		for _, it := range groupFocusables(g) {
			if ownSegment(it) {
				starts = append(starts, it.ID)
				index[it.ID] = len(starts) - 1
				seen, afterOwn = map[int]int{}, true
				continue
			}
			if afterOwn {
				seen, afterOwn = map[int]int{}, false
			}
			si, ok := seen[it.Seg]
			if !ok {
				starts = append(starts, it.ID)
				si = len(starts) - 1
				seen[it.Seg] = si
			}
			index[it.ID] = si
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

// Focused is the widget that holds focus when the caller's focus is focus: focus itself when it
// names a focusable widget, else the first one (an empty or stale ID).
func Focused(spec Spec, focus ID) ID {
	if _, ok := find(spec, focus); ok {
		return focus
	}
	if segs := segments(spec); len(segs) > 0 {
		return segs[0]
	}
	return focus
}

// tabTarget is where Tab lands on the segment that starts at id: the selected cell of a row with
// Item.TabToSelected, else id itself.
func tabTarget(spec Spec, id ID) ID {
	p, ok := find(spec, id)
	if !ok || p.isButton() || p.cell < 0 {
		return id
	}
	row := itemAt(spec, p)
	if !row.TabToSelected {
		return id
	}
	for _, c := range row.Cells {
		if c.On && c.Focusable() {
			return c.ID
		}
	}
	return id
}

// nextGroupFirst is the first item of the first non-empty group after g, else the first button.
func nextGroupFirst(spec Spec, g, idx int) (ID, bool) {
	for gi := g + 1; gi < len(spec.Groups); gi++ {
		if id, ok := groupFirst(spec.Groups[gi], idx); ok {
			return id, true
		}
	}
	if len(spec.Buttons) > 0 {
		return spec.Buttons[0].ID, true
	}
	return "", false
}

// prevGroupLast is the last item of column 0 of the closest non-empty group before g.
func prevGroupLast(spec Spec, g, idx int) (ID, bool) {
	for gi := g - 1; gi >= 0; gi-- {
		if id, ok := col0Last(spec.Groups[gi], idx); ok {
			return id, true
		}
	}
	return "", false
}

// adjacentButton is the button of the strip row above (d = -1) or below (d = 1) the row of button
// b that sits visually nearest to it: the same relative position in the other row.
func adjacentButton(spec Spec, b, d int) (ID, bool) {
	rows := buttonRows(spec.Buttons)
	row, col := 0, 0
	for ri, r := range rows {
		for ci, btn := range r {
			if btn.ID == spec.Buttons[b].ID {
				row, col = ri, ci
			}
		}
	}
	next := row + d
	if next < 0 || next >= len(rows) {
		return "", false
	}
	return rows[next][(2*col+1)*len(rows[next])/(2*len(rows[row]))].ID, true
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
	*focus = Focused(spec, *focus)
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
		switch it := widgetAt(spec, cur); {
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
		e := activation(spec, *focus)
		e.Enter = true
		return e, true
	case tcell.KeyTab, tcell.KeyBacktab:
		segs, index := segmentTable(spec)
		step := 1
		if ev.Key() == tcell.KeyBacktab {
			step = -1
		}
		i := index[*focus]
		return move(tabTarget(spec, segs[(i+step+len(segs))%len(segs)]), true)
	case tcell.KeyPgUp, tcell.KeyPgDn:
		return scrollOnlyKey(spec, ev.Key())
	case tcell.KeyHome, tcell.KeyEnd:
		if isInput(spec, cur) {
			return Event{}, false // the caller moves the text cursor
		}
		return scrollOnlyKey(spec, ev.Key())
	case tcell.KeyDown:
		if cur.isButton() {
			if id, ok := adjacentButton(spec, cur.button, 1); ok {
				return move(id, true)
			}
			if e, ok := scrollOnlyKey(spec, ev.Key()); ok {
				return e, true // nowhere to move: scroll the view instead
			}
			return Event{}, true
		}
		if id, ok := nextFocusable(spec.Groups[cur.group].Columns[cur.col], cur.row, cur.cell); ok {
			return move(id, true)
		}
		return move(nextGroupFirst(spec, cur.group, cur.cell))
	case tcell.KeyUp:
		if cur.isButton() {
			if id, ok := adjacentButton(spec, cur.button, -1); ok {
				return move(id, true)
			}
			if id, ok := prevGroupLast(spec, len(spec.Groups), -1); ok {
				return move(id, true)
			}
			if e, ok := scrollOnlyKey(spec, ev.Key()); ok {
				return e, true // nowhere to move: scroll the view instead
			}
			return Event{}, true
		}
		if id, ok := prevFocusable(spec.Groups[cur.group].Columns[cur.col], cur.row, cur.cell); ok {
			return move(id, true)
		}
		if cur.col == 0 {
			return move(prevGroupLast(spec, cur.group, cur.cell))
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
		if cur.cell >= 0 { // along the row, over cells that cannot be focused
			cells := itemAt(spec, cur).Cells
			for c := cur.cell + d; c >= 0 && c < len(cells); c += d {
				if cells[c].Focusable() {
					*focus = cells[c].ID
					break
				}
			}
			return Event{}, true
		}
		cols := spec.Groups[cur.group].Columns
		if c := cur.col + d; c >= 0 && c < len(cols) && cur.row < len(cols[c]) && cols[c][cur.row].Focusable() {
			if id, ok := entry(cols[c][cur.row], -1, false); ok {
				*focus = id
			}
		}
		return Event{}, true
	case tcell.KeyRune:
		return handleRune(spec, focus, isInput(spec, cur), ev)
	}
	return Event{}, false
}

// HandleMnemonic is the Alt+letter stage of HandleKey on its own: Alt+letter activates the widget
// or button with that mnemonic and focuses it, except that it never pulls focus out of a text
// input (the user keeps typing where they were). Handlers whose own keys run before the engine
// (completion, path shortcuts, chords) run it first, so a mnemonic wins over every key of theirs.
// Every other key is (Event{}, false).
func HandleMnemonic(spec Spec, focus *ID, ev *tcell.EventKey) (Event, bool) {
	if ev.Key() != tcell.KeyRune || !keymap.AltLetterModifiers(ev.Modifiers()) {
		return Event{}, false
	}
	*focus = Focused(spec, *focus)
	cur, ok := find(spec, *focus)
	if !ok {
		return Event{}, false
	}
	inInput := isInput(spec, cur)
	r := unicode.ToLower(ev.Rune())
	hit := func(id ID) (Event, bool) {
		if !inInput {
			*focus = id
		}
		return Event{ID: id}, true
	}
	for _, it := range leaves(spec) {
		if it.Mnemonic != 0 && !it.Disabled && unicode.ToLower(it.Mnemonic) == r {
			return hit(it.ID)
		}
	}
	for _, b := range spec.Buttons {
		if b.Mnemonic != 0 && unicode.ToLower(b.Mnemonic) == r {
			return hit(b.ID)
		}
	}
	return Event{}, false
}

func handleRune(spec Spec, focus *ID, inInput bool, ev *tcell.EventKey) (Event, bool) {
	r := unicode.ToLower(ev.Rune())
	if keymap.AltLetterModifiers(ev.Modifiers()) {
		return HandleMnemonic(spec, focus, ev)
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
