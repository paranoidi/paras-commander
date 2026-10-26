package dlg

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
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
		{"down bottom of col1 to next group, never into col0", "b1", key(tcell.KeyDown), "c0", true},
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
	ev, ok := Click(spec, 80, 24, &focus, 22, 11, false)
	if !ok || ev != (Event{ID: "c0"}) || focus != "c0" {
		t.Fatalf("checkbox click: got (%+v, %v), focus %q", ev, ok, focus)
	}
	ev, ok = Click(spec, 80, 24, &focus, 45, 14, true)
	if !ok || ev != (Event{ID: Cancel}) || focus != Cancel {
		t.Fatalf("Cancel click: got (%+v, %v), focus %q", ev, ok, focus)
	}
	if _, ok := Click(spec, 80, 24, &focus, 20, 8, false); ok || focus != Cancel {
		t.Fatalf("the border is no target and leaves focus alone (focus %q)", focus)
	}
}

func label(text string) Item { return Item{Kind: KindLabel, Label: text} }

// labelSpec: a text-only group (heading, spacer), then a column with a label between its items,
// then a two-column group whose right column has a label on the row of a left item, then buttons.
func labelSpec() Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, MinHeight: 6,
		Groups: []Group{
			{Columns: [][]Item{{label("Moor the boat?"), label("")}}},
			{Columns: [][]Item{{radio("r0", "Yes", 'y'), label("(ropes included)"), radio("r1", "No", 'n')}}},
			{Columns: [][]Item{
				{check("c0", "Lantern", 'l'), check("c1", "Mast", 'm')},
				{label("note"), check("d1", "Sail", 's')},
			}},
		},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestLabelPlacement(t *testing.T) {
	spec := labelSpec()
	spec.Groups[0].Columns[0][0].Label = "Moor the boat at a very long pier name that cannot fit the dialog"
	p, ok := Place(80, 24, spec)
	if !ok {
		t.Fatal("Place not ok")
	}
	// 2 border + 2 + 3 + 2 rows + separator + buttons.
	if p.Rect != (Rect{X: 20, Y: 6, Width: 40, Height: 11}) {
		t.Fatalf("rect = %+v", p.Rect)
	}
	var labels []PlacedItem
	for _, it := range p.Items {
		if it.Kind == KindLabel {
			labels = append(labels, it)
		}
	}
	if len(labels) != 4 {
		t.Fatalf("placed %d labels, want 4: %+v", len(labels), labels)
	}
	// Text starts at DialogTextX (rect.X+2) and spans the content width, one cell short of the border.
	head := labels[0]
	if head.Rect != (Rect{X: 22, Y: 7, Width: 36, Height: 1}) || head.Label == spec.Groups[0].Columns[0][0].Label {
		t.Errorf("heading = %+v %q, want x=22 w=36 and a truncated label", head.Rect, head.Label)
	}
	if spacer := labels[1]; spacer.Rect.Y != 8 || spacer.Label != "" {
		t.Errorf("spacer = %+v, want a blank row at y=8", spacer)
	}
	if mid := labels[2]; mid.Rect.Y != 10 || mid.Label != "(ropes included)" {
		t.Errorf("middle label = %+v, want it between the radios at y=10", mid)
	}
	// A label in the second column starts at that column's text x and stops before the border.
	if note := labels[3]; note.Rect != (Rect{X: 42, Y: 12, Width: 16, Height: 1}) {
		t.Errorf("note = %+v, want x=42 y=12 w=16", note.Rect)
	}
	if p.SepY != 14 || p.ButtonY != 15 {
		t.Errorf("SepY/ButtonY = %d/%d, want 14/15", p.SepY, p.ButtonY)
	}
}

