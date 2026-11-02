package dlg

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const tooltipText = "Lantern meadow willow"

func tooltipSpec(listHeight int, area ID) Spec {
	rows := make([]ListRow, 8)
	for i := range rows {
		rows[i] = ListRow{Text: "orchard", Icon: " "}
	}
	return Spec{
		Title: "Harbor", Width: 50, MinWidth: 30, Unclamped: true, TooltipArea: area,
		Groups: []Group{{Columns: [][]Item{{
			{ID: "top", Kind: KindInput},
			{ID: "tip", Kind: KindInput, Tooltip: tooltipText},
			{Kind: KindLabel, Label: "ember"},
			{ID: "rows", Kind: KindList, Rows: rows, Height: listHeight},
		}}}},
		Buttons: OKCancelButtons(),
	}
}

func drawTooltipSpec(t *testing.T, spec Spec, focus ID) (tcell.SimulationScreen, Placement) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 30)
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	p, ok := Draw(screen, 80, 30, spec, focus, styles)
	if !ok {
		t.Fatal("dialog does not fit")
	}
	return screen, p
}

func rowText(screen tcell.SimulationScreen, r Rect) string {
	var b strings.Builder
	for x := r.X; x < r.X+r.Width; x++ {
		str, _, _ := screen.Get(x, r.Y)
		b.WriteString(str)
	}
	return b.String()
}

func TestTooltipShowsOnlyForFocusedWidget(t *testing.T) {
	_, p := drawTooltipSpec(t, tooltipSpec(6, ""), "top")
	if p.Tooltip != (Rect{}) {
		t.Fatalf("tooltip drawn for an unfocused widget: %+v", p.Tooltip)
	}
	screen, p := drawTooltipSpec(t, tooltipSpec(6, ""), "tip")
	tip, _ := p.Item("tip")
	if p.Tooltip.Y != tip.Rect.Y+1 || p.Tooltip.X != tip.Rect.X {
		t.Fatalf("tooltip at %+v, want just below the input at %+v", p.Tooltip, tip.Rect)
	}
	got := rowText(screen, p.Tooltip)
	if !strings.HasPrefix(got, " "+tooltipText) || !strings.HasSuffix(got, theme.IconDialogInfoASCII+" ") {
		t.Fatalf("tooltip row = %q, want the text then the info icon at the right", got)
	}
}

func TestTooltipCoversTooltipArea(t *testing.T) {
	_, p := drawTooltipSpec(t, tooltipSpec(6, "rows"), "tip")
	list, _ := p.Item("rows")
	if p.Tooltip.X != list.Rect.X || p.Tooltip.Y != list.Rect.Y {
		t.Fatalf("tooltip at %+v, want the list's top-left corner %+v", p.Tooltip, list.Rect)
	}
	var drawn []int
	p.EachListRow("rows", func(row, _ int) { drawn = append(drawn, row) })
	if len(drawn) != list.Rect.Height-p.Tooltip.Height || drawn[0] != p.Tooltip.Height {
		t.Fatalf("EachListRow rows = %v, want the rows below the %d-row tooltip", drawn, p.Tooltip.Height)
	}
}

func TestTooltipFallsBackWhenItDoesNotFit(t *testing.T) {
	spec := tooltipSpec(2, "rows")
	spec.Groups[0].Columns[0][1].Tooltip = strings.Repeat("glacier quartz thistle ", 5) // three lines: taller than the list
	_, p := drawTooltipSpec(t, spec, "tip")
	tip, _ := p.Item("tip")
	if p.Tooltip.Height != 3 || p.Tooltip.Y != tip.Rect.Y+1 {
		t.Fatalf("tooltip at %+v, want three rows below the input (row %d)", p.Tooltip, tip.Rect.Y+1)
	}
	if p.Tooltip.Y+p.Tooltip.Height > p.ContentBottom || p.Tooltip.X+p.Tooltip.Width > p.Rect.X+p.Rect.Width-1 {
		t.Fatalf("tooltip %+v leaves the content area of %+v (bottom %d)", p.Tooltip, p.Rect, p.ContentBottom)
	}

	spec.Groups[0].Columns[0][1].Tooltip = strings.Repeat("glacier quartz thistle ", 30)
	if _, p = drawTooltipSpec(t, spec, "tip"); p.Tooltip != (Rect{}) {
		t.Fatalf("a tooltip that fits nowhere was drawn at %+v", p.Tooltip)
	}
}
