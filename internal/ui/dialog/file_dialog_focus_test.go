package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFileDialogFocusFormDeleteDialog(t *testing.T) {
	st := FileDialogState{DialogType: FileDialogDelete, FocusedField: 0}
	form := FileDialogFocusForm(st)
	if form.TotalFocus() != 2 {
		t.Fatalf("TotalFocus() = %d want 2", form.TotalFocus())
	}
	if form.OKIndex() != 0 || form.CancelIndex() != 1 {
		t.Fatalf("OK/Cancel indices = %d/%d want 0/1", form.OKIndex(), form.CancelIndex())
	}
	if nf, ok := form.MoveFocus(0, tcell.KeyRight); !ok || nf != 1 {
		t.Fatalf("Right from Yes: focus=%d ok=%v want 1,true", nf, ok)
	}
}

func TestFileDialogFocusFormMkdirWithRadios(t *testing.T) {
	st := FileDialogState{
		DialogType:       FileDialogMkdir,
		MkdirShowActions: true,
		Fields:           []FileDialogField{{}},
	}
	form := FileDialogFocusForm(st)
	wantContent := 1 + 3 // one field + three radio rows
	if form.NumContent != wantContent {
		t.Fatalf("NumContent = %d want %d", form.NumContent, wantContent)
	}
	if form.OKIndex() != wantContent {
		t.Fatalf("OKIndex = %d want %d", form.OKIndex(), wantContent)
	}
}

func TestFileDialogFocusFormRenameWithFocusCheckbox(t *testing.T) {
	st := FileDialogState{
		DialogType:  FileDialogRename,
		RenamePhase: RenamePhaseMain,
		Fields:      []FileDialogField{{}},
	}
	form := FileDialogFocusForm(st)
	wantContent := 1 + 1 // one field + focus checkbox
	if form.NumContent != wantContent {
		t.Fatalf("NumContent = %d want %d", form.NumContent, wantContent)
	}
	if form.TotalFocus() != wantContent+2 {
		t.Fatalf("TotalFocus() = %d want %d", form.TotalFocus(), wantContent+2)
	}
	if form.OKIndex() != wantContent {
		t.Fatalf("OKIndex = %d want %d", form.OKIndex(), wantContent)
	}
}

func TestFileDialogFocusFormDuplicateWithFocusCheckbox(t *testing.T) {
	st := FileDialogState{
		DialogType:  FileDialogDuplicate,
		RenamePhase: RenamePhaseMain,
		Fields:      []FileDialogField{{}},
	}
	form := FileDialogFocusForm(st)
	wantContent := 1 + 1
	if form.NumContent != wantContent {
		t.Fatalf("NumContent = %d want %d", form.NumContent, wantContent)
	}
	if form.OKIndex() != wantContent {
		t.Fatalf("OKIndex = %d want %d", form.OKIndex(), wantContent)
	}
}

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
