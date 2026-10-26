package dlg

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// capRow is a label cell with a fixed box followed by three radios, like a terminal capability row.
func capRow(prefix string, label string, mnemonic rune) Item {
	return Item{Kind: KindRow, Cells: []Item{
		{Kind: KindLabel, Label: label, Width: 12},
		radio(ID(prefix+"-auto"), "auto", mnemonic),
		radio(ID(prefix+"-yes"), "yes", 0),
		radio(ID(prefix+"-no"), "no", 0),
	}}
}

// rowSpec: three capability rows (one group, one column), a separator, a plain radio pair and
// OK / Cancel. The screen is 80x24 and the dialog 40 wide, so its rect starts at x=20.
func rowSpec() Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{
			{Columns: [][]Item{{
				label("Signals:"),
				capRow("sail", "Sail", 0), capRow("keel", "Keel", 0), capRow("mast", "Mast", 0),
				sep(""),
			}}},
			{Columns: [][]Item{{radio("p0", "Port", 'p'), radio("p1", "Starboard", 's')}}},
		},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestRowPlacement(t *testing.T) {
	p := mustPlace(t, rowSpec())
	if p.Rect.X != 20 || p.Rect.Width != 40 {
		t.Fatalf("rect = %+v, want x=20 width=40", p.Rect)
	}
	y := p.Rect.Y + 2 // row 0 is the "Signals:" label
	want := []struct {
		id    ID
		kind  Kind
		x, w  int
		label string
	}{
		{"", KindLabel, 22, 11, "Sail"},         // text starts one cell into its 12-cell box
		{"sail-auto", KindRadio, 33, 9, "auto"}, // the next box starts at x=21+12
		{"sail-yes", KindRadio, 42, 8, "yes"},
		{"sail-no", KindRadio, 50, 7, "no"},
	}
	for i, w := range want {
		got := p.Items[1+i]
		if got.ID != w.id || got.Kind != w.kind || got.Rect.X != w.x || got.Rect.Y != y || got.Rect.Width != w.w || got.Label != w.label {
			t.Errorf("cell %d = %+v, want %+v on y=%d", i, got, w, y)
		}
	}
	// Every cell of the second row sits on the next screen row at the same x positions.
	for i := 0; i < 4; i++ {
		a, b := p.Items[1+i], p.Items[5+i]
		if b.Rect.Y != a.Rect.Y+1 || b.Rect.X != a.Rect.X {
			t.Errorf("row 1 cell %d = %+v, want row 0 cell shifted down: %+v", i, b.Rect, a.Rect)
		}
	}
}

func TestRowGapGrowAndRightAlign(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{
				{Kind: KindLabel, Label: "Pattern:"},
				{Kind: KindLabel, Label: "12 f", Grow: true, Right: true},
			}},
			{Kind: KindRow, Cells: []Item{
				{ID: "q", Kind: KindInput, Value: "x", Gap: 2},
				{ID: "depth", Kind: KindInput, Width: 6},
			}},
			{Kind: KindRow, Cells: []Item{
				{ID: "a", Kind: KindRadio, Label: "Alpha", Gap: 4}, {ID: "b", Kind: KindRadio, Label: "Beta", Gap: 4}, check("c", "Gamma", 0),
			}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	p := mustPlace(t, spec)
	pattern, preview := p.Items[0], p.Items[1]
	if pattern.Rect.X != 22 || pattern.Label != "Pattern:" {
		t.Errorf("pattern label = %+v", pattern)
	}
	// The grown label ends one cell before the right border's margin: x 22..57 are text cells.
	if got := preview.Rect.X + preview.Rect.Width; got != 58 {
		t.Errorf("preview label ends at %d, want 58", got)
	}
	if preview.Rect.X != 58-4 || preview.Label != "12 f" {
		t.Errorf("preview label = %+v, want it right-aligned at x=54", preview)
	}
	q, depth := p.Items[2], p.Items[3]
	if depth.Rect.Width != 5 || depth.Rect.X+depth.Rect.Width != 58 {
		t.Errorf("fixed input = %+v, want width 5 (box 6 minus its leading cell) ending at 58", depth.Rect)
	}
	if q.Rect.X != 22 || q.Rect.X+q.Rect.Width != depth.Rect.X-1-2 {
		t.Errorf("growing input = %+v, want it to end %d cells before the depth box", q.Rect, 2)
	}
	a, b, c := p.Items[4], p.Items[5], p.Items[6]
	if b.Rect.X != a.Rect.X+a.Rect.Width+4 || c.Rect.X != b.Rect.X+b.Rect.Width+4 {
		t.Errorf("gap 4 after content boxes: %v %v %v", a.Rect, b.Rect, c.Rect)
	}
}