func TestLabelsAreSkippedByFocus(t *testing.T) {
	cases := []struct {
		name string
		from ID
		key  *tcell.EventKey
		want ID
	}{
		{"empty focus is the first focusable", "", key(tcell.KeyDown), "r1"},
		{"down skips the label between radios", "r0", key(tcell.KeyDown), "r1"},
		{"up skips the label between radios", "r1", key(tcell.KeyUp), "r0"},
		{"up from the first focusable skips the text-only group", "r0", key(tcell.KeyUp), "r0"},
		{"down to the next group", "r1", key(tcell.KeyDown), "c0"},
		{"up into the previous group skips its labels", "c0", key(tcell.KeyUp), "r1"},
		{"right does not land on a label", "c0", key(tcell.KeyRight), "c0"},
		{"right onto an item beside a label row", "c1", key(tcell.KeyRight), "d1"},
		{"left back", "d1", key(tcell.KeyLeft), "c1"},
		{"down in the right column: the button under it", "d1", key(tcell.KeyDown), Cancel},
		{"tab skips the text-only group", OK, key(tcell.KeyTab), "r0"},
		{"backtab skips the text-only group", "r0", key(tcell.KeyBacktab), OK},
		{"tab between item groups", "r1", key(tcell.KeyTab), "c0"},
	}
	for _, c := range cases {
		focus := c.from
		if _, ok := HandleKey(labelSpec(), &focus, c.key); !ok || focus != c.want {
			t.Errorf("%s: got focus %q ok=%v, want %q", c.name, focus, ok, c.want)
		}
	}
	// Unknown or label-less IDs resolve to the first focusable, and activation never reports a label.
	focus := ID("")
	if ev, _ := HandleKey(labelSpec(), &focus, key(tcell.KeyEnter)); ev.ID != "r0" {
		t.Errorf("Enter on empty focus = %q, want r0", ev.ID)
	}
	if ev, ok := HandleKey(labelSpec(), &focus, runeKey('x', tcell.ModAlt)); ok || ev.ID != "" {
		t.Errorf("Alt+x = (%q, %v), want unhandled", ev.ID, ok)
	}
}

func TestLabelsAreNotHitTargets(t *testing.T) {
	spec := labelSpec()
	p, _ := Place(80, 24, spec)
	for _, c := range [][2]int{{22, 7}, {30, 7}, {22, 8}, {22, 10}, {42, 12}, {50, 12}} {
		if id, ok := HitTest(p, c[0], c[1]); ok {
			t.Errorf("(%d,%d) hit %q, want a miss on a label cell", c[0], c[1], id)
		}
	}
	if id, ok := HitTest(p, 22, 9); !ok || id != "r0" {
		t.Errorf("radio row hit = %q %v, want r0", id, ok)
	}
	var focus ID
	if _, ok := Click(spec, 80, 24, &focus, 22, 10, false); ok {
		t.Error("a label is no target")
	}
	if ev, ok := Click(spec, 80, 24, &focus, 22, 11, false); !ok || ev.ID != "r1" {
		t.Errorf("radio below the label = (%+v, %v), want r1", ev, ok)
	}
}

func TestDrawLabels(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := labelSpec()
	Draw(screen, 80, 24, spec, "", styles)
	screen.Show()
	row := func(y int) string { return tcelltest.TextAt(screen, 20, y, 40) }
	if got := row(7); !strings.HasPrefix(got, "│ Moor the boat?") {
		t.Errorf("heading row = %q", got)
	}
	if got := strings.TrimSpace(strings.Trim(row(8), "│")); got != "" {
		t.Errorf("spacer row = %q, want blank", got)
	}
	if got := row(10); !strings.HasPrefix(got, "│ (ropes included)") {
		t.Errorf("middle label row = %q", got)
	}
	if got := tcelltest.TextAt(screen, 42, 12, 4); got != "note" {
		t.Errorf("right column label = %q at x=42, want note at the column text x", got)
	}
	// Label text uses the dialog text style on the surface background.
	_, surface, _ := styles.DialogSurface.Decompose()
	want := styles.DialogText.Background(surface)
	if _, st, _ := screen.Get(22, 7); st != want {
		t.Errorf("label style = %v, want %v", st, want)
	}
}

func TestUnclampedPlacement(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 72, MinWidth: 44, Unclamped: true,
		Groups:  []Group{{Columns: [][]Item{{label("one"), label("two")}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}},
	}
	cases := []struct {
		name         string
		screenW      int
		screenH      int
		wantW, wantH int
	}{
		{"wide screen caps at Width", 120, 30, 72, 6},
		{"width follows the screen margin", 60, 30, 56, 6},
		{"below MinWidth widens to the screen minus 2", 46, 30, 44, 6},
		{"narrower than MinWidth", 40, 30, 38, 6},
		{"height shrinks to the screen, never refuses", 80, 4, 72, 4},
	}
	for _, c := range cases {
		p, ok := Place(c.screenW, c.screenH, spec)
		if !ok || p.Rect.Width != c.wantW || p.Rect.Height != c.wantH {
			t.Errorf("%s: rect %+v ok=%v, want %dx%d", c.name, p.Rect, ok, c.wantW, c.wantH)
		}
	}
	// The default (clamped) mode refuses where Unclamped draws.
	spec.Unclamped = false
	if _, ok := Place(40, 30, spec); ok {
		t.Error("clamped placement should refuse a screen below MinWidth")
	}
	// MinHeight raises an unclamped dialog's height too.
	spec.Unclamped, spec.MinHeight = true, 9
	if p, _ := Place(80, 30, spec); p.Rect.Height != 9 {
		t.Errorf("height = %d, want MinHeight 9", p.Rect.Height)
	}
}

