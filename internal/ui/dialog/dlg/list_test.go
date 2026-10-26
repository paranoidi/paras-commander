package dlg

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func listRows(n int) []ListRow {
	words := []string{"anchor", "buoy", "cable", "dock", "ferry", "harbor", "lantern", "mast", "oar", "rope"}
	rows := make([]ListRow, n)
	for i := range rows {
		rows[i] = ListRow{Text: fmt.Sprintf("%s %d", words[i%len(words)], i)}
	}
	return rows
}

func listItem(n, cursor, offset, height int) Item {
	return Item{ID: "hosts", Kind: KindList, Rows: listRows(n), Cursor: cursor, Offset: offset, Height: height}
}

// listSpec is a filter input driving a list, a location input and OK / Cancel (the SFTP shape).
func listSpec(list Item) Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{ID: "filter", Kind: KindInput, Drives: "hosts"},
			sep(""),
			list,
		}}}, {Columns: [][]Item{{
			{ID: "where", Kind: KindInput},
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

// listOnlySpec is a focusable list with OK / Cancel (the theme picker shape).
func listOnlySpec(list Item) Spec {
	return Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true, PlainButtonKeys: true,
		Groups:  []Group{{Columns: [][]Item{{list}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestListPlacementAndHeight(t *testing.T) {
	spec := listOnlySpec(listItem(30, 4, 0, 6))
	p := mustPlace(t, spec)
	it, ok := p.Item("hosts")
	if !ok || it.Kind != KindList {
		t.Fatalf("list not placed: %+v", it)
	}
	// border + 6 list rows + separator + buttons + border
	if p.Rect.Height != 10 {
		t.Errorf("dialog height = %d, want 10", p.Rect.Height)
	}
	if it.Rect.Y != p.Rect.Y+1 || it.Rect.Height != 6 || it.Rect.X != p.Rect.X+2 || it.Rect.Width != p.Rect.Width-4 {
		t.Errorf("list rect = %+v in dialog %+v", it.Rect, p.Rect)
	}
	if it.Rows != 30 || it.Offset != 0 {
		t.Errorf("rows/offset = %d/%d, want 30/0", it.Rows, it.Offset)
	}
	if p.SepY != p.Rect.Y+7 || p.ButtonY != p.Rect.Y+8 {
		t.Errorf("SepY/ButtonY = %d/%d", p.SepY, p.ButtonY)
	}
}

func TestListViewOffset(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want int
	}{
		{"short list never scrolls", listItem(4, 3, 2, 6), 0},
		{"sticky keeps a visible offset", listItem(30, 7, 4, 6), 4},
		{"sticky follows cursor down", listItem(30, 12, 4, 6), 7},
		{"sticky follows cursor up", listItem(30, 2, 4, 6), 2},
		{"sticky clamps a stale offset", listItem(30, 29, 99, 6), 24},
		{"empty", listItem(0, 0, 3, 6), 0},
	}
	for _, c := range cases {
		if got := c.item.ViewOffset(); got != c.want {
			t.Errorf("%s: ViewOffset = %d, want %d", c.name, got, c.want)
		}
	}
	centered := listItem(30, 15, 0, 6)
	centered.CenterScroll = true
	if got := centered.ViewOffset(); got != 12 {
		t.Errorf("centered middle: ViewOffset = %d, want 12", got)
	}
	centered.Cursor = 29
	if got := centered.ViewOffset(); got != 24 {
		t.Errorf("centered bottom: ViewOffset = %d, want 24", got)
	}
}

func TestListMotionFocused(t *testing.T) {
	cases := []struct {
		name       string
		item       Item
		key        *tcell.EventKey
		wantIndex  int
		wantOffset int
	}{
		{"down", listItem(30, 3, 0, 6), key(tcell.KeyDown), 4, 0},
		{"down scrolls at the bottom edge", listItem(30, 5, 0, 6), key(tcell.KeyDown), 6, 1},
		{"up", listItem(30, 3, 0, 6), key(tcell.KeyUp), 2, 0},
		{"up at the top stops and is consumed", listItem(30, 0, 0, 6), key(tcell.KeyUp), 0, 0},
		{"down at the bottom stops and is consumed", listItem(30, 29, 24, 6), key(tcell.KeyDown), 29, 24},
		{"pgdn is height-1", listItem(30, 0, 0, 6), key(tcell.KeyPgDn), 5, 0},
		{"pgdn clamps", listItem(30, 27, 22, 6), key(tcell.KeyPgDn), 29, 24},
		{"pgup is height-1", listItem(30, 20, 15, 6), key(tcell.KeyPgUp), 15, 15},
		{"pgup clamps", listItem(30, 2, 0, 6), key(tcell.KeyPgUp), 0, 0},
		{"home", listItem(30, 20, 15, 6), key(tcell.KeyHome), 0, 0},
		{"end", listItem(30, 2, 0, 6), key(tcell.KeyEnd), 29, 24},
		{"ctrl-home", listItem(30, 20, 15, 6), tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl), 0, 0},
		{"ctrl-end", listItem(30, 2, 0, 6), tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl), 29, 24},
	}
	for _, c := range cases {
		focus := ID("hosts")
		ev, ok := HandleKey(listOnlySpec(c.item), &focus, c.key)
		if !ok || ev.Kind != EventListMove || ev.ID != "hosts" || ev.Index != c.wantIndex || ev.Offset != c.wantOffset {
			t.Errorf("%s: got (%+v, %v), want ListMove index %d offset %d", c.name, ev, ok, c.wantIndex, c.wantOffset)
		}
		if focus != "hosts" {
			t.Errorf("%s: focus moved to %q", c.name, focus)
		}
	}
}

func TestListMotionPageStepWrapAndCenter(t *testing.T) {
	it := listItem(30, 0, 0, 6)
	it.PageStep = 6
	focus := ID("hosts")
	if ev, _ := HandleKey(listOnlySpec(it), &focus, key(tcell.KeyPgDn)); ev.Index != 6 {
		t.Errorf("PageStep 6: pgdn index = %d, want 6", ev.Index)
	}
	it = listItem(25, 0, 0, 6)
	it.Wrap, it.CenterScroll = true, true
	for _, c := range []struct {
		cursor int
		k      tcell.Key
		want   int
	}{
		{0, tcell.KeyUp, 24}, {24, tcell.KeyDown, 0}, {0, tcell.KeyPgUp, 20}, {24, tcell.KeyPgDn, 4}, {3, tcell.KeyDown, 4},
	} {
		it.Cursor = c.cursor
		ev, ok := HandleKey(listOnlySpec(it), &focus, key(c.k))
		if !ok || ev.Index != c.want {
			t.Errorf("wrap cursor %d key %v: index = %d, want %d", c.cursor, c.k, ev.Index, c.want)
		}
	}
	it.Cursor = 12
	if ev, _ := HandleKey(listOnlySpec(it), &focus, key(tcell.KeyDown)); ev.Offset != 10 {
		t.Errorf("centered offset after moving to row 13 = %d, want 10", ev.Offset)
	}
}

func TestListEnterSpaceAndLetters(t *testing.T) {
	spec := listOnlySpec(listItem(30, 7, 3, 6))
	for _, k := range []*tcell.EventKey{key(tcell.KeyEnter), runeKey(' ', tcell.ModNone)} {
		focus := ID("hosts")
		ev, ok := HandleKey(spec, &focus, k)
		if !ok || ev.Kind != EventActivate || ev.ID != "hosts" || ev.Index != 7 {
			t.Errorf("%v on list: (%+v, %v), want activate index 7", k.Name(), ev, ok)
		}
	}
	focus := ID("hosts")
	// plain letters are the caller's unless PlainButtonKeys presses a button
	noPlain := listOnlySpec(listItem(30, 7, 3, 6))
	noPlain.PlainButtonKeys = false
	if ev, ok := HandleKey(noPlain, &focus, runeKey('x', tcell.ModNone)); ok || ev != (Event{}) {
		t.Errorf("plain letter: (%+v, %v), want (zero, false)", ev, ok)
	}
	if ev, ok := HandleKey(noPlain, &focus, runeKey('o', tcell.ModNone)); ok || ev != (Event{}) {
		t.Errorf("plain o without PlainButtonKeys: (%+v, %v), want (zero, false)", ev, ok)
	}
	if ev, ok := HandleKey(spec, &focus, runeKey('o', tcell.ModNone)); !ok || ev.ID != OK {
		t.Errorf("plain o with PlainButtonKeys: (%+v, %v), want OK", ev, ok)
	}
	if ev, ok := HandleKey(spec, &focus, runeKey('c', tcell.ModAlt)); !ok || ev.ID != Cancel || focus != Cancel {
		t.Errorf("alt+c: (%+v, %v) focus %q, want Cancel", ev, ok, focus)
	}
}

func TestListTabsAndEmptyList(t *testing.T) {
	spec := listSpec(listItem(5, 0, 0, 4))
	focus := ID("filter")
	var visited []ID
	for i := 0; i < 5; i++ {
		HandleKey(spec, &focus, key(tcell.KeyTab))
		visited = append(visited, focus)
	}
	if got := fmt.Sprint(visited); got != "[hosts where ok filter hosts]" {
		t.Errorf("tab order = %s, want the list as its own stop", got)
	}
	focus = "hosts"
	HandleKey(spec, &focus, key(tcell.KeyBacktab))
	if focus != "filter" {
		t.Errorf("backtab from list = %q, want filter", focus)
	}
	// An empty list is not a focus stop: Tab and the arrows skip it.
	emptySpec := listSpec(listItem(0, 0, 0, 4))
	focus = "filter"
	HandleKey(emptySpec, &focus, key(tcell.KeyTab))
	if focus != "where" {
		t.Errorf("tab past an empty list = %q, want where", focus)
	}
	focus = "where"
	HandleKey(emptySpec, &focus, key(tcell.KeyUp))
	if focus != "filter" {
		t.Errorf("up past an empty list = %q, want filter", focus)
	}
	if segs := segments(listOnlySpec(listItem(0, 0, 0, 4))); len(segs) != 1 || segs[0] != OK {
		t.Errorf("segments with an empty list = %v, want only the buttons", segs)
	}
	// Left / Right do nothing on a list.
	focus = "hosts"
	if ev, ok := HandleKey(spec, &focus, key(tcell.KeyLeft)); !ok || ev != (Event{}) || focus != "hosts" {
		t.Errorf("left on list: (%+v, %v) focus %q", ev, ok, focus)
	}
}

func TestInputDrivesList(t *testing.T) {
	spec := listSpec(listItem(30, 5, 0, 6))
	focus := ID("filter")
	cases := []struct {
		name  string
		k     *tcell.EventKey
		index int
	}{
		{"down", key(tcell.KeyDown), 6},
		{"up", key(tcell.KeyUp), 4},
		{"pgdn", key(tcell.KeyPgDn), 10},
		{"pgup", key(tcell.KeyPgUp), 0},
		{"ctrl-end", tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModCtrl), 29},
		{"ctrl-home", tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModCtrl), 0},
	}
	for _, c := range cases {
		ev, ok := HandleKey(spec, &focus, c.k)
		if !ok || ev.Kind != EventListMove || ev.ID != "hosts" || ev.Index != c.index {
			t.Errorf("%s: (%+v, %v), want ListMove hosts index %d", c.name, ev, ok, c.index)
		}
		if focus != "filter" {
			t.Fatalf("%s: focus left the input: %q", c.name, focus)
		}
	}
	// Text editing keys stay with the caller.
	for _, k := range []*tcell.EventKey{key(tcell.KeyHome), key(tcell.KeyEnd), key(tcell.KeyLeft), key(tcell.KeyRight), key(tcell.KeyBackspace2), runeKey('x', tcell.ModNone), runeKey(' ', tcell.ModNone)} {
		if ev, ok := HandleKey(spec, &focus, k); ok || ev != (Event{}) {
			t.Errorf("%s in driving input: (%+v, %v), want (zero, false)", k.Name(), ev, ok)
		}
	}
	// Enter is still the input's own event.
	if ev, ok := HandleKey(spec, &focus, key(tcell.KeyEnter)); !ok || ev.ID != "filter" || ev.Kind != EventActivate {
		t.Errorf("enter in driving input: (%+v, %v)", ev, ok)
	}
	// With an empty list the same keys move focus like in any input.
	emptySpec := listSpec(listItem(0, 0, 0, 6))
	focus = "filter"
	if ev, ok := HandleKey(emptySpec, &focus, key(tcell.KeyDown)); !ok || ev != (Event{}) || focus != "where" {
		t.Errorf("down with empty list: (%+v, %v) focus %q, want focus where", ev, ok, focus)
	}
	focus = "filter"
	if ev, ok := HandleKey(emptySpec, &focus, key(tcell.KeyUp)); !ok || ev != (Event{}) || focus != "filter" {
		t.Errorf("up with empty list: (%+v, %v) focus %q, want stay", ev, ok, focus)
	}
	// An input that drives nothing is not affected.
	plain := listSpec(listItem(30, 5, 0, 6))
	plain.Groups[0].Columns[0][0].Drives = ""
	focus = "filter"
	HandleKey(plain, &focus, key(tcell.KeyDown))
	if focus != "hosts" {
		t.Errorf("down in a non-driving input = %q, want the list", focus)
	}
}

func TestListClickWheelAndHitRects(t *testing.T) {
	spec := listOnlySpec(listItem(3, 1, 0, 6))
	p := mustPlace(t, spec)
	it, _ := p.Item("hosts")
	for row := 0; row < 6; row++ {
		id, hit := HitTest(p, it.Rect.X+1, it.Rect.Y+row)
		if want := row < 3; hit != want || (hit && id != "hosts") {
			t.Errorf("row %d: HitTest = (%q, %v), want hit=%v", row, id, hit, want)
		}
	}
	var focus ID
	ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+2, false)
	if !ok || ev != (Event{ID: "hosts", Kind: EventListMove, Index: 2}) || focus != "hosts" {
		t.Errorf("row 2: (%+v, %v), focus %q", ev, ok, focus)
	}
	// A double click on the cursor row activates it like Enter; elsewhere it only moves.
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+1, true); !ok || ev != (Event{ID: "hosts", Index: 1, Enter: true}) {
		t.Errorf("double click on the cursor row: (%+v, %v), want its activation", ev, ok)
	}
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+2, true); !ok || ev.Kind != EventListMove {
		t.Errorf("double click off the cursor row: (%+v, %v), want a move", ev, ok)
	}
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+4, false); ok {
		t.Errorf("past the last row: (%+v, %v), want a miss", ev, ok)
	}
	// The wheel moves the cursor, clamped to the rows.
	if ev, ok := Wheel(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y, 3); !ok || ev.Kind != EventListMove || ev.Index != 2 {
		t.Errorf("wheel down: (%+v, %v), want the last row 2", ev, ok)
	}
	if ev, ok := Wheel(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y, -3); !ok || ev.Index != 0 {
		t.Errorf("wheel up: (%+v, %v), want row 0", ev, ok)
	}
	// A scrolled list maps the clicked line to the row at that line.
	long := listOnlySpec(listItem(30, 12, 7, 6))
	lp := mustPlace(t, long)
	lit, _ := lp.Item("hosts")
	if lit.Offset != 7 {
		t.Fatalf("placed offset = %d, want 7", lit.Offset)
	}
	if ev, ok = Click(long, 80, 24, &focus, lit.Rect.X, lit.Rect.Y+3, false); !ok || ev.Index != 10 || ev.Offset != 7 {
		t.Errorf("scrolled list: (%+v, %v), want row 10 at offset 7", ev, ok)
	}
}