func TestRowClipsOverflowingCells(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 24, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{
				{ID: "a", Kind: KindRadio, Label: "Anchorage", Width: 12},
				{ID: "b", Kind: KindCheckbox, Label: "Breakwater"},
				{ID: "c", Kind: KindRadio, Label: "Cove"},
			}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	p := mustPlace(t, spec)
	end := p.Rect.X + p.Rect.Width - 2 // the row stops one cell before the right border
	a, b, c := p.Items[0], p.Items[1], p.Items[2]
	if a.Rect.Width != 12 || a.Label != primitive.TruncateRight("Anchorage", 7) {
		t.Errorf("fixed box cell = %+v, want its label shortened to the 12-cell box", a)
	}
	if b.Rect.X+b.Rect.Width > end {
		t.Errorf("clipped cell %+v ends past %d", b.Rect, end)
	}
	if c.Rect.Width != 0 || c.Label != "" {
		t.Errorf("cell past the end = %+v, want an empty rect", c)
	}
	// Nothing is drawn outside the dialog.
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	Draw(screen, 80, 24, spec, "a", theme.Default())
	screen.Show()
	for x := 0; x < 80; x++ {
		if x >= p.Rect.X && x < p.Rect.X+p.Rect.Width {
			continue
		}
		if ch, _, _ := screen.Get(x, p.Rect.Y+1); strings.TrimSpace(ch) != "" {
			t.Errorf("cell outside the dialog at x=%d = %q", x, ch)
		}
	}
}

func TestRowNarrowsGapsBeforeClipping(t *testing.T) {
	cells := func(gap int) []Item {
		return []Item{
			{ID: "a", Kind: KindRadio, Label: "Anchorage", Gap: gap},
			{ID: "b", Kind: KindCheckbox, Label: "Breakwater", Gap: gap},
			{ID: "c", Kind: KindRadio, Label: "Cove"},
		}
	}
	spec := func(gap int) Spec {
		return Spec{
			Title: "Harbor", Width: 44, MinWidth: 20, Unclamped: true,
			Groups:  []Group{{Columns: [][]Item{{{Kind: KindRow, Cells: cells(gap)}}}}},
			Buttons: []Button{{ID: OK, Label: "OK"}},
		}
	}
	// 14 + 15 + 9 cells of boxes in a 41-cell row: gaps of 6 do not fit, gaps of 1 do.
	wide := mustPlace(t, spec(6))
	a, b, c := wide.Items[0], wide.Items[1], wide.Items[2]
	if got := b.Rect.X - (a.Rect.X + a.Rect.Width); got >= 6 || got < 1 {
		t.Errorf("gap a..b = %d, want it narrowed below 6 (at least 1)", got)
	}
	if c.Label != "Cove" {
		t.Errorf("last cell = %+v, want it drawn whole once the gaps narrowed", c)
	}
	if end := wide.Rect.X + wide.Rect.Width - 2; c.Rect.X+c.Rect.Width > end {
		t.Errorf("last cell %+v ends past %d", c.Rect, end)
	}
	// A row that fits keeps its gaps.
	fits := mustPlace(t, spec(1))
	if got := fits.Items[1].Rect.X - (fits.Items[0].Rect.X + fits.Items[0].Rect.Width); got != 1 {
		t.Errorf("gap in a row that fits = %d, want 1", got)
	}
}

