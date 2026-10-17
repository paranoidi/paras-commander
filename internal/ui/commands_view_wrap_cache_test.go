package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestCommandsStreamPanelsCacheInvalidatesOnLengthChange(t *testing.T) {
	col := Rect{X: 0, Y: 0, Width: 40, Height: 20}
	e := CommandRunEntry{ID: "cache-a", Stdout: "alpha\nbeta"}
	_, _, out1, _ := CommandsStreamPanels(col, e, tcell.StyleDefault)
	_, _, out2, _ := CommandsStreamPanels(col, e, tcell.StyleDefault)
	if len(out1) != 2 || len(out2) != 2 || &out1[0] != &out2[0] {
		t.Fatalf("second call should reuse cached lines: %d %d", len(out1), len(out2))
	}
	e.Stdout += "\ngamma"
	_, _, out3, _ := CommandsStreamPanels(col, e, tcell.StyleDefault)
	if len(out3) != 3 {
		t.Fatalf("cache not invalidated on growth: %d lines", len(out3))
	}
}
