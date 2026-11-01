package previewpanel

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSearchMarkRows(t *testing.T) {
	hl := tcell.StyleDefault.Foreground(tcell.ColorRed)
	s := SearchState{Active: true, Matches: []SearchMatch{{}}, MatchStyle: hl, CurrentStyle: hl.Bold(true)}
	lines := make([][]AnsiCell, 100)
	for i := range lines {
		lines[i] = []AnsiCell{{R: 'a'}}
	}
	lines[55] = []AnsiCell{{R: 'b', St: hl}}
	for i, m := range s.searchMarkRows(lines, 10) {
		if m != (i == 5) {
			t.Fatalf("row %d marked=%v", i, m)
		}
	}
}
