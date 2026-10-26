package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// listHeight is the height of list "hosts" in spec fitted to an 80 x screenH screen.
func listHeight(t *testing.T, screenH int, spec Spec) (int, bool) {
	t.Helper()
	fitted, ok := FitSpec(80, screenH, spec)
	list, _ := fitted.Item("hosts")
	return list.Height, ok
}

// listOnlySpec's frame is the list plus 4 rows: the borders, the button separator and the strip.
func TestFitSpecShrinksToTheScreenAndMatchesPlace(t *testing.T) {
	spec := listOnlySpec(listItem(30, 0, 0, 30))
	h, ok := listHeight(t, 14, spec)
	if !ok || h != 10 {
		t.Fatalf("list height on a 14-row screen = %d (%v), want 10", h, ok)
	}
	fitted, _ := FitSpec(80, 14, spec)
	if again, _ := FitSpec(80, 14, fitted); again.Groups[0].Columns[0][0].Height != h {
		t.Error("fitting a fitted spec changed its list height")
	}
	p, _ := Place(80, 14, spec)
	if it, _ := p.Item("hosts"); it.Rect.Height != h {
		t.Errorf("Place drew %d list rows, FitSpec says %d", it.Rect.Height, h)
	}
	if spec.Groups[0].Columns[0][0].Height != 30 {
		t.Error("FitSpec modified the spec it was given")
	}
}

func TestMaxHeightCapsListsButNotFixedContent(t *testing.T) {
	spec := listOnlySpec(listItem(30, 0, 0, 30))
	spec.MaxHeight = 12
	if h, _ := listHeight(t, 40, spec); h != 8 {
		t.Errorf("list height under MaxHeight 12 = %d, want 8", h)
	}
	// Five labels above the list: at MaxHeight 6 the list gives up all but one row and the cap gives
	// way to the labels instead of cutting them off.
	col := []Item{{Kind: KindLabel, Label: "a"}, {Kind: KindLabel, Label: "b"}, {Kind: KindLabel, Label: "c"}, {Kind: KindLabel, Label: "d"}, {Kind: KindLabel, Label: "e"}, listItem(30, 0, 0, 30)}
	spec.Groups = []Group{{Columns: [][]Item{col}}}
	spec.MaxHeight = 6
	p, ok := Place(80, 40, spec)
	if !ok || p.Rect.Height != 10 {
		t.Fatalf("frame = %d rows (%v), want 10: five labels, a one-row list and the chrome", p.Rect.Height, ok)
	}
	if it, _ := p.Item("hosts"); it.Rect.Height != 1 {
		t.Errorf("list height = %d, want 1", it.Rect.Height)
	}
}

func TestMinHeight(t *testing.T) {
	list := listItem(30, 0, 0, 30)
	list.MinHeight = 4
	spec := listOnlySpec(list)
	// MaxHeight gives way to the list's MinHeight.
	spec.MaxHeight = 5
	if h, ok := listHeight(t, 40, spec); !ok || h != 4 {
		t.Errorf("list height under MaxHeight 5 = %d (%v), want its MinHeight 4", h, ok)
	}
	// An Unclamped dialog shrinks to the screen; below MinHeight it does not fit.
	spec.MaxHeight = 0
	if h, ok := listHeight(t, 8, spec); !ok || h != 4 {
		t.Errorf("8-row screen: %d (%v), want 4", h, ok)
	}
	if _, ok := listHeight(t, 7, spec); ok {
		t.Error("7-row screen: fits, want !ok (the list would drop below its MinHeight)")
	}
	if _, ok := Place(80, 7, spec); ok {
		t.Error("Place on a 7-row screen: ok, want !ok")
	}
	// Margin keeps a row free above and below.
	spec.Margin = true
	if _, ok := listHeight(t, 9, spec); ok {
		t.Error("Margin on a 9-row screen: fits, want !ok")
	}
	if h, ok := listHeight(t, 10, spec); !ok || h != 4 {
		t.Errorf("Margin on a 10-row screen: %d (%v), want 4", h, ok)
	}
}

// A list and a spacer side by side give up rows together: the taller column shrinks first.
func TestShrinkSideBySide(t *testing.T) {
	list := listItem(30, 0, 0, 10)
	list.MinHeight = 4
	spec := Spec{
		Title: "Harbor", Width: 60, Unclamped: true,
		Groups:  []Group{{ColWidths: []int{20}, Columns: [][]Item{{list}, {{ID: "pane", Kind: KindSpacer, Height: 18}}}}},
		Buttons: OKCancelButtons(),
	}
	p, ok := Place(80, 16, spec) // 12 rows for the group
	if !ok {
		t.Fatal("does not fit")
	}
	l, _ := p.Item("hosts")
	pane, _ := p.Item("pane")
	if l.Rect.Height != 10 || pane.Rect.Height != 12 {
		t.Errorf("16-row screen: list %d, pane %d, want 10 and 12 (only the taller pane shrinks)", l.Rect.Height, pane.Rect.Height)
	}
	p, _ = Place(80, 12, spec) // 8 rows
	l, _ = p.Item("hosts")
	pane, _ = p.Item("pane")
	if l.Rect.Height != 8 || pane.Rect.Height != 8 {
		t.Errorf("12-row screen: list %d, pane %d, want both 8", l.Rect.Height, pane.Rect.Height)
	}
}

func TestScrollOnlyPagesByTheFittedHeight(t *testing.T) {
	list := listItem(30, 0, 0, 30)
	list.ScrollOnly = true
	spec, _ := FitSpec(80, 14, listOnlySpec(list)) // 10 rows
	focus := OK
	if ev, ok := HandleKey(spec, &focus, key(tcell.KeyPgDn)); !ok || ev.Offset != 10 {
		t.Errorf("PgDn = (%+v, %v), want offset 10 (a full fitted page)", ev, ok)
	}
}

// A MustFit dialog does not draw where content would be cut off; without it the content is cut
// above the button separator.
func TestMustFit(t *testing.T) {
	col := []Item{check("c0", "Lantern", 'l'), check("c1", "Harbor", 'h'), check("c2", "Willow", 'w')}
	spec := Spec{Title: "Harbor", Width: 40, Groups: []Group{{Columns: [][]Item{col}}}, Buttons: OKCancelButtons()}
	// The frame is the three rows plus 4 (the borders, the button separator and the strip), and a
	// clamped dialog keeps a row free above and below: a 9-row screen fits it exactly.
	if _, ok := Place(80, 8, spec); !ok {
		t.Fatal("without MustFit an 8-row screen still draws, cut off")
	}
	spec.MustFit = true
	if _, ok := Place(80, 8, spec); ok {
		t.Error("MustFit on an 8-row screen: drawn, want !ok")
	}
	if _, ok := FitSpec(80, 8, spec); ok {
		t.Error("FitSpec of MustFit on an 8-row screen: ok, want !ok")
	}
	if _, ok := Place(80, 9, spec); !ok {
		t.Error("MustFit on a 9-row screen: not drawn, want the whole dialog")
	}
}
