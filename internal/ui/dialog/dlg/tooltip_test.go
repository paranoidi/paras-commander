package dlg

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const tooltipText = "Lantern meadow willow"

func tooltipSpec(listHeight int) Spec {
	rows := make([]ListRow, 8)
	for i := range rows {
		rows[i] = ListRow{Text: "orchard", Icon: " "}
	}
	return Spec{
		Title: "Harbor", Width: 50, MinWidth: 30, Unclamped: true,
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
	return drawTooltipSpecHidden(t, spec, focus, false)
}

func drawTooltipSpecHidden(t *testing.T, spec Spec, focus ID, hide bool) (tcell.SimulationScreen, Placement) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 30)
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	styles.HideTooltips = hide
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
	_, p := drawTooltipSpec(t, tooltipSpec(6), "top")
	if p.Tooltip != (Rect{}) {
		t.Fatalf("tooltip drawn for an unfocused widget: %+v", p.Tooltip)
	}
	screen, p := drawTooltipSpec(t, tooltipSpec(6), "tip")
	tip, _ := p.Item("tip")
	if p.Tooltip.Y != tip.Rect.Y+1 || p.Tooltip.X != tip.Rect.X {
		t.Fatalf("tooltip at %+v, want just below the input at %+v", p.Tooltip, tip.Rect)
	}
	got := rowText(screen, p.Tooltip)
	if want := " " + theme.IconDialogInfoASCII + " " + tooltipText + " "; got != want {
		t.Fatalf("tooltip row = %q, want %q: the info icon at the left, then the text", got, want)
	}
}

// A box wider than the room right of its widget shifts left just enough to fit, and list rows
// whose first cell it covers are left to it by EachListRow.
func TestTooltipShiftsLeftToFit(t *testing.T) {
	spec := tooltipSpec(6)
	spec.Groups[0].Columns[0][1] = Item{Kind: KindRow, Cells: []Item{
		{Kind: KindLabel, Label: "ember", Width: 30},
		{ID: "tip", Kind: KindInput, Tooltip: strings.Repeat("glacier quartz thistle ", 2)},
	}}
	_, p := drawTooltipSpec(t, spec, "tip")
	tip, _ := p.Item("tip")
	right := p.Rect.X + p.Rect.Width - 1
	if p.Tooltip.X >= tip.Rect.X || p.Tooltip.X+p.Tooltip.Width != right {
		t.Fatalf("tooltip %+v, want it shifted left of the input (x %d) to end at the border (%d)", p.Tooltip, tip.Rect.X, right)
	}
	list, _ := p.Item("rows")
	var drawn []int
	p.EachListRow("rows", func(row, _ int) { drawn = append(drawn, row) })
	covered := 0
	if p.Tooltip.Contains(list.Rect.X, list.Rect.Y) {
		covered = p.Tooltip.Height
	}
	if len(drawn) != list.Rect.Height-covered {
		t.Fatalf("EachListRow rows = %v, want %d rows (tooltip covers the first cell of %d)", drawn, list.Rect.Height-covered, covered)
	}
}

func TestTooltipMultilineAndUnfitting(t *testing.T) {
	spec := tooltipSpec(2)
	spec.Groups[0].Columns[0][1].Tooltip = strings.Repeat("glacier quartz thistle ", 5) // three lines
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

func TestTooltipHiddenByTheme(t *testing.T) {
	if _, p := drawTooltipSpecHidden(t, tooltipSpec(6), "tip", true); p.Tooltip != (Rect{}) {
		t.Fatalf("HideTooltips: tooltip drawn at %+v", p.Tooltip)
	}
}

func TestTooltipNewlineStartsLine(t *testing.T) {
	spec := tooltipSpec(6)
	spec.Groups[0].Columns[0][1].Tooltip = "Lantern meadow.\nWillow orchard."
	screen, p := drawTooltipSpec(t, spec, "tip")
	if p.Tooltip.Height != 2 {
		t.Fatalf("tooltip height = %d, want a line per sentence", p.Tooltip.Height)
	}
	second := p.Tooltip
	second.Y++
	if got := strings.TrimSpace(rowText(screen, second)); got != "Willow orchard." {
		t.Fatalf("second line = %q", got)
	}
}

func TestTooltipGate(t *testing.T) {
	var g tooltipGate
	const delay = 800 * time.Millisecond
	t0 := time.Now()
	k1 := tooltipKey{Rect{X: 1, Y: 1, Width: 40, Height: 10}, Rect{X: 3, Y: 3, Width: 20, Height: 1}, "lantern"}
	k2 := tooltipKey{Rect{X: 1, Y: 1, Width: 40, Height: 10}, Rect{X: 3, Y: 5, Width: 20, Height: 1}, "lantern"}
	woke := make(chan struct{}, 4)
	wake := func() { woke <- struct{}{} }

	if g.due(k1, t0, delay, wake) {
		t.Fatal("shown at once")
	}
	if g.due(k1, t0.Add(delay-time.Millisecond), delay, wake) {
		t.Fatal("shown before the delay")
	}
	if !g.due(k1, t0.Add(delay), delay, wake) {
		t.Fatal("not shown after the delay")
	}
	if g.due(k2, t0.Add(delay+time.Second), delay, wake) {
		t.Fatal("another widget inherited the wait")
	}
	if !g.due(k2, t0.Add(2*delay+time.Second), delay, wake) {
		t.Fatal("other widget not shown after its own delay")
	}
	g.clear(Rect{X: 9, Y: 9, Width: 1, Height: 1}) // another dialog's frame: untouched
	if !g.due(k2, t0.Add(4*delay), delay, wake) {
		t.Fatal("clear of another frame reset the tooltip")
	}
	g.clear(k2.frame)
	if g.due(k2, t0.Add(5*delay), delay, wake) {
		t.Fatal("shown at once after a frame without tooltip")
	}
	if !g.due(k1, t0, 0, wake) {
		t.Fatal("zero delay must show at once")
	}
}

func TestTooltipGateWakesOnce(t *testing.T) {
	var g tooltipGate
	woke := make(chan struct{}, 4)
	wake := func() { woke <- struct{}{} }
	k := tooltipKey{text: "willow"}
	now := time.Now()
	g.due(k, now, 20*time.Millisecond, wake)
	g.due(k, now, 20*time.Millisecond, wake) // redraw while pending: no second timer
	select {
	case <-woke:
	case <-time.After(time.Second):
		t.Fatal("no wake-up")
	}
	select {
	case <-woke:
		t.Fatal("more than one wake-up")
	case <-time.After(60 * time.Millisecond):
	}
}

func TestPaintTooltipForDelay(t *testing.T) {
	spec := tooltipSpec(2)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 30)
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	styles.TooltipDelay = time.Hour
	p, ok := Draw(screen, 80, 30, spec, "tip", styles)
	if !ok {
		t.Fatal("dialog does not fit")
	}
	if p.Tooltip != (Rect{}) {
		t.Fatal("tooltip painted before the delay")
	}
	Draw(screen, 80, 30, spec, "top", styles) // focus left: state reset
	theTooltipGate.mu.Lock()
	pending := theTooltipGate.set
	theTooltipGate.mu.Unlock()
	if pending {
		t.Fatal("gate kept state after a frame without tooltip")
	}
}