func TestDrivenList(t *testing.T) {
	item := listItem(30, 5, 0, 6)
	item.Driven = true
	spec := listSpec(item)
	// A driven list is not a focus stop or Tab segment: the filter input is.
	focus := ID("filter")
	var visited []ID
	for i := 0; i < 4; i++ {
		HandleKey(spec, &focus, key(tcell.KeyTab))
		visited = append(visited, focus)
	}
	if got := fmt.Sprint(visited); got != "[where ok filter where]" {
		t.Errorf("tab order = %s, want the driven list skipped", got)
	}
	focus = "filter"
	HandleKey(spec, &focus, key(tcell.KeyDown))
	if focus != "filter" {
		t.Errorf("down in the driving input moved focus to %q", focus)
	}
	// A row of a driven list gives focus to the driving input.
	p := mustPlace(t, spec)
	it, _ := p.Item("hosts")
	focus = "where"
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+2, false); !ok || ev.Kind != EventListMove || ev.ID != "hosts" || ev.Index != 2 || focus != "filter" {
		t.Errorf("driven list click: (%+v, %v), focus %q, want row 2 and focus filter", ev, ok, focus)
	}
	// A double click on the cursor row is Enter in the driving input.
	if ev, ok := Click(spec, 80, 24, &focus, it.Rect.X+1, it.Rect.Y+5, true); !ok || ev != (Event{ID: "filter", Enter: true}) {
		t.Errorf("driven list double click: (%+v, %v), want the filter's activation", ev, ok)
	}
}

