package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestFileDialogFocusFormRunForEachCheckboxes locks down the two run-for-each checkbox focus
// indices (RunForEachInDirs at len(Fields), RunForEachPTY at len(Fields)+1) and, when a pool
// selector is present, that its radios start right after both checkboxes — regression coverage
// for the baseFocus bump (+1 -> +2) that came with adding the second checkbox.
func TestFileDialogFocusFormRunForEachCheckboxes(t *testing.T) {
	st := FileDialogState{
		DialogType: FileDialogRunForEach,
		Fields:     []FileDialogField{{}},
	}
	form := FileDialogFocusForm(st)
	wantContent := 1 + 2 // one field + two checkboxes (InDirs, PTY)
	if form.NumContent != wantContent {
		t.Fatalf("NumContent = %d want %d", form.NumContent, wantContent)
	}
	if form.OKIndex() != wantContent {
		t.Fatalf("OKIndex = %d want %d", form.OKIndex(), wantContent)
	}

	stPools := FileDialogState{
		DialogType:      FileDialogRunForEach,
		Fields:          []FileDialogField{{}},
		RunForEachPools: []string{"pool-a", "pool-b"},
	}
	formPools := FileDialogFocusForm(stPools)
	// One field + two checkboxes + "No pool" + two pool radios.
	wantPoolsContent := 1 + 2 + 1 + 2
	if formPools.NumContent != wantPoolsContent {
		t.Fatalf("NumContent (pools) = %d want %d", formPools.NumContent, wantPoolsContent)
	}
}

// TestFileDialogFocusFormTabsBetweenInputs covers the multi-text-input rule: each input row is
// its own Tab group, so Tab steps between them instead of jumping straight to the buttons.
func TestFileDialogFocusFormTabsBetweenInputs(t *testing.T) {
	st := FileDialogState{
		DialogType: FileDialogExtract,
		Fields:     []FileDialogField{{Label: "Destination"}, {Label: "Archive"}},
	}
	form := FileDialogFocusForm(st)
	okIdx := form.OKIndex()
	if okIdx != 2 {
		t.Fatalf("OKIndex = %d want 2", okIdx)
	}
	for _, tc := range []struct {
		from int
		key  tcell.Key
		want int
	}{
		{0, tcell.KeyTab, 1},
		{1, tcell.KeyTab, okIdx},
		{okIdx, tcell.KeyTab, 0},
		{okIdx, tcell.KeyBacktab, 1},
		{1, tcell.KeyBacktab, 0},
		{0, tcell.KeyBacktab, okIdx},
	} {
		if nf, ok := form.MoveFocus(tc.from, tc.key); !ok || nf != tc.want {
			t.Fatalf("MoveFocus(%d, %v) = %d,%v want %d,true", tc.from, tc.key, nf, ok, tc.want)
		}
	}
}

// TestFileDialogFocusFormTabVisitsOptionGroups covers Tab/Backtab per named single-input
// dialog: each leading input is its own group, option rows (radios/checkboxes) are the next
// group, and buttons are last. Up/Down still step item-by-item.
func TestFileDialogFocusFormTabVisitsOptionGroups(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state FileDialogState
		from  int
		key   tcell.Key
		want  int
	}{
		{
			name: "run-for-each Tab from command to checkboxes",
			state: FileDialogState{
				DialogType: FileDialogRunForEach, Fields: []FileDialogField{{}},
			},
			from: 0, key: tcell.KeyTab, want: 1,
		},
		{
			name: "run-for-each Tab from checkbox to OK",
			state: FileDialogState{
				DialogType: FileDialogRunForEach, Fields: []FileDialogField{{}},
			},
			from: 2, key: tcell.KeyTab, want: 3,
		},
		{
			name: "run-for-each Backtab from OK to checkboxes",
			state: FileDialogState{
				DialogType: FileDialogRunForEach, Fields: []FileDialogField{{}},
			},
			from: 3, key: tcell.KeyBacktab, want: 1,
		},
		{
			name: "run-for-each pools Tab from command to options",
			state: FileDialogState{
				DialogType: FileDialogRunForEach, Fields: []FileDialogField{{}}, RunForEachPools: []string{"thicket", "meadow"},
			},
			from: 0, key: tcell.KeyTab, want: 1,
		},
		{
			name: "run-for-each pools Tab from pool radio to OK",
			state: FileDialogState{
				DialogType: FileDialogRunForEach, Fields: []FileDialogField{{}}, RunForEachPools: []string{"thicket", "meadow"},
			},
			from: 4, key: tcell.KeyTab, want: 6,
		},
		{
			name: "run-for-each Down still walks items",
			state: FileDialogState{
				DialogType: FileDialogRunForEach, Fields: []FileDialogField{{}},
			},
			from: 1, key: tcell.KeyDown, want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			form := FileDialogFocusForm(tt.state)
			if nf, ok := form.MoveFocus(tt.from, tt.key); !ok || nf != tt.want {
				t.Fatalf("MoveFocus(%d, %v) = %d,%v want %d,true (OK=%d)",
					tt.from, tt.key, nf, ok, tt.want, form.OKIndex())
			}
		})
	}
}
