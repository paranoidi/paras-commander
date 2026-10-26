package dlg

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// stripSpec has a three-row button strip: 3 + 3 + 1 buttons, like the file conflict prompt.
func stripSpec() Spec {
	b := func(id ID, label string, newRow bool) Button {
		return Button{ID: id, Label: label, Mnemonic: rune(label[0]), NewRow: newRow}
	}
	return Spec{
		Title: "Harbor", Width: 50, MinWidth: 20, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{{Kind: KindLabel, Label: "Moor the boat?"}, check("c0", "Lantern", 'l')}}}},
		Buttons: []Button{
			b("a", "Anchor", false), b("b", "Buoy", false), b("c", "Cable", false),
			b("d", "Dock", true), b("e", "Ferry", false), b("f", "Gangway", false),
			b("g", "Hull", true),
		},
	}
}

func TestButtonRowsPlacement(t *testing.T) {
	spec := stripSpec()
	p := mustPlace(t, spec)
	// label + checkbox + separator + 3 strip rows between the borders.
	if p.Rect.Height != 2+2+1+3 {
		t.Errorf("height = %d, want 8", p.Rect.Height)
	}
	if p.SepY != p.Rect.Y+3 || p.ButtonY != p.SepY+1 || p.ContentBottom != p.SepY {
		t.Errorf("SepY %d ButtonY %d ContentBottom %d in %+v", p.SepY, p.ButtonY, p.ContentBottom, p.Rect)
	}
	wantRow := map[ID]int{"a": 0, "b": 0, "c": 0, "d": 1, "e": 1, "f": 1, "g": 2}
	var lastX = map[int]int{}
	for id, row := range wantRow {
		pi, ok := p.Item(id)
		if !ok || pi.Rect.Y != p.ButtonY+row {
			t.Errorf("button %s placed %+v, want row %d", id, pi, row)
		}
		lastX[row] = max(lastX[row], pi.Rect.X+pi.Rect.Width)
	}
	// Every row is centered on its own.
	for row, right := range lastX {
		first := p.Rect.Width
		for id, r := range wantRow {
			if pi, _ := p.Item(id); r == row {
				first = min(first, pi.Rect.X-p.Rect.X)
			}
		}
		if left, rightMargin := first, p.Rect.X+p.Rect.Width-right; left-rightMargin > 1 || rightMargin-left > 1 {
			t.Errorf("row %d margins %d / %d, not centered", row, left, rightMargin)
		}
	}
}

func TestButtonRowsWidensToWidestRow(t *testing.T) {
	spec := stripSpec()
	spec.Width = 10
	spec.Unclamped = false
	spec.MinWidth = 10
	p := mustPlace(t, spec)
	if p.Rect.Width < 3+3+3+2 { // three buttons of the first row, borders
		t.Errorf("width = %d, too narrow for the widest row", p.Rect.Width)
	}
	for _, id := range []ID{"a", "b", "c", "d", "e", "f", "g"} {
		pi, _ := p.Item(id)
		if pi.Rect.X < p.Rect.X+1 || pi.Rect.X+pi.Rect.Width > p.Rect.X+p.Rect.Width-1 {
			t.Errorf("button %s %+v outside dialog %+v", id, pi.Rect, p.Rect)
		}
	}
}