func TestRowCellInputDrivesList(t *testing.T) {
	// A driving input inside a row (find's query beside its depth input) steers the list too.
	list := listItem(30, 5, 0, 6)
	list.Driven = true
	spec := Spec{
		Title: "Harbor", Width: 40, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindRow, Cells: []Item{
				{ID: "filter", Kind: KindInput, Grow: true, Drives: "hosts"},
				{ID: "depth", Kind: KindInput, Width: 6},
			}},
			sep(""),
			list,
		}}}},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
	focus := ID("filter")
	ev, ok := HandleKey(spec, &focus, key(tcell.KeyDown))
	if !ok || ev.Kind != EventListMove || ev.ID != "hosts" || ev.Index != 6 || focus != "filter" {
		t.Errorf("down in a row-cell driving input: (%+v, %v) focus %q, want ListMove 6 and focus kept", ev, ok, focus)
	}
	// The other cell of the row does not drive it.
	focus = "depth"
	if ev, _ := HandleKey(spec, &focus, key(tcell.KeyDown)); ev.Kind == EventListMove {
		t.Errorf("down in the depth cell moved the list: %+v", ev)
	}
}

func TestSpacerAndColumnWidths(t *testing.T) {
	spec := Spec{
		Title: "Harbor", Width: 60, MinWidth: 20, Unclamped: true,
		Groups: []Group{{
			ColWidths: []int{20},
			Columns: [][]Item{
				{listItem(3, 0, 0, 5)},
				{{ID: "pane", Kind: KindSpacer, Height: 5}},
			},
		}},
		Buttons: []Button{{ID: OK, Label: "OK"}},
	}
	p := mustPlace(t, spec)
	list, _ := p.Item("hosts")
	pane, _ := p.Item("pane")
	if list.Rect.X != p.Rect.X+2 || list.Rect.Width != 18 {
		t.Errorf("list rect = %+v, want x %d width 18", list.Rect, p.Rect.X+2)
	}
	if pane.Rect.X != p.Rect.X+21 || pane.Rect.Width != p.Rect.Width-22 || pane.Rect.Height != 5 || pane.Rect.Y != list.Rect.Y {
		t.Errorf("spacer rect = %+v, dialog %+v", pane.Rect, p.Rect)
	}
	if _, hit := HitTest(p, pane.Rect.X+1, pane.Rect.Y+1); hit {
		t.Error("spacer is a hit target")
	}
	focus := ID("hosts")
	HandleKey(spec, &focus, key(tcell.KeyRight))
	if focus != "hosts" {
		t.Errorf("right onto a spacer moved focus to %q", focus)
	}
	if ev, ok := HandleKey(spec, &focus, key(tcell.KeyTab)); !ok || ev != (Event{}) || focus != OK {
		t.Errorf("tab from list = %q, want OK (the spacer is not a stop)", focus)
	}
	if p.Rect.Height != 2+5+2 {
		t.Errorf("dialog height = %d, want 9", p.Rect.Height)
	}
}

