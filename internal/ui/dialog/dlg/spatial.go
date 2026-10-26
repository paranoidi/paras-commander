package dlg

// navScreen is the screen the spec is laid out on to find where Up/Down go: large enough that
// nothing is clamped or cut off, so the dialog has its preferred width and every widget a rect.
const navScreen = 1 << 12

// spatialTarget is the focusable widget Up (d = -1) or Down (d = 1) moves to from widget from, by
// where the widgets are drawn: of the widgets wholly above / below it, the nearest row; in that row
// the one whose columns are nearest its own (an overlapping one first, so a column or a grid column
// is kept), then the leftmost. A widget's columns are its group column for an item of a column (a
// narrow radio still owns its column), else its drawn cells (row cells, buttons). The columns of a
// group are lanes: Up/Down never move into another column of the focused widget's own group, so
// leaving a column at its end goes on past the group (the sort dialog's mode and meta columns). Buttons are widgets like any other, so this also
// steps between the rows of a multi-row button strip.
func spatialTarget(spec Spec, from ID, d int) (ID, bool) {
	p, ok := Place(navScreen, navScreen, spec)
	if !ok {
		return "", false
	}
	cur, ok := p.Item(from)
	if !ok {
		return "", false
	}
	focusable := map[ID]bool{}
	for _, it := range leaves(spec) {
		if it.Focusable() {
			focusable[it.ID] = true
		}
	}
	for _, b := range spec.Buttons {
		focusable[b.ID] = true
	}
	span, lane := columnSpans(spec, p.Rect)
	ownLane, inLane := lane[from]
	spanOf := func(pi PlacedItem) Rect {
		if r, ok := span[pi.ID]; ok {
			return Rect{X: r.X, Y: pi.Rect.Y, Width: r.Width, Height: pi.Rect.Height}
		}
		return pi.Rect
	}
	from0 := spanOf(cur)
	type score struct{ dy, dx, x int }
	better := func(a, b score) bool {
		switch {
		case a.dy != b.dy:
			return a.dy < b.dy
		case a.dx != b.dx:
			return a.dx < b.dx
		}
		return a.x < b.x
	}
	var best ID
	var bestScore score
	for _, c := range p.Items {
		if c.ID == from || !focusable[c.ID] {
			continue
		}
		if l, ok := lane[c.ID]; inLane && ok && l.group == ownLane.group && l.col != ownLane.col {
			continue // another column of the same group
		}
		var dy int
		if d > 0 {
			dy = c.Rect.Y - (cur.Rect.Y + cur.Rect.Height)
		} else {
			dy = cur.Rect.Y - (c.Rect.Y + c.Rect.Height)
		}
		if dy < 0 {
			continue // not wholly above / below
		}
		s := score{dy: dy, dx: spanGap(from0, spanOf(c)), x: c.Rect.X}
		if best == "" || better(s, bestScore) {
			best, bestScore = c.ID, s
		}
	}
	return best, best != ""
}

// spanGap is how many cells apart the column spans of a and b are (0 when they overlap).
func spanGap(a, b Rect) int {
	switch {
	case b.X >= a.X+a.Width:
		return b.X - (a.X + a.Width) + 1
	case a.X >= b.X+b.Width:
		return a.X - (b.X + b.Width) + 1
	}
	return 0
}

// lane is a group column.
type lane struct{ group, col int }

// columnSpans maps every item of a group column (not the cells of a row, which keep their drawn
// rect) to the x span of its column in a dialog placed at rect, and every item and cell to its
// lane.
func columnSpans(spec Spec, rect Rect) (map[ID]Rect, map[ID]lane) {
	spans, lanes := map[ID]Rect{}, map[ID]lane{}
	for gi, g := range spec.Groups {
		for ci, col := range g.Columns {
			x, right := g.columnX(rect, ci)
			for _, it := range col {
				for _, c := range it.Cells {
					lanes[c.ID] = lane{gi, ci}
				}
				if it.Kind != KindRow && it.ID != "" {
					spans[it.ID] = Rect{X: x, Width: max(right-x, 1)}
					lanes[it.ID] = lane{gi, ci}
				}
			}
		}
	}
	return spans, lanes
}