func TestWidthGrowsToFitButtons(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 20, MinWidth: 10, MinHeight: 5,
		Groups: []Group{{Columns: [][]Item{{label("one")}}}},
		Buttons: []Button{
			{ID: "a", Label: "Anchor", Mnemonic: 'A'}, {ID: "b", Label: "Buoy", Mnemonic: 'B'},
			{ID: "c", Label: "Cable", Mnemonic: 'C'},
		},
	}
	// Three buttons are 12 + 10 + 11 wide with two 2-cell gaps: 37 between the borders.
	for _, unclamped := range []bool{false, true} {
		spec.Unclamped = unclamped
		p, ok := Place(80, 24, spec)
		if !ok || p.Rect.Width != 39 {
			t.Fatalf("unclamped=%v: rect %+v ok=%v, want width 39", unclamped, p.Rect, ok)
		}
		last := p.Items[len(p.Items)-1]
		if last.Rect.X < p.Rect.X+1 || last.Rect.X+last.Rect.Width > p.Rect.X+p.Rect.Width-1 {
			t.Errorf("unclamped=%v: last button %+v sticks out of the dialog %+v", unclamped, last.Rect, p.Rect)
		}
	}
	// A screen that cannot hold the strip still clamps.
	if p, _ := Place(30, 24, Spec{Width: 20, Unclamped: true, Buttons: spec.Buttons}); p.Rect.Width != 26 {
		t.Errorf("narrow screen width = %d, want 26", p.Rect.Width)
	}
}

func input(id ID, value string) Item { return Item{ID: id, Kind: KindInput, Value: value} }

func sep(text string) Item { return Item{Kind: KindSeparator, Label: text} }

// inputSpec: a label and two adjacent inputs, a separator, a checkbox/radio block and buttons.
func inputSpec() Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, MinHeight: 6, PlainButtonKeys: true,
		Groups: []Group{
			{Columns: [][]Item{{label("Find:"), input("find", "anchor"), input("repl", ""), sep("")}}},
			{Columns: [][]Item{{check("c0", "Lantern", 'l'), radio("r0", "Mast", 'm'), sep("Sails")}}},
		},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestSeparatorAndInputPlacement(t *testing.T) {
	p, ok := Place(80, 24, inputSpec())
	if !ok {
		t.Fatal("Place not ok")
	}
	// 2 border + label, 2 inputs, separator, 2 options, separator + button separator + buttons.
	if p.Rect != (Rect{X: 20, Y: 6, Width: 40, Height: 11}) {
		t.Fatalf("rect = %+v", p.Rect)
	}
	find, ok := p.Item("find")
	if !ok || find.Kind != KindInput || find.Rect != (Rect{X: 22, Y: 8, Width: 36, Height: 1}) {
		t.Errorf("find = %+v %v, want the row at DialogTextX across DialogContentWidth", find, ok)
	}
	if repl, _ := p.Item("repl"); repl.Rect.Y != 9 {
		t.Errorf("repl y = %d, want 9 (inputs stack without blank rows)", repl.Rect.Y)
	}
	var seps []PlacedItem
	for _, it := range p.Items {
		if it.Kind == KindSeparator {
			seps = append(seps, it)
		}
	}
	if len(seps) != 2 || seps[0].Rect != (Rect{X: 20, Y: 10, Width: 40, Height: 1}) || seps[0].Label != "" {
		t.Fatalf("separators = %+v, want a full-width rule at y=10", seps)
	}
	if seps[1].Rect.Y != 13 || seps[1].Label != " Sails " {
		t.Errorf("labelled separator = %+v, want y=13 and a padded label", seps[1])
	}
	if _, ok := p.Item(""); ok {
		t.Error("Item(\"\") must not match unnamed separators")
	}
	if p.SepY != 14 || p.ButtonY != 15 {
		t.Errorf("SepY/ButtonY = %d/%d, want 14/15", p.SepY, p.ButtonY)
	}
}

