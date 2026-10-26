package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// scrollSpec is a ScrollOnly list of 30 rows (6 visible, scrolled to offset) above OK / Cancel,
// with an optional checkbox above the list.
func scrollSpec(offset int, withCheck bool) Spec {
	list := listItem(30, 0, offset, 6)
	list.ScrollOnly, list.PageStep = true, 6
	col := []Item{list}
	if withCheck {
		col = append([]Item{check("c0", "Lantern", 'l')}, col...)
	}
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups:  []Group{{Columns: [][]Item{col}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestScrollOnlyListKeys(t *testing.T) {
	spec := scrollSpec(4, false)
	if f := Focused(spec, ""); f != OK {
		t.Fatalf("first focus = %q, want OK: a ScrollOnly list is never focused", f)
	}
	scroll := func(spec Spec, focus ID, k tcell.Key) Event {
		t.Helper()
		ev, ok := HandleKey(spec, &focus, key(k))
		if !ok || ev.ID != "hosts" || ev.Kind != EventListMove {
			t.Fatalf("%v: (%+v, %v), want a list move", k, ev, ok)
		}
		return ev
	}
	for k, want := range map[tcell.Key]int{
		tcell.KeyPgDn: 10, tcell.KeyPgUp: 0, tcell.KeyDown: 5, tcell.KeyUp: 3, tcell.KeyHome: 0, tcell.KeyEnd: 24,
	} {
		if ev := scroll(spec, OK, k); ev.Offset != want {
			t.Errorf("%v from OK: offset %d, want %d", k, ev.Offset, want)
		}
	}
	if ev := scroll(scrollSpec(22, false), Cancel, tcell.KeyPgDn); ev.Offset != 24 {
		t.Errorf("PgDn near the end: offset %d, want the last full page 24", ev.Offset)
	}
	// Up from a button with a widget above moves focus there instead of scrolling.
	focus := OK
	if ev, ok := HandleKey(scrollSpec(4, true), &focus, key(tcell.KeyUp)); !ok || ev.ID != "" || focus != "c0" {
		t.Errorf("Up with a checkbox above: (%+v, %v), focus %q, want the checkbox", ev, ok, focus)
	}
	// PgDn still scrolls from there; Home/End scroll from a checkbox too.
	if ev := scroll(scrollSpec(4, true), "c0", tcell.KeyPgDn); ev.Offset != 10 {
		t.Errorf("PgDn from the checkbox: offset %d, want 10", ev.Offset)
	}
	if ev := scroll(scrollSpec(4, true), "c0", tcell.KeyEnd); ev.Offset != 24 {
		t.Errorf("End from the checkbox: offset %d, want 24", ev.Offset)
	}
}

func TestScrollOnlyListMouse(t *testing.T) {
	spec := scrollSpec(4, false)
	p := mustPlace(t, spec)
	it, _ := p.Item("hosts")
	focus := Cancel
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+1, false); ok || focus != Cancel {
		t.Errorf("click on a ScrollOnly list = (%+v, %v), focus %q, want a miss", ev, ok, focus)
	}
	if ev, ok := Wheel(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+1, 3); !ok || ev != (Event{ID: "hosts", Kind: EventListMove, Offset: 7}) || focus != Cancel {
		t.Errorf("wheel = (%+v, %v), focus %q, want offset 7 and focus left on Cancel", ev, ok, focus)
	}
	if ev, _ := Wheel(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+1, -9); ev.Offset != 0 {
		t.Errorf("wheel past the top: offset %d, want 0", ev.Offset)
	}
	rows := 0
	p.EachListRow("hosts", func(row, y int) {
		if row != 4+rows || y != it.Rect.Y+rows {
			t.Errorf("EachListRow call %d = (%d, %d), want (%d, %d)", rows, row, y, 4+rows, it.Rect.Y+rows)
		}
		rows++
	})
	if rows != 6 {
		t.Errorf("EachListRow visited %d rows, want the 6 visible", rows)
	}
}

// headerList is a list of 10 rows whose rows 0 and 4 are section headers.
func headerList(cursor, offset, height int) Item {
	list := listItem(10, cursor, offset, height)
	list.Rows[0].Header, list.Rows[4].Header = true, true
	return list
}

func TestHeaderRows(t *testing.T) {
	move := func(list Item, k tcell.Key) int {
		t.Helper()
		focus := ID("hosts")
		ev, ok := HandleKey(listOnlySpec(list), &focus, key(k))
		if !ok || ev.Kind != EventListMove {
			t.Fatalf("%v: (%+v, %v), want a list move", k, ev, ok)
		}
		return ev.Index
	}
	for _, c := range []struct {
		cursor int
		k      tcell.Key
		want   int
	}{
		{1, tcell.KeyUp, 1},   // row 0 is a header: the cursor stays on the first entry
		{3, tcell.KeyDown, 5}, // over the header at 4
		{5, tcell.KeyUp, 3},
		{7, tcell.KeyHome, 1},
		{1, tcell.KeyEnd, 9},
	} {
		if got := move(headerList(c.cursor, 0, 10), c.k); got != c.want {
			t.Errorf("%v from %d = %d, want %d", c.k, c.cursor, got, c.want)
		}
	}
	// Revealing the row under a header reveals the header too, while the cursor still fits.
	if off := headerList(5, 5, 3).ViewOffset(); off != 4 {
		t.Errorf("viewport for cursor 5 at offset 5 = %d, want 4 (its header)", off)
	}
	if off := headerList(5, 5, 1).ViewOffset(); off != 5 {
		t.Errorf("one-row viewport = %d, want the cursor row 5", off)
	}
	// A click on a header is a miss.
	spec := listOnlySpec(headerList(1, 0, 10))
	it, _ := mustPlace(t, spec).Item("hosts")
	var focus ID
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+4, false); ok {
		t.Errorf("click on a header = (%+v, %v), want a miss", ev, ok)
	}
}

func TestDrawRowSpansAndHeader(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	keyStyle := styles.DialogHelpKey
	list := headerList(1, 0, 10)
	list.Fit = FitNone
	for _, i := range []int{1, 2} {
		list.Rows[i].Spans = []primitive.Span{{Start: 0, End: 4, Style: keyStyle}}
		list.Rows[i].Match = []search.Range{{Start: 0, End: 1}}
	}
	spec := listOnlySpec(list)
	Draw(screen, 80, 24, spec, "hosts", styles)
	screen.Show()
	it, _ := mustPlace(t, spec).Item("hosts")
	_, surface, _ := styles.DialogSurface.Decompose()
	style := func(x, row int) tcell.Style {
		_, st, _ := screen.Get(it.Rect.X+x, it.Rect.Y+row)
		return st
	}
	if st := style(1, 0); st != styles.DialogHelpSection.Background(surface) {
		t.Errorf("header row style = %v, want dialog.help.section", st)
	}
	if st := style(1, 2); st != keyStyle {
		t.Errorf("span cell = %v, want the span's style", st)
	}
	if st := style(0, 2); st == keyStyle {
		t.Error("the match highlight must sit on top of the span")
	}
	if st := style(1, 1); st != styles.DialogOptionRowStyle(true, false) {
		t.Errorf("cursor row span cell = %v, want the cursor style (spans dropped)", st)
	}
}