func TestRowHitRectsAndClick(t *testing.T) {
	spec := rowSpec()
	p := mustPlace(t, spec)
	y := p.Rect.Y + 3 // the keel row
	cases := []struct {
		x    int
		want ID
		hit  bool
	}{
		{22, "", false}, // the label cell is no target
		{33, "keel-auto", true},
		{41, "keel-auto", true},
		{42, "keel-yes", true},
		{50, "keel-no", true},
		{57, "", false}, // past the last radio
	}
	for _, c := range cases {
		id, ok := HitTest(p, c.x, y)
		if ok != c.hit || id != c.want {
			t.Errorf("HitTest(%d,%d) = (%q,%v), want (%q,%v)", c.x, y, id, ok, c.want, c.hit)
		}
	}
	var focus ID
	if ev, ok := Click(spec, 80, 24, &focus, 43, y, false); !ok || ev != (Event{ID: "keel-yes"}) || focus != "keel-yes" {
		t.Errorf("Click on a cell = (%+v,%v), focus %q, want keel-yes", ev, ok, focus)
	}
}

func TestRowLeftRight(t *testing.T) {
	cases := []struct {
		name string
		from ID
		key  tcell.Key
		want ID
	}{
		{"right along the row", "sail-auto", tcell.KeyRight, "sail-yes"},
		{"right again", "sail-yes", tcell.KeyRight, "sail-no"},
		{"right on the last cell stays", "sail-no", tcell.KeyRight, "sail-no"},
		{"left along the row", "sail-no", tcell.KeyLeft, "sail-yes"},
		{"left over the label stays on the first radio", "sail-auto", tcell.KeyLeft, "sail-auto"},
		{"left never leaves the row for another group", "p0", tcell.KeyLeft, "p0"},
	}
	for _, c := range cases {
		focus := c.from
		ev, ok := HandleKey(rowSpec(), &focus, key(c.key))
		if !ok || focus != c.want || ev.ID != "" {
			t.Errorf("%s: got (focus=%q ev=%q ok=%v), want %q", c.name, focus, ev.ID, ok, c.want)
		}
	}
}

func TestRowLeftRightSkipDisabledCells(t *testing.T) {
	spec := rowSpec()
	spec.Groups[0].Columns[0][1].Cells[2].Disabled = true // sail-yes
	focus := ID("sail-auto")
	HandleKey(spec, &focus, key(tcell.KeyRight))
	if focus != "sail-no" {
		t.Errorf("Right over a disabled cell = %q, want sail-no", focus)
	}
	HandleKey(spec, &focus, key(tcell.KeyLeft))
	if focus != "sail-auto" {
		t.Errorf("Left over a disabled cell = %q, want sail-auto", focus)
	}
}

func TestRowUpDownKeepsCellIndex(t *testing.T) {
	cases := []struct {
		name string
		from ID
		key  tcell.Key
		want ID
	}{
		{"down keeps the column", "sail-yes", tcell.KeyDown, "keel-yes"},
		{"down again", "keel-no", tcell.KeyDown, "mast-no"},
		{"up keeps the column", "mast-yes", tcell.KeyUp, "keel-yes"},
		{"up from the first row stays", "sail-no", tcell.KeyUp, "sail-no"},
		{"down from the last row enters the next group", "mast-no", tcell.KeyDown, "p0"},
		{"up into the rows from a plain item lands on the first cell", "p0", tcell.KeyUp, "mast-auto"},
	}
	for _, c := range cases {
		focus := c.from
		ev, ok := HandleKey(rowSpec(), &focus, key(c.key))
		if !ok || focus != c.want || ev.ID != "" {
			t.Errorf("%s: got (focus=%q ev=%q ok=%v), want %q", c.name, focus, ev.ID, ok, c.want)
		}
	}
}