func TestSeparatorsAreNotFocusableOrHitTargets(t *testing.T) {
	spec := inputSpec()
	p, _ := Place(80, 24, spec)
	for _, y := range []int{10, 13} {
		for _, x := range []int{20, 21, 30, 59} {
			if id, ok := HitTest(p, x, y); ok {
				t.Errorf("(%d,%d) hit %q, want a miss on a separator", x, y, id)
			}
		}
	}
	var focus ID
	if _, ok := Click(spec, 80, 24, &focus, 30, 10, false); ok {
		t.Error("a separator is no target")
	}
	for from, want := range map[ID]ID{"repl": "c0", "c0": "r0", "r0": OK} {
		f := from
		if _, ok := HandleKey(spec, &f, key(tcell.KeyDown)); !ok || f != want {
			t.Errorf("down from %q = %q, want %q (separators skipped)", from, f, want)
		}
	}
	f := ID("c0")
	if _, ok := HandleKey(spec, &f, key(tcell.KeyUp)); !ok || f != "repl" {
		t.Errorf("up from c0 = %q, want repl", f)
	}
}

func TestInputsAreTheirOwnTabSegments(t *testing.T) {
	cases := []struct {
		from ID
		k    tcell.Key
		want ID
	}{
		{"find", tcell.KeyTab, "repl"}, // adjacent inputs: Tab does not skip the second
		{"repl", tcell.KeyTab, "c0"},
		{"c0", tcell.KeyTab, OK},
		{"r0", tcell.KeyTab, OK}, // the option block is one segment
		{OK, tcell.KeyTab, "find"},
		{"find", tcell.KeyBacktab, OK},
		{"repl", tcell.KeyBacktab, "find"},
		{"c0", tcell.KeyBacktab, "repl"},
		{"r0", tcell.KeyBacktab, "repl"},
		{"find", tcell.KeyDown, "repl"},
		{"repl", tcell.KeyUp, "find"},
		{"find", tcell.KeyUp, "find"},
	}
	for _, c := range cases {
		f := c.from
		if ev, ok := HandleKey(inputSpec(), &f, key(c.k)); !ok || f != c.want || ev.ID != "" {
			t.Errorf("%v from %q = (focus %q, ev %q, ok %v), want focus %q", c.k, c.from, f, ev.ID, ok, c.want)
		}
	}
	// An input between two option rows splits the group into three segments.
	spec := Spec{Groups: []Group{{Columns: [][]Item{{check("a", "A", 'a'), check("b", "B", 'b'), input("in", ""), check("c", "C", 'c')}}}}}
	f := ID("a")
	for _, want := range []ID{"in", "c", "a"} {
		if _, ok := HandleKey(spec, &f, key(tcell.KeyTab)); !ok || f != want {
			t.Fatalf("tab = %q, want %q", f, want)
		}
	}
}

