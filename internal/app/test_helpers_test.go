package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
)

func TestTextAtWideGlyphOccupiesTwoColumns(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(8, 2)
	screen.SetContent(0, 0, '中', nil, tcell.StyleDefault)

	got := tcelltest.TextAt(screen, 0, 0, 2)
	if got != "中" {
		t.Fatalf("TextAt = %q, want 中 (one rune for two columns)", got)
	}
	if line := screenLine(screen, 0, 2); line != "中" {
		t.Fatalf("screenLine = %q, want 中", line)
	}
}