func TestRowEntryLast(t *testing.T) {
	spec := rowSpec()
	spec.Groups[0].Columns[0][3].EntryLast = true // the mast row
	// Up from a plain item lands on the last focusable cell of an EntryLast row.
	focus := ID("p0")
	HandleKey(spec, &focus, key(tcell.KeyUp))
	if focus != "mast-no" {
		t.Errorf("Up from p0 into an EntryLast row = %q, want its last cell", focus)
	}
	// Without EntryLast it lands on the first cell.
	focus = "p0"
	HandleKey(rowSpec(), &focus, key(tcell.KeyUp))
	if focus != "mast-auto" {
		t.Errorf("Up from p0 into a plain row = %q, want its first cell", focus)
	}
	// From another row's cell the cell index wins over EntryLast.
	focus = "keel-yes"
	HandleKey(spec, &focus, key(tcell.KeyDown))
	if focus != "mast-yes" {
		t.Errorf("Down from a row cell into an EntryLast row = %q, want mast-yes", focus)
	}
	// Up from a button goes to the last item of the last group, a plain radio here.
	focus = Cancel
	HandleKey(spec, &focus, key(tcell.KeyUp))
	if focus != "p1" {
		t.Errorf("Up from Cancel = %q, want p1", focus)
	}
	// A row that is the last item of the last group is entered from the buttons at its last cell.
	last := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, EntryLast: true, Cells: []Item{radio("a", "A", 0), radio("b", "B", 0)}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	focus = OK
	HandleKey(last, &focus, key(tcell.KeyUp))
	if focus != "b" {
		t.Errorf("Up from OK into an EntryLast row = %q, want its last cell", focus)
	}
}

func TestRowIndexFallsBackToNearestFocusableCell(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{radio("a0", "A", 0), radio("a1", "B", 0), radio("a2", "C", 0)}},
			{Kind: KindRow, Cells: []Item{radio("b0", "D", 0), radio("b1", "E", 0)}},
			{Kind: KindRow, Cells: []Item{{Kind: KindLabel, Label: "x"}, radio("c1", "F", 0), {Kind: KindLabel, Label: "y"}}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	steps := []struct {
		from ID
		key  tcell.Key
		want ID
	}{
		{"a2", tcell.KeyDown, "b1"}, // past the end of a shorter row: its last cell
		{"b1", tcell.KeyDown, "c1"}, // index 1
		{"a0", tcell.KeyDown, "b0"}, // plain index
		{"b0", tcell.KeyDown, "c1"}, // label cell: the next focusable cell to the right
		{"a2", tcell.KeyUp, "a2"},   // nothing above
	}
	for _, s := range steps {
		focus := s.from
		HandleKey(spec, &focus, key(s.key))
		if focus != s.want {
			t.Errorf("%s %v = %q, want %q", s.from, s.key, focus, s.want)
		}
	}
	focus := ID("c1")
	HandleKey(spec, &focus, key(tcell.KeyUp))
	if focus != "b1" {
		t.Errorf("Up from c1 = %q, want b1", focus)
	}
}

