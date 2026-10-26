package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSpatialUpDown(t *testing.T) {
	cell := func(id ID, label string) Item {
		r := radio(id, label, 0)
		r.Width = 14
		return r
	}
	// Two columns of a group (the right one shorter), a row of four cells over a row of two, and a
	// full-width input above OK / Cancel.
	spec := Spec{
		Title: "Harbor", Width: 64, Unclamped: true,
		Groups: []Group{
			{Columns: [][]Item{
				{radio("l0", "Lantern", 0), radio("l1", "Orchard", 0), radio("l2", "Willow", 0)},
				{radio("r0", "Meadow", 0)},
			}},
			{Columns: [][]Item{{
				{Kind: KindRow, Cells: []Item{cell("a0", "Anchor"), cell("a1", "Timber"), cell("a2", "Ribbon"), cell("a3", "Velvet")}},
				{Kind: KindRow, Cells: []Item{cell("b0", "Marble"), cell("b1", "Otter")}},
				{ID: "in", Kind: KindInput},
			}}},
		},
		Buttons: OKCancelButtons(),
	}
	for _, c := range []struct {
		name string
		from ID
		k    tcell.Key
		want ID
	}{
		{"a column is a lane: Down off its end skips the other column, to the cell under it", "r0", tcell.KeyDown, "a2"},
		{"down within a column", "l0", tcell.KeyDown, "l1"},
		{"up into a two-column group from below: the leftmost of its last row", "a1", tcell.KeyUp, "l2"},
		{"a grid column is kept", "a1", tcell.KeyDown, "b1"},
		{"the next row comes first, at its nearest cell", "a3", tcell.KeyDown, "b1"},
		{"a full-width input below: its row", "b1", tcell.KeyDown, "in"},
		{"from a full-width input: the leftmost cell above", "in", tcell.KeyUp, "b0"},
		{"down to the buttons", "in", tcell.KeyDown, OK},
		{"nothing above the first row: stay", "l0", tcell.KeyUp, "l0"},
	} {
		focus := c.from
		if _, ok := HandleKey(spec, &focus, key(c.k)); !ok || focus != c.want {
			t.Errorf("%s: %s %v = %q (%v), want %q", c.name, c.from, c.k, focus, ok, c.want)
		}
	}
}
