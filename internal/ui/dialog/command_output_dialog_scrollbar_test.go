package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestCommandOutputDialogScrollbar(t *testing.T) {
	words := []string{"apple", "river", "cloud", "table", "green", "stone", "maple", "bridge"}
	for _, tc := range []struct {
		name  string
		lines int
		want  bool
	}{{"overflow", 100, true}, {"fits", 3, false}} {
		t.Run(tc.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			defer screen.Fini()
			screen.SetSize(80, 24)
			layout := Layout{Width: 80, Height: 24}
			state := CommandOutputDialogState{Open: true, Title: "Output"}
			for i := range tc.lines {
				state.Lines = append(state.Lines, words[i%len(words)])
			}
			DrawCommandOutputDialog(screen, layout, state, theme.Default(), uiscrollbar.StyleBar)
			m, ok := ComputeCommandOutputDialogMetrics(layout, state)
			if !ok {
				t.Fatal("metrics")
			}
			got := false
			for row := range m.ListH {
				ch, _, _ := screen.Get(m.Rect.X+m.Rect.Width-1, m.Rect.Y+1+row)
				if ch != "│" {
					got = true
				}
			}
			if got != tc.want {
				t.Fatalf("scrollbar drawn = %v, want %v", got, tc.want)
			}
		})
	}
}
