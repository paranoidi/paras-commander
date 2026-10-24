package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func key(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

func runeKey(r rune, m tcell.ModMask) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, m) }

func radio(id ID, label string, m rune) Item {
	return Item{ID: id, Kind: KindRadio, Label: label, Mnemonic: m}
}

func check(id ID, label string, m rune) Item {
	return Item{ID: id, Kind: KindCheckbox, Label: label, Mnemonic: m}
}

// testSpec: group 0 is a 2-column grid (3 rows left, 2 right), group 1 a single column, then
// OK / Cancel.
func testSpec() Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, MinHeight: 8, PlainButtonKeys: true,
		Groups: []Group{
			{Columns: [][]Item{
				{radio("a0", "Anchor", 'a'), radio("a1", "Buoy", 'b'), radio("a2", "Cable", 'c')},
				{radio("b0", "Dock", 0), radio("b1", "Ferry", 0)},
			}},
			{Columns: [][]Item{{check("c0", "Lantern", 'l'), check("c1", "Mast", 'm')}}},
		},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestNavigation(t *testing.T) {
	cases := []struct {
		name  string
		from  ID
		key   *tcell.EventKey
		want  ID
		wantB bool
	}{
		{"empty focus is first, down moves on", "", key(tcell.KeyDown), "a1", true},
		{"unknown focus is first", "nope", key(tcell.KeyUp), "a0", true},
		{"down in column", "a0", key(tcell.KeyDown), "a1", true},
		{"down bottom of col0 to next group", "a2", key(tcell.KeyDown), "c0", true},
		{"down bottom of col1 to next group", "b1", key(tcell.KeyDown), "c0", true},
		{"down inside col1", "b0", key(tcell.KeyDown), "b1", true},
		{"down last group to first button", "c1", key(tcell.KeyDown), OK, true},
		{"down on button stays", OK, key(tcell.KeyDown), OK, true},
		{"up top of first group stays", "a0", key(tcell.KeyUp), "a0", true},
		{"up top of col1 stays", "b0", key(tcell.KeyUp), "b0", true},
		{"up inside col1", "b1", key(tcell.KeyUp), "b0", true},
		{"up top of group 1 to last of col0 of group 0", "c0", key(tcell.KeyUp), "a2", true},
		{"up from button to last item of last group", Cancel, key(tcell.KeyUp), "c1", true},
		{"right to same row", "a1", key(tcell.KeyRight), "b1", true},
		{"right with no item on that row stays", "a2", key(tcell.KeyRight), "a2", true},
		{"right on last column stays", "b0", key(tcell.KeyRight), "b0", true},
		{"left back", "b1", key(tcell.KeyLeft), "a1", true},
		{"left on first column stays", "a0", key(tcell.KeyLeft), "a0", true},
		{"left right in single column stay", "c0", key(tcell.KeyRight), "c0", true},
		{"right along buttons", OK, key(tcell.KeyRight), Cancel, true},
		{"right on last button stays", Cancel, key(tcell.KeyRight), Cancel, true},
		{"left along buttons", Cancel, key(tcell.KeyLeft), OK, true},
		{"left on first button stays", OK, key(tcell.KeyLeft), OK, true},
		{"tab to next group", "a1", key(tcell.KeyTab), "c0", true},
		{"tab from col1", "b1", key(tcell.KeyTab), "c0", true},
		{"tab to buttons", "c1", key(tcell.KeyTab), OK, true},
		{"tab wraps", Cancel, key(tcell.KeyTab), "a0", true},
		{"backtab wraps to buttons", "b0", key(tcell.KeyBacktab), OK, true},
		{"backtab to first of previous group", "c1", key(tcell.KeyBacktab), "a0", true},
		{"backtab from button", Cancel, key(tcell.KeyBacktab), "c0", true},
		{"esc unhandled", "a1", key(tcell.KeyEsc), "a1", false},
		{"plain letter unhandled", "a1", runeKey('z', tcell.ModNone), "a1", false},
	}
	for _, c := range cases {
		focus := c.from
		ev, ok := HandleKey(testSpec(), &focus, c.key)
		if ok != c.wantB || focus != c.want || ev.ID != "" {
			t.Errorf("%s: got (focus=%q ev=%q ok=%v), want (focus=%q ok=%v)", c.name, focus, ev.ID, ok, c.want, c.wantB)
		}
	}
}