func TestRowInputKeepsLeftRightForTextEditing(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 60, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{
				{ID: "q", Kind: KindInput, Value: "rope"},
				{ID: "d", Kind: KindInput, Width: 6},
			}},
			{Kind: KindRow, Cells: []Item{{ID: "a", Kind: KindRadio, Label: "Alpha", Mnemonic: 'a', Gap: 2}, check("b", "Beta", 'b')}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	for _, from := range []ID{"q", "d"} {
		for _, k := range []tcell.Key{tcell.KeyLeft, tcell.KeyRight, tcell.KeyHome, tcell.KeyBackspace2} {
			focus := from
			ev, ok := HandleKey(spec, &focus, key(k))
			if ok || ev.ID != "" || focus != from {
				t.Errorf("%s %v: got (ok=%v ev=%q focus=%q), want the key handed to the caller", from, k, ok, ev.ID, focus)
			}
		}
	}
	// Every input of a row is its own Tab segment; Up/Down keep the cell index.
	focus := ID("q")
	HandleKey(spec, &focus, key(tcell.KeyTab))
	if focus != "d" {
		t.Errorf("Tab from the first input = %q, want the second input", focus)
	}
	HandleKey(spec, &focus, key(tcell.KeyTab))
	if focus != "a" {
		t.Errorf("Tab from the second input = %q, want the next row", focus)
	}
	focus = "d"
	HandleKey(spec, &focus, key(tcell.KeyDown))
	if focus != "b" {
		t.Errorf("Down from input cell 1 = %q, want the checkbox at cell index 1", focus)
	}
	HandleKey(spec, &focus, key(tcell.KeyUp))
	if focus != "d" {
		t.Errorf("Up back = %q, want d", focus)
	}
	// An Alt mnemonic flips the option and leaves focus in the input.
	focus = "q"
	ev, ok := HandleKey(spec, &focus, runeKey('b', tcell.ModAlt))
	if !ok || ev.ID != "b" || focus != "q" {
		t.Errorf("Alt+b in an input = (%q,%v) focus %q, want b with focus kept", ev.ID, ok, focus)
	}
}

func TestSegmentsSplitTabStops(t *testing.T) {
	// A grid whose rows hold a radio and a checkbox: Seg makes the radios one Tab stop and the
	// checkboxes another, wherever they sit.
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{radio("r0", "A", 0), {ID: "t0", Kind: KindCheckbox, Label: "X", Seg: 1}}},
			{Kind: KindRow, Cells: []Item{radio("r1", "B", 0), {ID: "t1", Kind: KindCheckbox, Label: "Y", Seg: 1}}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	steps := []struct {
		from ID
		key  tcell.Key
		want ID
	}{
		{"r0", tcell.KeyTab, "t0"},
		{"r1", tcell.KeyTab, "t0"},
		{"t1", tcell.KeyTab, OK},
		{OK, tcell.KeyTab, "r0"},
		{"t1", tcell.KeyBacktab, "r0"},
		{"r1", tcell.KeyBacktab, OK},
		{OK, tcell.KeyBacktab, "t0"},
	}
	for _, s := range steps {
		focus := s.from
		HandleKey(spec, &focus, key(s.key))
		if focus != s.want {
			t.Errorf("%s %v = %q, want %q", s.from, s.key, focus, s.want)
		}
	}
}

func TestSegmentsAfterAnInputStartOver(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			radio("a", "A", 0), input("q", ""), radio("b", "B", 0), {ID: "c", Kind: KindRadio, Label: "C", Seg: 1},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	want := []ID{"a", "q", "b", "c", OK}
	got := segments(spec)
	if len(got) != len(want) {
		t.Fatalf("segments = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segments = %v, want %v", got, want)
		}
	}
}