func TestInputKeyContract(t *testing.T) {
	unhandled := []*tcell.EventKey{
		key(tcell.KeyLeft), key(tcell.KeyRight), key(tcell.KeyHome), key(tcell.KeyEnd),
		key(tcell.KeyBackspace), key(tcell.KeyBackspace2), key(tcell.KeyDelete), key(tcell.KeyEsc),
		runeKey(' ', tcell.ModNone), runeKey('a', tcell.ModNone), runeKey('Z', tcell.ModNone),
		runeKey('o', tcell.ModNone), runeKey('c', tcell.ModNone), // PlainButtonKeys letters are text here
		runeKey('x', tcell.ModAlt),
	}
	for _, k := range unhandled {
		f := ID("find")
		if ev, ok := HandleKey(inputSpec(), &f, k); ok || ev != (Event{}) || f != "find" {
			t.Errorf("%v %q: (focus %q, ev %+v, ok %v), want unhandled with focus kept", k.Key(), k.Rune(), f, ev, ok)
		}
	}
	f := ID("repl")
	if ev, ok := HandleKey(inputSpec(), &f, key(tcell.KeyEnter)); !ok || ev.ID != "repl" || f != "repl" {
		t.Errorf("Enter = (%q, %v, focus %q), want Event{repl}", ev.ID, ok, f)
	}
	// Alt+mnemonic of a checkbox / radio reports the widget but focus stays in the input.
	for r, want := range map[rune]ID{'l': "c0", 'M': "r0"} {
		f = "find"
		if ev, ok := HandleKey(inputSpec(), &f, runeKey(r, tcell.ModAlt)); !ok || ev.ID != want || f != "find" {
			t.Errorf("Alt+%c = (%q, %v, focus %q), want %q with focus kept", r, ev.ID, ok, f, want)
		}
	}
	// A button mnemonic presses the button, also without pulling focus out of the input; off an
	// input it focuses the button.
	f = "find"
	if ev, ok := HandleKey(inputSpec(), &f, runeKey('o', tcell.ModAlt)); !ok || ev.ID != OK || f != "find" {
		t.Errorf("Alt+o = (%q, %v, focus %q), want OK with focus kept", ev.ID, ok, f)
	}
	f = "c0"
	if ev, ok := HandleKey(inputSpec(), &f, runeKey('o', tcell.ModAlt)); !ok || ev.ID != OK || f != OK {
		t.Errorf("Alt+o from a checkbox = (%q, %v, focus %q), want OK focused", ev.ID, ok, f)
	}
	// HandleMnemonic is that stage alone: every other key is declined.
	f = "find"
	for _, k := range []*tcell.EventKey{key(tcell.KeyEnter), key(tcell.KeyTab), runeKey('l', tcell.ModNone), runeKey('z', tcell.ModAlt)} {
		if ev, ok := HandleMnemonic(inputSpec(), &f, k); ok || f != "find" {
			t.Errorf("HandleMnemonic(%v) = (%+v, %v), focus %q, want declined", k.Name(), ev, ok, f)
		}
	}
	if ev, ok := HandleMnemonic(inputSpec(), &f, runeKey('L', tcell.ModAlt)); !ok || ev.ID != "c0" || f != "find" {
		t.Errorf("HandleMnemonic(Alt+L) = (%+v, %v), focus %q, want c0 with focus kept", ev, ok, f)
	}
	// Off an input the same keys keep working: Space toggles, plain button letters press.
	f = "c0"
	if ev, ok := HandleKey(inputSpec(), &f, runeKey(' ', tcell.ModNone)); !ok || ev.ID != "c0" {
		t.Errorf("Space on checkbox = (%q, %v)", ev.ID, ok)
	}
	if ev, ok := HandleKey(inputSpec(), &f, runeKey('o', tcell.ModNone)); !ok || ev.ID != OK || f != "c0" {
		t.Errorf("plain o on checkbox = (%q, %v, focus %q)", ev.ID, ok, f)
	}
	// Alt mnemonic from a non-input row still moves focus.
	f = "r0"
	if _, ok := HandleKey(inputSpec(), &f, runeKey('l', tcell.ModAlt)); !ok || f != "c0" {
		t.Errorf("Alt+l from radio: focus %q, want c0", f)
	}
}

func TestInputClick(t *testing.T) {
	spec := inputSpec()
	var focus ID
	click := func(spec Spec, x, y int) Event {
		t.Helper()
		ev, ok := Click(spec, 80, 24, &focus, x, y, false)
		if !ok || ev.Kind != EventCaret {
			t.Fatalf("click (%d,%d) = (%+v, %v), want an EventCaret", x, y, ev, ok)
		}
		return ev
	}
	// The caret goes to the clicked rune, clamped to the end of the text ("anchor": 6 runes).
	if ev := click(spec, 22+5, 9); ev.ID != "repl" || ev.Index != 0 || focus != "repl" {
		t.Fatalf("empty input: got %+v (focus %q), want repl at 0", ev, focus)
	}
	if ev := click(spec, 22+2, 8); ev.ID != "find" || ev.Index != 2 || focus != "find" {
		t.Fatalf("third cell: got %+v, want find at 2", ev)
	}
	if ev := click(spec, 57, 8); ev.Index != 6 {
		t.Fatalf("past the text: got %+v, want the end 6", ev)
	}
	// A leading icon (plus its space) comes before the text, and a scrolled input adds its Scroll.
	icon := inputSpec()
	icon.Groups[0].Columns[0][1].Value = "anchorage bay"
	icon.Groups[0].Columns[0][1].LeadingIcon = ">"
	icon.Groups[0].Columns[0][1].Scroll = 4
	if ev := click(icon, 22+5, 8); ev.Index != 3+4 {
		t.Fatalf("icon input: got %+v, want 7", ev)
	}
	if ev := click(icon, 22, 8); ev.Index != 4 {
		t.Fatalf("click on the icon: got %+v, want the first visible character 4", ev)
	}
	// Wide runes take two cells.
	wide := inputSpec()
	wide.Groups[0].Columns[0][1].Value = "港口bay"
	if ev := click(wide, 22+3, 8); ev.Index != 1 {
		t.Fatalf("second cell of the second wide rune: got %+v, want 1", ev)
	}
	if ev := click(wide, 22+4, 8); ev.Index != 2 {
		t.Fatalf("cell after the wide runes: got %+v, want 2", ev)
	}
	for _, miss := range [][2]int{{21, 8}, {58, 8}} { // the margins around the input
		if id, hit := HitTest(mustPlace(t, spec), miss[0], miss[1]); hit {
			t.Errorf("(%d,%d) hit %q, want a miss in the input margin", miss[0], miss[1], id)
		}
	}
}