func TestActivationKeys(t *testing.T) {
	for _, k := range []*tcell.EventKey{key(tcell.KeyEnter), runeKey(' ', tcell.ModNone)} {
		focus := ID("b1")
		ev, ok := HandleKey(testSpec(), &focus, k)
		if !ok || ev.ID != "b1" || focus != "b1" {
			t.Errorf("%v: got (%q, %v, focus %q), want b1 activation", k.Key(), ev.ID, ok, focus)
		}
	}
	focus := ID("")
	if ev, _ := HandleKey(testSpec(), &focus, key(tcell.KeyEnter)); ev.ID != "a0" {
		t.Errorf("Enter on empty focus = %q, want first widget", ev.ID)
	}
}

func TestMnemonics(t *testing.T) {
	focus := ID("c1")
	ev, ok := HandleKey(testSpec(), &focus, runeKey('B', tcell.ModAlt))
	if !ok || ev.ID != "a1" || focus != "a1" {
		t.Fatalf("Alt+B: got (%q, %v, focus %q)", ev.ID, ok, focus)
	}
	ev, ok = HandleKey(testSpec(), &focus, runeKey('l', tcell.ModAlt))
	if !ok || ev.ID != "c0" || focus != "c0" {
		t.Fatalf("Alt+l: got (%q, %v, focus %q)", ev.ID, ok, focus)
	}
	ev, ok = HandleKey(testSpec(), &focus, runeKey('c', tcell.ModAlt)) // item 'c' wins over button 'C'
	if !ok || ev.ID != "a2" {
		t.Fatalf("Alt+c: got (%q, %v)", ev.ID, ok)
	}
	ev, ok = HandleKey(testSpec(), &focus, runeKey('o', tcell.ModAlt))
	if !ok || ev.ID != OK || focus != OK {
		t.Fatalf("Alt+o: got (%q, %v, focus %q)", ev.ID, ok, focus)
	}
	focus = "a0"
	if ev, ok := HandleKey(testSpec(), &focus, runeKey('z', tcell.ModAlt)); ok || ev.ID != "" || focus != "a0" {
		t.Fatalf("Alt+z should be unhandled, got (%q, %v, focus %q)", ev.ID, ok, focus)
	}
}

func TestPlainButtonKeys(t *testing.T) {
	focus := ID("a1")
	ev, ok := HandleKey(testSpec(), &focus, runeKey('o', tcell.ModNone))
	if !ok || ev.ID != OK || focus != "a1" {
		t.Fatalf("plain o: got (%q, %v, focus %q), want OK with focus unchanged", ev.ID, ok, focus)
	}
	if ev, _ := HandleKey(testSpec(), &focus, runeKey('C', tcell.ModNone)); ev.ID != Cancel {
		t.Fatalf("plain C = %q, want Cancel", ev.ID)
	}
	spec := testSpec()
	spec.PlainButtonKeys = false
	if _, ok := HandleKey(spec, &focus, runeKey('o', tcell.ModNone)); ok {
		t.Fatal("plain o handled without PlainButtonKeys")
	}
}

