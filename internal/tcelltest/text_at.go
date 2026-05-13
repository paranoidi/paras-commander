// Package tcelltest holds small helpers for tests that drive tcell.SimulationScreen.
package tcelltest

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// TextAt reads up to width terminal cells starting at (x, y), returning one rune per
// logical cell (wide glyphs consume their full width).
func TextAt(screen tcell.SimulationScreen, x, y, width int) string {
	runes := make([]rune, 0, width)
	for col := 0; col < width; {
		str, _, cw := screen.Get(x+col, y)
		if cw < 1 {
			cw = 1
		}
		r := ' '
		if str != "" {
			r, _ = utf8.DecodeRuneInString(str)
		}
		runes = append(runes, r)
		col += cw
	}
	return string(runes)
}