func TestDrawListPlain(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	item := listItem(12, 8, 3, 5)
	item.Fit = FitMiddle
	item.Rows[8].Match = []search.Range{{Start: 0, End: 2}}
	spec := listSpec(item)
	spec.ScrollbarStyle = uiscrollbar.StyleThumb
	Draw(screen, 80, 24, spec, "filter", styles)
	screen.Show()
	p := mustPlace(t, spec)
	it, _ := p.Item("hosts")
	rowText := func(i int) string {
		return strings.TrimSpace(tcelltest.TextAt(screen, it.Rect.X, it.Rect.Y+i, it.Rect.Width))
	}
	for i, want := range []string{"ferry 4", "harbor 5", "lantern 6", "mast 7", "oar 8"} {
		if got := rowText(i); got != want {
			t.Errorf("visible row %d = %q, want %q", i, got, want)
		}
	}
	// Offset 3, cursor 8 is below the 5-row window 3..7: the sticky viewport follows to 4..8.
	if it.Offset != 4 {
		t.Fatalf("placed offset = %d, want 4", it.Offset)
	}
	if got := rowText(4); got != "oar 8" {
		t.Errorf("cursor row = %q, want oar 8", got)
	}
	// The driving input's focus lights the cursor row; the match is highlighted on top of it.
	_, surface, _ := styles.DialogSurface.Decompose()
	if _, st, _ := screen.Get(it.Rect.X+4, it.Rect.Y+4); st != styles.DialogOptionRowStyle(true, false) {
		t.Errorf("cursor row style = %v, want the focused option row", st)
	}
	if _, st, _ := screen.Get(it.Rect.X, it.Rect.Y+4); st == styles.DialogOptionRowStyle(true, false) {
		t.Errorf("match cells keep the plain row style %v, want the highlight", st)
	}
	if _, st, _ := screen.Get(it.Rect.X+4, it.Rect.Y); st != styles.DialogText.Background(surface) {
		t.Errorf("plain row style = %v", st)
	}
	// Without focus on the list or its input the cursor row is a plain row.
	Draw(screen, 80, 24, spec, "where", styles)
	screen.Show()
	if _, st, _ := screen.Get(it.Rect.X+4, it.Rect.Y+4); st != styles.DialogText.Background(surface) {
		t.Errorf("unfocused cursor row style = %v, want a plain row", st)
	}
	// No scrollbar unless the list asks for one.
	if ch, _, _ := screen.Get(p.Rect.X+p.Rect.Width-1, it.Rect.Y+1); ch != "│" {
		t.Errorf("right border cell = %q, want the plain border", ch)
	}
	item.Scrollbar = true
	spec = listSpec(item)
	spec.ScrollbarStyle = uiscrollbar.StyleBar
	Draw(screen, 80, 24, spec, "filter", styles)
	screen.Show()
	found := false
	for y := it.Rect.Y; y < it.Rect.Y+it.Rect.Height; y++ {
		if ch, _, _ := screen.Get(p.Rect.X+p.Rect.Width-1, y); ch != "│" {
			found = true
		}
	}
	if !found {
		t.Error("an overflowing list with Scrollbar drew no scrollbar on the border")
	}
}

