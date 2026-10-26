package dlg

import "github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

// TargetKind is what a mouse press landed on.
type TargetKind int

const (
	TargetButton TargetKind = iota
	TargetOption            // a radio or a checkbox
	TargetInput
	TargetList
)

// Target describes the widget under a screen cell, for callers that turn a mouse press into the
// keys the engine already handles (focus it, then Space, Enter or list motion).
type Target struct {
	ID   ID
	Kind TargetKind
	// Focus is the widget that takes focus: ID itself, or for a list the input that drives it
	// (a driven list has no focus of its own).
	Focus ID
	// Row is the list row under the pointer and Cursor the list's current cursor (TargetList).
	Row, Cursor int
	// TextCol is the text position under the pointer (TargetInput): the clicked column past the
	// LeadingIcon, plus the input's Scroll.
	TextCol int
}

// TargetAt returns the widget drawn at (x, y) when spec is placed on a screenW x screenH screen,
// as Draw places it. ok is false on a miss: labels, separators, spacers (reserved regions), bars,
// disabled widgets and list rows past the last row are not targets.
func TargetAt(spec Spec, screenW, screenH, x, y int) (Target, bool) {
	p, placed := Place(screenW, screenH, spec)
	if !placed {
		return Target{}, false
	}
	id, hit := HitTest(p, x, y)
	if !hit {
		return Target{}, false
	}
	t := Target{ID: id, Focus: id}
	for _, b := range spec.Buttons {
		if b.ID == id {
			return t, true // TargetButton
		}
	}
	pi, _ := p.Item(id)
	it, _ := leafByID(spec, id)
	switch pi.Kind {
	case KindList:
		t.Kind = TargetList
		t.Row = pi.Offset + (y - pi.Rect.Y)
		t.Cursor = it.cursorRow()
		if driver, ok := driverOf(spec, id); ok && it.Driven {
			t.Focus = driver
		}
	case KindInput:
		t.Kind = TargetInput
		col := x - pi.Rect.X
		if !it.Simple {
			col -= draw.LeadingIconCols(it.LeadingIcon)
		}
		t.TextCol = max(col, 0) + max(it.Scroll, 0)
	default:
		t.Kind = TargetOption
	}
	return t, true
}

// leafByID returns the group item named id.
func leafByID(spec Spec, id ID) (Item, bool) {
	for _, it := range leaves(spec) {
		if it.ID == id && id != "" {
			return it, true
		}
	}
	return Item{}, false
}
