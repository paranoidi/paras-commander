package app

import (
	"slices"

	"github.com/gdamore/tcell/v2"
)

// vsStripScreen drops emoji/text variation selectors (U+FE0E/U+FE0F) before tcell sees
// them: tcell (uniseg), go-runewidth and the terminal disagree on the width of
// VS-modified clusters (e.g. ⚪︎ = 1 vs 2 cells), which shifts the rest of the row.
// ponytail: only SetContent is overridden; nothing calls Put/PutStr — override those too if that changes.
type vsStripScreen struct{ tcell.Screen }

func isVS(r rune) bool { return r == '\uFE0E' || r == '\uFE0F' }

func (s vsStripScreen) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	if isVS(mainc) {
		mainc = ' ' // lone selector painted as its own cell keeps its one-cell slot
	}
	if slices.ContainsFunc(combc, isVS) {
		combc = slices.DeleteFunc(slices.Clone(combc), isVS)
	}
	s.Screen.SetContent(x, y, mainc, combc, style)
}
