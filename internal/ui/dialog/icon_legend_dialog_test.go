package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// iconLegendSample is 23 entries of varied label lengths with distinct icons and colours.
func iconLegendSample() []primitive.LegendItem {
	icons := []rune("αβγδεζηθικλμνξπρστυφχψωϊ")
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper"}
	legend := make([]primitive.LegendItem, 23)
	for i := range legend {
		label := words[i%len(words)]
		for j := 0; j < i%4; j++ {
			label += " " + words[(i+j+1)%len(words)]
		}
		legend[i] = primitive.LegendItem{
			Icon:  string(icons[i]),
			Style: tcell.StyleDefault.Foreground(tcell.NewRGBColor(int32(10+i*10), 200, 50)),
			Label: label,
		}
	}
	return legend
}

func legendListOf(t *testing.T, state IconLegendDialogState, w, h int) (dlg.Spec, dlg.Item) {
	t.Helper()
	spec, ok := IconLegendDialogView(state, Layout{Width: w, Height: h}, theme.Default())
	if !ok {
		t.Fatalf("legend does not fit %dx%d", w, h)
	}
	list, ok := spec.Item(iconLegendList)
	if !ok {
		t.Fatal("no list")
	}
	return spec, list
}

func TestIconLegendOneRowPerEntryAllVisible(t *testing.T) {
	items := iconLegendSample()
	_, list := legendListOf(t, IconLegendDialogState{Open: true, Items: items}, 100, 40)
	if list.Height != len(items) || list.RowCount != len(items) {
		t.Fatalf("height %d rows %d, want %d", list.Height, list.RowCount, len(items))
	}
	for i, r := range list.Rows {
		if want := items[i].Icon + " " + items[i].Label; r.Text != want {
			t.Fatalf("row %d = %q, want %q", i, r.Text, want)
		}
		if len(r.Spans) != 1 || r.Spans[0].Start != 0 || r.Spans[0].End != 1 {
			t.Fatalf("row %d spans = %+v, want the icon only", i, r.Spans)
		}
	}
}

func TestIconLegendShortScreenScrolls(t *testing.T) {
	items := iconLegendSample()
	_, list := legendListOf(t, IconLegendDialogState{Open: true, Items: items}, 80, 14)
	if list.Height >= len(items) || list.Height < iconLegendMinListH {
		t.Fatalf("height = %d", list.Height)
	}
	_, list = legendListOf(t, IconLegendDialogState{Open: true, Items: items, Scroll: len(items)}, 80, 14)
	last := list.RowsFrom + len(list.Rows) - 1
	if last != len(items)-1 || list.Rows[len(list.Rows)-1].Text != items[last].Icon+" "+items[last].Label {
		t.Fatalf("last visible row = %d", last)
	}
}

func TestIconLegendKeysScrollWithCloseFocused(t *testing.T) {
	state := IconLegendDialogState{Open: true, Items: iconLegendSample()}
	spec, _ := legendListOf(t, state, 80, 14)
	ev, ok := dlg.HandleKey(spec, &state.Focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if !ok || ev.Kind != dlg.EventListMove || ev.Offset != 1 {
		t.Fatalf("Down = %+v, %v", ev, ok)
	}
}

func TestIconLegendDrawsIconInOwnColourAndFullLabel(t *testing.T) {
	long := strings.Repeat("word ", 11) + "end"
	items := append(iconLegendSample(), primitive.LegendItem{
		Icon: "Ω", Style: tcell.StyleDefault.Foreground(tcell.NewRGBColor(250, 10, 20)), Label: long,
	})
	state := IconLegendDialogState{Open: true, Items: items}
	screen := newSimScreen(t, 100, 40)
	DrawIconLegendDialog(screen, Layout{Width: 100, Height: 40}, state, theme.Default(), "")
	spec, _ := legendListOf(t, state, 100, 40)
	p, ok := dlg.Place(100, 40, spec)
	if !ok {
		t.Fatal("no placement")
	}
	list, _ := p.Item(iconLegendList)
	y := list.Rect.Y + len(items) - 1
	var row strings.Builder
	for x := list.Rect.X; x < list.Rect.X+list.Rect.Width; x++ {
		str, _, _ := screen.Get(x, y)
		row.WriteString(str)
	}
	if !strings.Contains(row.String(), "Ω "+long) {
		t.Fatalf("row = %q, want the full label", row.String())
	}
	for x := list.Rect.X; x < list.Rect.X+list.Rect.Width; x++ {
		if str, st, _ := screen.Get(x, y); str == "Ω" {
			if fg, _, _ := st.Decompose(); fg != tcell.NewRGBColor(250, 10, 20) {
				t.Fatalf("icon fg = %v", fg)
			}
			return
		}
	}
	t.Fatal("icon not drawn")
}