func TestDisabledItems(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true, PlainButtonKeys: true,
		Groups: []Group{{Columns: [][]Item{{
			radio("a", "Alpha", 'a'),
			{Kind: KindRow, Cells: []Item{
				{ID: "b", Kind: KindCheckbox, Label: "Beta", Mnemonic: 'b', Disabled: true},
				{ID: "c", Kind: KindCheckbox, Label: "Gamma", Mnemonic: 'c', Width: 14},
			}},
			{ID: "d", Kind: KindCheckbox, Label: "Delta", Mnemonic: 'd', Disabled: true},
			radio("e", "Echo", 'e'),
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}},
	}
	p := mustPlace(t, spec)
	b, ok := p.Item("b")
	if !ok || !b.Disabled || b.Rect.Width == 0 {
		t.Fatalf("disabled item is still placed and drawn: %+v ok=%v", b, ok)
	}
	if id, hit := HitTest(p, b.Rect.X+2, b.Rect.Y); hit {
		t.Errorf("click on a disabled item hit %q", id)
	}
	// Focus skips disabled items in every direction.
	steps := []struct {
		from ID
		key  tcell.Key
		want ID
	}{
		{"a", tcell.KeyDown, "c"},
		{"c", tcell.KeyDown, "e"},
		{"e", tcell.KeyUp, "c"},
		{"c", tcell.KeyUp, "a"},
		{"a", tcell.KeyTab, OK},
	}
	for _, s := range steps {
		focus := s.from
		HandleKey(spec, &focus, key(s.key))
		if focus != s.want {
			t.Errorf("%s %v = %q, want %q", s.from, s.key, focus, s.want)
		}
	}
	// Alt+mnemonics of disabled items do nothing; others still work.
	for _, r := range []rune{'b', 'd'} {
		focus := ID("a")
		if ev, ok := HandleKey(spec, &focus, runeKey(r, tcell.ModAlt)); ok || ev.ID != "" || focus != "a" {
			t.Errorf("Alt+%c on a disabled item = (%q,%v) focus %q, want unhandled", r, ev.ID, ok, focus)
		}
	}
	focus := ID("a")
	if ev, ok := HandleKey(spec, &focus, runeKey('c', tcell.ModAlt)); !ok || ev.ID != "c" || focus != "c" {
		t.Errorf("Alt+c = (%q,%v) focus %q", ev.ID, ok, focus)
	}
	// A focus pointing at an item that became disabled falls back to the first focusable item.
	focus = "b"
	if _, ok := HandleKey(spec, &focus, key(tcell.KeyDown)); !ok || focus != "c" {
		t.Errorf("focus on a disabled item moved to %q, want c", focus)
	}
}

func TestDrawRowsAndDisabled(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{
				{Kind: KindLabel, Label: "Pattern:"},
				{Kind: KindLabel, Label: "9 f", Grow: true, Right: true},
			}},
			{Kind: KindRow, Cells: []Item{
				{ID: "v", Kind: KindCheckbox, Label: "Videos", Width: 14, On: true},
				{ID: "p", Kind: KindCheckbox, Label: "Pictures", Disabled: true},
			}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	Draw(screen, 80, 24, spec, "v", styles)
	screen.Show()
	p := mustPlace(t, spec)
	top := p.Rect.Y + 1
	row := tcelltest.TextAt(screen, 20, top, 40)
	if !strings.HasPrefix(row, "│ Pattern:") || !strings.HasSuffix(row, "9 f │") {
		t.Errorf("label row = %q, want Pattern: left and 9 f right-aligned before the margin", row)
	}
	if got := tcelltest.TextAt(screen, 20, top+1, 40); !strings.Contains(got, "Videos") || !strings.Contains(got, "Pictures") {
		t.Errorf("checkbox row = %q", got)
	}
	pic, _ := p.Item("p")
	_, st, _ := screen.Get(pic.Rect.X+6, pic.Rect.Y)
	if want := styles.DialogOptionDisabledStyle(); st != want {
		t.Errorf("disabled checkbox style = %v, want %v", st, want)
	}
	vid, _ := p.Item("v")
	_, st, _ = screen.Get(vid.Rect.X+6, vid.Rect.Y)
	if want := styles.DialogOptionRowStyle(true, true); st != want {
		t.Errorf("focused checked checkbox style = %v, want %v", st, want)
	}
}

func TestRowMarkerWidth(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{radio("a", "Alpha", 0), radio("b", "Beta", 0)}},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	a5, _ := mustPlace(t, spec).Item("b")
	spec.MarkerWidth = 3 // a theme whose icons are narrower than the ASCII " ( ) "
	p := mustPlace(t, spec)
	a, _ := p.Item("a")
	b, _ := p.Item("b")
	if a.Rect.Width != 3+len("Alpha") || b.Rect.X != a.Rect.X+a.Rect.Width {
		t.Errorf("content boxes with MarkerWidth 3: a=%+v b=%+v, want adjacent boxes of marker+label", a.Rect, b.Rect)
	}
	if b.Rect.X >= a5.Rect.X {
		t.Errorf("narrow markers should pull the next cell left: %d vs %d", b.Rect.X, a5.Rect.X)
	}
	nerd := theme.Default()
	nerd.UseNerdfontIcons = true
	if w := MarkerWidth(nerd); w >= 5 {
		t.Errorf("MarkerWidth(nerd) = %d, want narrower than the ASCII marker", w)
	}
	ascii := theme.Default()
	ascii.UseNerdfontIcons = false
	if w := MarkerWidth(ascii); w != 5 {
		t.Errorf("MarkerWidth(ascii) = %d, want 5", w)
	}
}

