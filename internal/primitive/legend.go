package primitive

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// LegendItem is one "icon label" entry of an icon legend. Style's foreground and attributes color
// the icon; the surface it is drawn on supplies the background.
type LegendItem struct {
	Icon  string
	Style tcell.Style
	Label string
}

// LegendGap is the number of blank columns between legend entries.
const LegendGap = 3

// Width is the columns the entry needs: icon, space, label.
func (it LegendItem) Width() int {
	return utf8.RuneCountInString(it.Icon) + 1 + utf8.RuneCountInString(it.Label)
}

// IconStyle is the icon's style on a surface with background bg.
func (it LegendItem) IconStyle(bg tcell.Color) tcell.Style {
	fg, _, attrs := it.Style.Decompose()
	return tcell.StyleDefault.Foreground(fg).Background(bg).Attributes(attrs)
}

// LegendLines flows the legend entries left to right into lines of at most width
// columns (each line holds legend indices); an entry wider than a line gets a line of its own
// and is truncated when drawn.
func LegendLines(legend []LegendItem, width int) [][]int {
	var lines [][]int
	used := 0
	for i, it := range legend {
		w := it.Width()
		if n := len(lines); n > 0 && used+LegendGap+w <= width {
			lines[n-1] = append(lines[n-1], i)
			used += LegendGap + w
			continue
		}
		lines = append(lines, []int{i})
		used = w
	}
	return lines
}
