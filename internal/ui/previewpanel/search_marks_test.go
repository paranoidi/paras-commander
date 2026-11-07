package previewpanel

import "testing"

func TestSearchMarkRows(t *testing.T) {
	cells := make([]AnsiCell, 0, 200)
	for range 100 {
		cells = append(cells, AnsiCell{R: 'a'}, AnsiCell{R: '\n'})
	}
	rows, lineRows := wrapRows(cells, 10, 0)
	l := &wrapLayout{cells: cells, rows: rows[:100], lineRows: lineRows}
	s := SearchState{Active: true, Matches: []SearchMatch{{Start: 110, End: 111, Line: 55}}}
	for i, m := range s.searchMarkRows(l, 10) {
		if m != (i == 5) {
			t.Fatalf("row %d marked=%v", i, m)
		}
	}
}