// TestDrawNeverPaintsOnOrBelowTheBottomBorder: a dialog that does not fit the screen keeps its
// frame and shows the rows that do fit; nothing lands on the border row or below it.
func TestDrawNeverPaintsOnOrBelowTheBottomBorder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 8)
	spec := labelSpec()
	for i := 0; i < 8; i++ {
		spec.Groups[0].Columns[0] = append(spec.Groups[0].Columns[0], label("row "+string(rune('a'+i))))
	}
	spec.Buttons = []Button{{ID: OK, Label: "OK"}}
	p, ok := Place(80, 8, spec)
	if !ok {
		t.Fatal("Place not ok")
	}
	Draw(screen, 80, 8, spec, "", theme.Default())
	screen.Show()
	bottom := p.Rect.Y + p.Rect.Height - 1
	row := tcelltest.TextAt(screen, p.Rect.X, bottom, p.Rect.Width)
	if want := "└" + strings.Repeat("─", p.Rect.Width-2) + "┘"; row != want {
		t.Errorf("bottom border row = %q, want an intact border", row)
	}
	for y := bottom + 1; y < 8; y++ {
		if got := strings.TrimSpace(tcelltest.TextAt(screen, 0, y, 80)); got != "" {
			t.Errorf("row %d below the dialog = %q, want blank", y, got)
		}
	}
}

// TestClampedHeightKeepsTheButtons: when the frame is shorter than the content, lists shrink
// first and the separator and button strip stay inside the frame; only content that cannot shrink
// is cut off above the separator.
func TestClampedHeightKeepsTheButtons(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20,
		Groups: []Group{
			{Columns: [][]Item{{label("top"), {ID: "l", Kind: KindList, Height: 10, Rows: []ListRow{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}}}}}},
			{Columns: [][]Item{{radio("r0", "One", 0), radio("r1", "Two", 0), radio("r2", "Three", 0)}}},
		},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, h := range []int{10, 8, 6} { // frame heights 8, 6, 4 after the 2-row screen margin
		screen.SetSize(80, h)
		p, ok := Place(80, h, spec)
		if !ok {
			t.Fatalf("height %d: Place not ok", h)
		}
		bottom := p.Rect.Y + p.Rect.Height - 1
		if p.SepY >= bottom || p.ButtonY != p.SepY+1 || p.ButtonY >= bottom {
			t.Errorf("height %d: separator %d / buttons %d outside the frame ending at %d", h, p.SepY, p.ButtonY, bottom)
		}
		for _, it := range p.Items {
			if it.ID != OK && it.Rect.Y < p.ContentBottom && it.Rect.Y+it.Rect.Height > p.ContentBottom {
				t.Errorf("height %d: %q spills over the separator", h, it.ID)
			}
		}
		screen.Clear()
		Draw(screen, 80, h, spec, "", theme.Default())
		screen.Show()
		if row := tcelltest.TextAt(screen, p.Rect.X, p.ButtonY, p.Rect.Width); !strings.Contains(row, "OK") {
			t.Errorf("height %d: button row = %q, want the OK button", h, row)
		}
	}
	// The list gives up rows before the radios do.
	p, _ := Place(80, 14, spec)
	if l, ok := p.Item("l"); !ok || l.Rect.Height >= 10 {
		t.Errorf("list height = %d, want it shrunk below 10", l.Rect.Height)
	}
	if r, ok := p.Item("r2"); !ok || r.Rect.Y >= p.ContentBottom {
		t.Errorf("last radio %+v is cut off while the list could shrink", r.Rect)
	}
}
