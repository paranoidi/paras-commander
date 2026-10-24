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

// segments are the first focusables of each Tab segment: every non-empty group, then the buttons.
func segments(spec Spec) []ID {
	var segs []ID
	for _, g := range spec.Groups {
		if id, ok := groupFirst(g); ok {
			segs = append(segs, id)
		}
	}
	if len(spec.Buttons) > 0 {
		segs = append(segs, spec.Buttons[0].ID)
	}
	return segs
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

// segmentIndex is the index in segments(spec) of the segment holding p.
func segmentIndex(spec Spec, p pos) int {
	if p.isButton() {
		return len(segments(spec)) - 1
	}
	n := 0
	for gi := range p.group {
		if _, ok := groupFirst(spec.Groups[gi]); ok {
			n++
		}
	}
	return n
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
	switch ev.Key() {
	case tcell.KeyEnter:
		return Event{ID: *focus}, true
	case tcell.KeyTab, tcell.KeyBacktab:
		segs := segments(spec)
		step := 1
		if ev.Key() == tcell.KeyBacktab {
			step = -1
		}
		i := segmentIndex(spec, cur)
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
		return handleRune(spec, focus, ev)
	}
	return Event{}, false
}

func handleRune(spec Spec, focus *ID, ev *tcell.EventKey) (Event, bool) {
	r := unicode.ToLower(ev.Rune())
	if keymap.AltLetterModifiers(ev.Modifiers()) {
		for _, g := range spec.Groups {
			for _, col := range g.Columns {
				for _, it := range col {
					if it.Mnemonic != 0 && unicode.ToLower(it.Mnemonic) == r {
						*focus = it.ID
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
	if ev.Rune() == ' ' {
		return Event{ID: *focus}, true
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

// Click focuses the widget under (x, y) and activates it like Space. ok is false on a miss.
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
	return Event{ID: id}, true
}