func TestPlace(t *testing.T) {
	p, ok := Place(80, 24, testSpec())
	if !ok {
		t.Fatal("Place not ok")
	}
	// 2 border + 3 group-0 rows + 2 group-1 rows + separator + button row = 9.
	if p.Rect != (Rect{X: 20, Y: 7, Width: 40, Height: 9}) {
		t.Fatalf("rect = %+v", p.Rect)
	}
	want := map[ID]Rect{
		"a0":   {X: 21, Y: 8, Width: 5 + 6, Height: 1},
		"a2":   {X: 21, Y: 10, Width: 5 + 5, Height: 1},
		"b0":   {X: 41, Y: 8, Width: 5 + 4, Height: 1}, // column 1 starts rect.Width/2 further right
		"b1":   {X: 41, Y: 9, Width: 5 + 5, Height: 1},
		"c0":   {X: 21, Y: 11, Width: 5 + 7, Height: 1}, // groups stack without blank rows
		"c1":   {X: 21, Y: 12, Width: 5 + 4, Height: 1},
		OK:     {X: 29, Y: 14, Width: 8, Height: 1},
		Cancel: {X: 39, Y: 14, Width: 12, Height: 1},
	}
	got := map[ID]Rect{}
	for _, it := range p.Items {
		got[it.ID] = it.Rect
	}
	for id, r := range want {
		if got[id] != r {
			t.Errorf("%s rect = %+v, want %+v", id, got[id], r)
		}
	}
	if p.SepY != 13 || p.ButtonY != 14 {
		t.Errorf("SepY/ButtonY = %d/%d, want 13/14", p.SepY, p.ButtonY)
	}
}

func TestPlaceClampsAndTruncates(t *testing.T) {
	spec := testSpec()
	spec.Groups[0].Columns[1][0].Label = "Extremely long second column label"
	p, ok := Place(30, 24, spec) // width clamps to 26; columns are 13 wide
	if !ok || p.Rect.Width != 26 {
		t.Fatalf("Place = %+v, %v; want width 26", p.Rect, ok)
	}
	for _, it := range p.Items {
		if it.ID == "b0" {
			if it.Rect.X != p.Rect.X+1+13 || it.Label == spec.Groups[0].Columns[1][0].Label {
				t.Fatalf("b0 = %+v label %q, want x=%d and a truncated label", it.Rect, it.Label, p.Rect.X+14)
			}
			if right := p.Rect.X + p.Rect.Width - 1; it.Rect.X+it.Rect.Width > right {
				t.Fatalf("b0 = %+v ends past the right border at x=%d", it.Rect, right)
			}
		}
	}
	if _, ok := Place(20, 24, spec); ok {
		t.Fatal("Place should fail when the width clamps below MinWidth")
	}
	if _, ok := Place(80, 7, spec); ok {
		t.Fatal("Place should fail when the height clamps below MinHeight")
	}
}

func TestHitTestAndClick(t *testing.T) {
	spec := testSpec()
	p, _ := Place(80, 24, spec)
	hit := func(x, y int) ID {
		id, _ := HitTest(p, x, y)
		return id
	}
	if hit(21, 8) != "a0" || hit(25, 8) != "a0" || hit(32, 8) != "" {
		t.Error("item hit/miss wrong on row 8")
	}
	if hit(42, 9) != "b1" || hit(30, 14) != OK || hit(45, 14) != Cancel {
		t.Error("expected b1, OK, Cancel hits")
	}
	for _, miss := range [][2]int{{20, 8}, {20, 7}, {40, 9}, {41, 10}, {41, 13}, {37, 14}, {0, 0}} {
		if id, ok := HitTest(p, miss[0], miss[1]); ok {
			t.Errorf("(%d,%d) hit %q, want miss (border, gap or blank)", miss[0], miss[1], id)
		}
	}

	focus := ID("a0")
	ev, ok := Click(spec, 80, 24, &focus, 22, 11)
	if !ok || ev.ID != "c0" || focus != "c0" {
		t.Fatalf("click checkbox: got (%q, %v, focus %q)", ev.ID, ok, focus)
	}
	ev, ok = Click(spec, 80, 24, &focus, 45, 14)
	if !ok || ev.ID != Cancel || focus != Cancel {
		t.Fatalf("click Cancel: got (%q, %v, focus %q)", ev.ID, ok, focus)
	}
	if _, ok := Click(spec, 80, 24, &focus, 20, 8); ok || focus != Cancel {
		t.Fatalf("click on border should miss and keep focus, focus=%q ok=%v", focus, ok)
	}
}
