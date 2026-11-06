package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// cellList is a two-column grid of three rows whose last row holds one cell.
func cellList() Item {
	cell := func(s string) ListCell { return ListCell{Text: s} }
	return Item{
		ID: "grid", Kind: KindList, Markers: true, CellColumns: 2, Height: 2,
		Rows: []ListRow{
			{Cells: []ListCell{cell("amber"), cell("birch")}},
			{Cells: []ListCell{cell("cedar"), cell("dune")}},
			{Cells: []ListCell{cell("ember")}},
		},
	}
}

func TestCellListKeys(t *testing.T) {
	spec := listOnlySpec(cellList())
	focus := ID("grid")
	press := func(k tcell.Key) Event {
		t.Helper()
		e, ok := HandleKey(spec, &focus, tcell.NewEventKey(k, 0, tcell.ModNone))
		if !ok {
			t.Fatalf("key %v not handled", k)
		}
		return e
	}
	if e := press(tcell.KeyRight); e.Kind != EventListMove || e.Index != 0 || e.Col != 1 {
		t.Fatalf("Right = %+v", e)
	}
	list := &spec.Groups[0].Columns[0][0]
	list.Col = 1
	if e := press(tcell.KeyRight); e.Col != 1 {
		t.Fatalf("Right at the end = %+v", e)
	}
	list.Cursor = 1
	if e := press(tcell.KeyDown); e.Index != 2 || e.Col != 0 {
		t.Fatalf("Down onto the short row = %+v, want its only cell", e)
	}
	if e := press(tcell.KeyEnter); e.ID != "grid" || e.Kind != EventActivate || e.Index != 1 || e.Col != 1 || !e.Enter {
		t.Fatalf("Enter = %+v", e)
	}
}

func TestCellListClickAndHover(t *testing.T) {
	spec := listOnlySpec(cellList())
	const w, h = 80, 24
	p, ok := Place(w, h, spec)
	if !ok {
		t.Fatal("not placed")
	}
	pi, _ := p.Item("grid")
	cellW := pi.Rect.Width / 2
	focus := ID(OK)
	e, ok := Click(spec, w, h, &focus, pi.Rect.X+cellW+1, pi.Rect.Y+1, false)
	if !ok || e.Kind != EventActivate || e.Index != 1 || e.Col != 1 || focus != "grid" {
		t.Fatalf("click = %+v ok %v focus %q", e, ok, focus)
	}
	if r, ok := HoveredListRow(spec, w, h, OK, pi.Rect.X+cellW+1, pi.Rect.Y); !ok || r.X != pi.Rect.X+cellW || r.Width != cellW {
		t.Fatalf("hover = %+v ok %v", r, ok)
	}
	// Scroll to the short row: its blank right half is no target.
	spec.Groups[0].Columns[0][0].Cursor = 2
	p, _ = Place(w, h, spec)
	pi, _ = p.Item("grid")
	focus = OK
	if e, ok := Click(spec, w, h, &focus, pi.Rect.X+cellW+1, pi.Rect.Y+1, false); ok {
		t.Fatalf("click past the last cell = %+v", e)
	}
}
