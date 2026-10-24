package tcelltest

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// DumpRect renders the w x h cell rect at (x, y) as golden text: the runes, a style legend, then
// one row of style letters per text row. Each distinct tcell.Style gets a letter in order of first
// appearance (row-major; colors print as raw tcell.Color values because Color.String is not
// stable for aliased names), so two renders are equal exactly when every rune and style is equal.
func DumpRect(screen tcell.SimulationScreen, x, y, w, h int) string {
	letters := map[tcell.Style]byte{}
	var order []tcell.Style
	var text, styles strings.Builder
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			ch, st, _ := screen.Get(col, row)
			if ch == "" {
				ch = " "
			}
			text.WriteString(ch)
			l, ok := letters[st]
			if !ok {
				l = byte('a' + len(order))
				letters[st] = l
				order = append(order, st)
			}
			styles.WriteByte(l)
		}
		text.WriteByte('\n')
		styles.WriteByte('\n')
	}
	var sb strings.Builder
	sb.WriteString("-- text --\n")
	sb.WriteString(text.String())
	sb.WriteString("-- legend --\n")
	for _, st := range order {
		fg, bg, attrs := st.Decompose()
		fmt.Fprintf(&sb, "%c fg=%#x bg=%#x attrs=%d ul=%d ulc=%#x\n", letters[st], uint64(fg), uint64(bg), attrs, st.GetUnderlineStyle(), uint64(st.GetUnderlineColor()))
	}
	sb.WriteString("-- styles --\n")
	sb.WriteString(styles.String())
	return sb.String()
}