func mustPlace(t *testing.T, spec Spec) Placement {
	t.Helper()
	p, ok := Place(80, 24, spec)
	if !ok {
		t.Fatal("Place not ok")
	}
	return p
}

func TestLabelStylesPlacement(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindLabel, Label: "/home/harbor/orchard/meadow/lantern/willow", Fit: FitMiddle},
			{Kind: KindLabel, Label: "/home/harbor/orchard/meadow/lantern/willow", Fit: FitPath, Prefix: "Shared: ", Indent: 2},
			{Kind: KindLabel, Label: "abc", Center: true},
			{Kind: KindLabel, Label: "Wrong password.", Style: TextError},
		}}}},
	}
	p := mustPlace(t, spec) // content width 36
	if l := p.Items[0]; l.Rect.Width != 36 || l.Label != primitive.TruncateMiddle(spec.Groups[0].Columns[0][0].Label, 36) {
		t.Errorf("middle fit = %+v", l)
	}
	l := p.Items[1]
	if l.Rect.X != 24 || l.Rect.Width != 34 || !strings.HasPrefix(l.Label, "Shared: ") || len([]rune(l.Label)) > 34 {
		t.Errorf("path fit with prefix and indent = %+v", l)
	}
	if want := primitive.FitPathForWidth(spec.Groups[0].Columns[0][1].Label, 34-len("Shared: ")); l.Label != "Shared: "+want {
		t.Errorf("path label = %q, want the path fitted into what the prefix leaves", l.Label)
	}
	if c := p.Items[2]; c.Rect.X != 22+16 || c.Rect.Width != 36-16 {
		t.Errorf("centered = %+v, want pad (36-3)/2=16", c.Rect)
	}
}

func TestDrawSeparatorAndInputs(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := inputSpec()
	spec.Groups[0].Columns[0][1] = Item{ID: "find", Kind: KindInput, Value: "anchor", Cursor: 6}
	spec.Groups[0].Columns[0][2] = Item{ID: "repl", Kind: KindInput, Value: "x", Placeholder: true, Simple: true}
	Draw(screen, 80, 24, spec, "find", styles)
	screen.Show()
	row := func(y int) string { return tcelltest.TextAt(screen, 20, y, 40) }
	if got := row(10); got != "├"+strings.Repeat("─", 38)+"┤" {
		t.Errorf("plain separator row = %q", got)
	}
	if got := row(13); !strings.Contains(got, "─ Sails ─") || !strings.HasPrefix(got, "├") || !strings.HasSuffix(got, "┤") {
		t.Errorf("labelled separator row = %q", got)
	}
	if got := tcelltest.TextAt(screen, 22, 8, 6); got != "anchor" {
		t.Errorf("input text = %q", got)
	}
	// The separator uses the frame's border style; inputs use the dialog input styles.
	_, surface, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(surface)
	if _, st, _ := screen.Get(30, 10); st != borderStyle {
		t.Errorf("separator style = %v, want %v", st, borderStyle)
	}
	focusedBase, _ := styles.DialogInputPair(true)
	if _, st, _ := screen.Get(22, 8); st != focusedBase {
		t.Errorf("focused input style = %v, want %v", st, focusedBase)
	}
	if _, st, _ := screen.Get(40, 8); st != focusedBase {
		t.Errorf("input fill style = %v, want %v", st, focusedBase)
	}
	// The cursor cell is reversed on the focused input.
	if _, st, _ := screen.Get(28, 8); st != focusedBase.Reverse(true) {
		t.Errorf("cursor cell style = %v, want reversed", st)
	}
	// An unfocused input uses the inactive pair.
	inactiveBase, _ := styles.DialogInputPair(false)
	if _, st, _ := screen.Get(40, 9); st != inactiveBase {
		t.Errorf("unfocused input fill = %v, want %v", st, inactiveBase)
	}
}

