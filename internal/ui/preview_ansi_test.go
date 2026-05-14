package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestAnsiStyledCellsBoldRed(t *testing.T) {
	base := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorSilver)
	cells := AnsiStyledCells("\x1b[1;31mX\x1b[0m", base)
	if len(cells) != 1 || cells[0].R != 'X' {
		t.Fatalf("cells = %#v", cells)
	}
	fg, _, _ := cells[0].St.Decompose()
	if fg != tcell.PaletteColor(1) {
		t.Fatalf("fg = %v want red palette 1", fg)
	}
}

func TestAnsi256Color(t *testing.T) {
	base := tcell.StyleDefault
	cells := AnsiStyledCells("\x1b[38;5;196mZ\x1b[0m", base)
	if len(cells) != 1 || cells[0].R != 'Z' {
		t.Fatalf("cells = %#v", cells)
	}
	fg, _, _ := cells[0].St.Decompose()
	if fg != tcell.PaletteColor(196) {
		t.Fatalf("fg = %v want palette 196", fg)
	}
}

func TestWrapAnsiCellsBreaksWidth(t *testing.T) {
	base := tcell.StyleDefault
	s := "abcd"
	var cells []AnsiCell
	for _, r := range s {
		cells = append(cells, AnsiCell{R: r, St: base})
	}
	lines := WrapAnsiCells(cells, 2)
	if len(lines) != 2 {
		t.Fatalf("len(lines)=%d want 2: %#v", len(lines), lines)
	}
}