func TestButtonRowsNavigation(t *testing.T) {
	spec := stripSpec()
	cases := []struct {
		name string
		from ID
		key  tcell.Key
		want ID
	}{
		{"right along the row", "a", tcell.KeyRight, "b"},
		{"right continues on the next row", "c", tcell.KeyRight, "d"},
		{"left continues on the previous row", "d", tcell.KeyLeft, "c"},
		{"right at the last button stays", "g", tcell.KeyRight, "g"},
		{"left at the first button stays", "a", tcell.KeyLeft, "a"},
		{"down keeps the relative position", "a", tcell.KeyDown, "d"},
		{"down from the middle", "b", tcell.KeyDown, "e"},
		{"down from the right", "c", tcell.KeyDown, "f"},
		{"down to a single button row", "e", tcell.KeyDown, "g"},
		{"down from the last row stays", "g", tcell.KeyDown, "g"},
		{"up from the single button row", "g", tcell.KeyUp, "e"},
		{"up keeps the relative position", "f", tcell.KeyUp, "c"},
		{"up from the first row leaves the strip", "b", tcell.KeyUp, "c0"},
	}
	for _, c := range cases {
		focus := c.from
		if _, ok := HandleKey(spec, &focus, key(c.key)); !ok || focus != c.want {
			t.Errorf("%s: %s %v -> %q, want %q", c.name, c.from, c.key, focus, c.want)
		}
	}
	// The strip is one Tab stop.
	focus := ID("a")
	HandleKey(spec, &focus, key(tcell.KeyTab))
	if focus != "c0" {
		t.Errorf("Tab from the strip = %q, want the checkbox", focus)
	}
	HandleKey(spec, &focus, key(tcell.KeyTab))
	if focus != "a" {
		t.Errorf("Tab onto the strip = %q, want its first button", focus)
	}
	// Mnemonics reach every row.
	focus = "a"
	if ev, ok := HandleKey(spec, &focus, runeKey('h', tcell.ModAlt)); !ok || ev.ID != "g" || focus != "g" {
		t.Errorf("Alt+h = %+v focus %q, want g", ev, focus)
	}
}

func TestButtonRowsClickAndClip(t *testing.T) {
	spec := stripSpec()
	p := mustPlace(t, spec)
	g, _ := p.Item("g")
	if id, ok := HitTest(p, g.Rect.X+1, g.Rect.Y); !ok || id != "g" {
		t.Errorf("hit on the third row = %q %v", id, ok)
	}
	// A clamped height keeps the whole strip and cuts the content.
	spec.Unclamped = true
	short, ok := Place(80, 7, spec)
	if !ok {
		t.Fatal("Place not ok on a short screen")
	}
	if short.SepY+4 != short.Rect.Y+short.Rect.Height-1 || short.ContentBottom != short.SepY {
		t.Errorf("short placement sep %d in %+v: strip rows not kept", short.SepY, short.Rect)
	}
}

func TestDrawButtonRows(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	spec := stripSpec()
	Draw(screen, 80, 24, spec, "d", theme.Default())
	screen.Show()
	p := mustPlace(t, spec)
	rows := []string{"Anchor", "Dock", "Hull"}
	for i, want := range rows {
		line := tcelltest.TextAt(screen, p.Rect.X, p.ButtonY+i, p.Rect.Width)
		if !strings.Contains(line, want) {
			t.Errorf("strip row %d = %q, want %q", i, line, want)
		}
	}
	if line := tcelltest.TextAt(screen, p.Rect.X, p.SepY, p.Rect.Width); !strings.HasPrefix(line, "├") {
		t.Errorf("separator row = %q", line)
	}
}

func TestFocusedResolvesStaleFocus(t *testing.T) {
	spec := testSpec()
	if got := Focused(spec, ""); got != "a0" {
		t.Errorf("Focused(empty) = %q, want the first widget", got)
	}
	if got := Focused(spec, "gone"); got != "a0" {
		t.Errorf("Focused(stale) = %q, want the first widget", got)
	}
	if got := Focused(spec, "c1"); got != "c1" {
		t.Errorf("Focused(c1) = %q", got)
	}
}

func TestDrawWarnLabelStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := Spec{Title: "Harbor", Width: 40, Unclamped: true,
		Groups:  []Group{{Columns: [][]Item{{{Kind: KindLabel, Label: "Low water.", Style: TextWarn}}}}},
		Buttons: []Button{{ID: OK, Label: "OK"}}}
	Draw(screen, 80, 24, spec, "", styles)
	screen.Show()
	p := mustPlace(t, spec)
	pi := p.Items[0]
	_, surface, _ := styles.DialogSurface.Decompose()
	_, got, _ := screen.Get(pi.Rect.X, pi.Rect.Y)
	if got != styles.MessageWarn.Background(surface) {
		t.Errorf("warn label style = %v, want %v", got, styles.MessageWarn.Background(surface))
	}
}