func TestDrawListMarkers(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	item := listItem(3, 1, 0, 5)
	item.Markers = true
	item.Rows[1].Marked = true
	spec := listOnlySpec(item)
	Draw(screen, 80, 24, spec, "hosts", styles)
	screen.Show()
	p := mustPlace(t, spec)
	it, _ := p.Item("hosts")
	row := func(i int) string { return tcelltest.TextAt(screen, it.Rect.X, it.Rect.Y+i, 14) }
	if got := row(0); got != " ( ) anchor 0 " {
		t.Errorf("row 0 = %q", got)
	}
	if got := row(1); got != " (*) buoy 1  " && got != " (*) buoy 1   " {
		t.Errorf("marked row = %q", got)
	}
	if _, st, _ := screen.Get(it.Rect.X+6, it.Rect.Y+1); st != styles.DialogOptionRowStyle(true, true) {
		t.Errorf("focused marked row style = %v", st)
	}
	if _, st, _ := screen.Get(it.Rect.X+6, it.Rect.Y); st != styles.DialogOptionRowStyle(false, false) {
		t.Errorf("other row style = %v", st)
	}
	// Rows past the data stay bare surface in an option-row list.
	if _, st, _ := screen.Get(it.Rect.X+6, it.Rect.Y+4); st != styles.DialogSurface {
		t.Errorf("unused row style = %v, want the dialog surface", st)
	}
}