func TestDrawInputGhostAndPlaceholder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20,
		Groups: []Group{{Columns: [][]Item{{
			{ID: "ghost", Kind: KindInput, Value: "/srv/or", Cursor: 7, CompletionSuffix: "chard"},
			{ID: "pending", Kind: KindInput, Value: "/srv/orchard/", Cursor: 13, Placeholder: true},
			{ID: "bad", Kind: KindInput, Value: "x", Cursor: 1, Invalid: true},
		}}}},
	}
	Draw(screen, 80, 24, spec, "ghost", styles)
	screen.Show()
	p := mustPlace(t, spec)
	row := func(id ID) PlacedItem { it, _ := p.Item(id); return it }
	if got := tcelltest.TextAt(screen, 22, row("ghost").Rect.Y, 12); got != "/srv/orchard" {
		t.Errorf("ghost row = %q, want the suffix drawn after the typed text", got)
	}
	focusedBase, focusedGhost := styles.DialogInputPair(true)
	if _, st, _ := screen.Get(22, row("ghost").Rect.Y); st != focusedBase {
		t.Errorf("typed text style = %v, want %v", st, focusedBase)
	}
	if _, st, _ := screen.Get(30, row("ghost").Rect.Y); st != focusedGhost {
		t.Errorf("ghost style = %v, want the placeholder style %v", st, focusedGhost)
	}
	inactiveBase, inactiveGhost := styles.DialogInputPair(false)
	if _, st, _ := screen.Get(24, row("pending").Rect.Y); st != inactiveGhost {
		t.Errorf("pending value style = %v, want the placeholder style %v", st, inactiveGhost)
	}
	if _, st, _ := screen.Get(22, row("bad").Rect.Y); st == inactiveBase {
		t.Errorf("invalid input drawn with the normal style %v", st)
	}
}

func TestDrawMessageErrorLabelStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := Spec{
		Title: "Harbor", Width: 40, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindLabel, Label: "lantern failed", Style: TextMessageError},
			{Kind: KindLabel, Label: "plain"},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}},
	}
	Draw(screen, 80, 24, spec, OK, styles)
	screen.Show()
	p := mustPlace(t, spec)
	_, surface, _ := styles.DialogSurface.Decompose()
	r := p.Items[0].Rect
	if _, st, _ := screen.Get(r.X, r.Y); st != styles.MessageError.Background(surface) {
		t.Errorf("message label style = %v, want MessageError on the surface", st)
	}
	r = p.Items[1].Rect
	if _, st, _ := screen.Get(r.X, r.Y); st != styles.DialogText.Background(surface) {
		t.Errorf("plain label style = %v, want DialogText", st)
	}
}

func TestInputTextWidth(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{ID: "plain", Kind: KindInput},
			{ID: "search", Kind: KindInput, LeadingIcon: "/"},
			{Kind: KindLabel, Label: "Lantern"},
		}}}},
		Buttons: OKCancelButtons(),
	}
	p := mustPlace(t, spec)
	plain, _ := p.Item("plain")
	if got := InputTextWidth(80, 24, spec, "plain"); got != plain.Rect.Width {
		t.Errorf("plain input: %d, want its placed width %d", got, plain.Rect.Width)
	}
	if got, want := InputTextWidth(80, 24, spec, "search"), plain.Rect.Width-draw.LeadingIconCols("/"); got != want {
		t.Errorf("input with a leading icon: %d, want %d (the icon's cells taken off)", got, want)
	}
	if got := InputTextWidth(80, 24, spec, "missing"); got != 0 {
		t.Errorf("unknown input: %d, want 0", got)
	}
}
