package dlg

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestBarFilledCols(t *testing.T) {
	cases := []struct {
		width int
		frac  float64
		want  int
	}{
		{56, 0.25, 14},
		{56, 0.0001, 1}, // started: never completely empty
		{56, 0, 0},
		{56, -1, 0},
		{56, 1, 56},
		{56, 3, 56}, // overshoot clamps
		{10, 0.55, 6},
		{0, 0.5, 0},
	}
	for _, c := range cases {
		if got := BarFilledCols(c.width, c.frac); got != c.want {
			t.Errorf("BarFilledCols(%d, %v) = %d, want %d", c.width, c.frac, got, c.want)
		}
	}
}

func barSpec(frac float64, label string) Spec {
	return Spec{
		Title: "Harbor", Width: 40, Unclamped: true,
		Groups: []Group{{Columns: [][]Item{{
			{Kind: KindLabel, Label: "Scanning"},
			{ID: "bar", Kind: KindBar, Label: label, Frac: frac},
			{Kind: KindCheckbox, ID: "c", Label: "Lantern", Mnemonic: 'l'},
		}}}},
		Buttons: []Button{{ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func TestBarPlacementAndFocus(t *testing.T) {
	spec := barSpec(0.5, "meadow")
	p := mustPlace(t, spec)
	bar, ok := p.Item("bar")
	if !ok {
		t.Fatal("bar not placed")
	}
	if bar.Kind != KindBar || bar.Rect.X != p.Rect.X+2 || bar.Rect.Width != p.Rect.Width-4 || bar.Rect.Y != p.Rect.Y+2 {
		t.Errorf("bar placed %+v in dialog %+v", bar, p.Rect)
	}
	if _, hit := HitTest(p, bar.Rect.X+1, bar.Rect.Y); hit {
		t.Error("bar is a hit target")
	}
	focus := ID("")
	if _, ok := HandleKey(spec, &focus, key(tcell.KeyDown)); !ok || focus != Cancel && focus != "c" {
		t.Errorf("down moved focus to %q", focus)
	}
	focus = "c"
	HandleKey(spec, &focus, key(tcell.KeyUp))
	if focus != "c" {
		t.Errorf("up from the first focusable item moved focus to %q: the bar is not a stop", focus)
	}
}

func TestDrawBar(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	styles := theme.Default()
	spec := barSpec(0.25, strings.Repeat("segment/", 12)+"filename.txt")
	Draw(screen, 80, 24, spec, "", styles)
	screen.Show()
	p := mustPlace(t, spec)
	bar, _ := p.Item("bar")
	_, fillBG, _ := styles.DialogProgressFill.Decompose()
	_, trackBG, _ := styles.DialogProgressTrack.Decompose()
	filled := BarFilledCols(bar.Rect.Width, 0.25)
	for col := 0; col < bar.Rect.Width; col++ {
		_, st, _ := screen.Get(bar.Rect.X+col, bar.Rect.Y)
		_, bg, _ := st.Decompose()
		want := trackBG
		if col < filled {
			want = fillBG
		}
		if bg != want {
			t.Fatalf("col %d bg %v, want %v", col, bg, want)
		}
	}
	// The label is fitted to the bar width and overlaid, also across the track.
	line := tcelltest.TextAt(screen, bar.Rect.X, bar.Rect.Y, bar.Rect.Width)
	if !strings.Contains(line, "filename.txt") || len([]rune(line)) != bar.Rect.Width {
		t.Errorf("bar line = %q", line)
	}
	// Cells outside the bar (the margins) keep the dialog surface.
	_, surface, _ := styles.DialogSurface.Decompose()
	_, marginStyle, _ := screen.Get(bar.Rect.X-1, bar.Rect.Y)
	if _, bg, _ := marginStyle.Decompose(); bg != surface {
		t.Errorf("left margin bg %v, want surface %v", bg, surface)
	}
}
