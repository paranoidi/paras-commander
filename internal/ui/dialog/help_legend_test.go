package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

var helpLegendIcons = []rune("αβγδεζηθικλμνξπρστυφχψωϊ")

// helpLegendSample is ~23 entries of varied label lengths with distinct icons and colours.
func helpLegendSample() []primitive.LegendItem {
	words := []string{"amber", "birch", "cinder", "dune", "ember", "fable", "glacier", "harbor", "ivory", "jasper"}
	legend := make([]primitive.LegendItem, 23)
	for i := range legend {
		label := words[i%len(words)]
		for j := 0; j < i%4; j++ {
			label += " " + words[(i+j+1)%len(words)]
		}
		legend[i] = primitive.LegendItem{
			Icon:  string(helpLegendIcons[i]),
			Style: tcell.StyleDefault.Foreground(tcell.NewRGBColor(int32(10+i*10), 200, 50)),
			Label: label,
		}
	}
	return legend
}

func helpLegendLabels(spec dlg.Spec) (labels []dlg.Item) {
	for _, it := range spec.Groups[0].Columns[0] {
		if it.Kind == dlg.KindLabel {
			labels = append(labels, it)
		}
	}
	return labels
}

func TestHelpDialogLegendGrowsDialogNotList(t *testing.T) {
	legend := helpLegendSample()
	layout := Layout{Width: 100, Height: 40}
	base, baseH, ok := HelpDialogView(HelpViewState{}, layout, theme.Default())
	if !ok {
		t.Fatal("base help dialog does not fit")
	}
	spec, listH, ok := HelpDialogView(HelpViewState{Legend: legend}, layout, theme.Default())
	if !ok {
		t.Fatal("help dialog with legend does not fit")
	}
	labels := helpLegendLabels(spec)
	if len(labels) < 2 {
		t.Fatalf("legend rows = %d, want a wrapped legend", len(labels))
	}
	if listH != baseH {
		t.Fatalf("listH = %d, want %d", listH, baseH)
	}
	if want := base.MaxHeight + 1 + len(labels); spec.MaxHeight != want {
		t.Fatalf("MaxHeight = %d, want %d", spec.MaxHeight, want)
	}
	m, ok := ComputeHelpDialogListMetrics(layout, legend)
	if !ok || m.ListH != listH {
		t.Fatalf("metrics ListH = %d (ok %v), want %d", m.ListH, ok, listH)
	}
}

func TestHelpDialogLegendLabelsNotTruncated(t *testing.T) {
	// 100 columns give an 86 wide dialog and 82 columns of label width: exact fills one line,
	// over would be truncated on one and so takes two.
	exact := []primitive.LegendItem{
		{Icon: "α", Label: strings.Repeat("a", 38)},
		{Icon: "β", Label: strings.Repeat("b", 37)},
	}
	over := []primitive.LegendItem{exact[0], {Icon: "β", Label: strings.Repeat("b", 38)}}
	checkHelpLegendLabels(t, helpLegendSample(), -1)
	checkHelpLegendLabels(t, exact, 1)
	checkHelpLegendLabels(t, over, 2)
}

func checkHelpLegendLabels(t *testing.T, legend []primitive.LegendItem, wantLines int) {
	t.Helper()
	layout := Layout{Width: 100, Height: 40}
	spec, _, ok := HelpDialogView(HelpViewState{Legend: legend}, layout, theme.Default())
	if !ok {
		t.Fatal("help dialog does not fit")
	}
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
	if !ok {
		t.Fatal("not placed")
	}
	built := helpLegendLabels(spec)
	if wantLines >= 0 && len(built) != wantLines {
		t.Fatalf("legend lines = %d, want %d", len(built), wantLines)
	}
	n := 0
	for _, pi := range p.Items {
		if pi.Kind != dlg.KindLabel {
			continue
		}
		if pi.Label != built[n].Label {
			t.Fatalf("placed label %q, built %q", pi.Label, built[n].Label)
		}
		if w := runewidth.StringWidth(pi.Label); w > pi.Rect.Width {
			t.Fatalf("label %q is %d wide in a %d wide rect", pi.Label, w, pi.Rect.Width)
		}
		n++
	}
	if n != len(built) {
		t.Fatalf("placed %d labels, built %d", n, len(built))
	}
}

func TestHelpDialogLegendIconPainted(t *testing.T) {
	legend := helpLegendSample()
	layout := Layout{Width: 100, Height: 40}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(layout.Width, layout.Height)
	DrawHelpDialog(screen, layout, HelpViewState{Open: true, Title: "Help", Legend: legend}, theme.Default(), uiscrollbar.StyleThumb)
	for _, it := range legend {
		want, _, _ := it.Style.Decompose()
		found := false
		for y := 0; y < layout.Height && !found; y++ {
			for x := 0; x < layout.Width; x++ {
				if c, st, _ := screen.Get(x, y); c == it.Icon {
					fg, _, _ := st.Decompose()
					if fg != want {
						t.Fatalf("icon %q fg = %v, want %v", it.Icon, fg, want)
					}
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("icon %q not drawn", it.Icon)
		}
	}
}

func TestHelpDialogLegendDroppedOnShortTerminal(t *testing.T) {
	legend := helpLegendSample()
	for _, layout := range []Layout{{Width: 100, Height: 40}, {Width: 80, Height: 12}} {
		t.Run(fmt.Sprintf("%dx%d", layout.Width, layout.Height), func(t *testing.T) {
			spec, listH, ok := HelpDialogView(HelpViewState{Legend: legend}, layout, theme.Default())
			if !ok {
				t.Fatal("help dialog does not fit")
			}
			if has := len(helpLegendLabels(spec)) > 0; has != (layout.Height == 40) {
				t.Fatalf("legend shown = %v", has)
			}
			m, ok := ComputeHelpDialogListMetrics(layout, legend)
			if !ok || m.ListH != listH {
				t.Fatalf("metrics ListH = %d (ok %v), want %d", m.ListH, ok, listH)
			}
		})
	}
}
