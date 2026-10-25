package dlg

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// modeSpec: a row of three radios (the second selected) and an option row, then an input.
func modeSpec(tabToSelected bool) Spec {
	return Spec{
		Title: "Harbor", Width: 50, MinWidth: 20, Unclamped: true,
		Groups: []Group{
			{Columns: [][]Item{{
				{Kind: KindRow, TabToSelected: tabToSelected, Cells: []Item{
					{ID: "simple", Kind: KindRadio, Label: "Simple", Width: 12, On: false},
					{ID: "regex", Kind: KindRadio, Label: "Regex", Width: 12, On: true},
					{ID: "editor", Kind: KindRadio, Label: "Editor", Width: 12},
				}},
				{Kind: KindRow, Cells: []Item{
					{ID: "trim", Kind: KindCheckbox, Label: "Trim", Width: 12, On: true},
				}},
			}}},
			{Columns: [][]Item{{{ID: "find", Kind: KindInput}}}},
		},
		Buttons: []Button{{ID: OK, Label: "OK", Mnemonic: 'O'}, {ID: Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

// With TabToSelected, Tab and Shift+Tab landing on the row focus its selected radio; without it
// they land on the first cell.
func TestTabToSelectedRow(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tabToSelected bool
		wantFirst     ID
	}{{"flag", true, "regex"}, {"default", false, "simple"}} {
		t.Run(tc.name, func(t *testing.T) {
			spec := modeSpec(tc.tabToSelected)
			focus := ID("find")
			HandleKey(spec, &focus, key(tcell.KeyBacktab))
			if focus != tc.wantFirst {
				t.Fatalf("Backtab from the input = %q, want %q", focus, tc.wantFirst)
			}
			focus = OK
			HandleKey(spec, &focus, key(tcell.KeyTab))
			if focus != tc.wantFirst {
				t.Fatalf("Tab from the buttons = %q, want %q", focus, tc.wantFirst)
			}
			focus = "editor"
			HandleKey(spec, &focus, key(tcell.KeyRight))
			if focus != "editor" {
				t.Fatalf("Right at the row end moved focus to %q", focus)
			}
			HandleKey(spec, &focus, key(tcell.KeyDown))
			if focus != "trim" {
				t.Fatalf("Down from the third radio = %q, want trim (nearest cell to the left)", focus)
			}
		})
	}
}
