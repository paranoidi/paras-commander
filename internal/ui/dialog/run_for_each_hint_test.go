package dialog

import "testing"

func TestRunForEachCommandFieldRowsWithError(t *testing.T) {
	state := FileDialogState{
		DialogType:             FileDialogRunForEach,
		RunForEachCommandError: "Command must include %f to represent the selected item",
	}
	if got := runForEachCommandFieldRows(state); got != 5 {
		t.Fatalf("rows = %d, want 5", got)
	}
	if runForEachCommandFieldRows(FileDialogState{DialogType: FileDialogRunForEach}) != 4 {
		t.Fatal("expected 4 rows without error")
	}
}
