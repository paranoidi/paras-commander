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

func TestRunForEachPreviewTextShownWhenValid(t *testing.T) {
	state := FileDialogState{
		DialogType:        FileDialogRunForEach,
		RunForEachPreview: `echo "/work/proj/beacon.txt"`,
	}
	if got := runForEachPreviewText(state); got != `echo "/work/proj/beacon.txt"` {
		t.Fatalf("preview = %q", got)
	}
	if !runForEachShowsPreview(state) {
		t.Fatal("expected runForEachShowsPreview to be true")
	}
	if got := runForEachCommandFieldRows(state); got != 5 {
		t.Fatalf("rows = %d, want 5", got)
	}
}

func TestRunForEachPreviewTextHiddenWhenErrorPresent(t *testing.T) {
	state := FileDialogState{
		DialogType:             FileDialogRunForEach,
		RunForEachPreview:      `echo "/work/proj/beacon.txt"`,
		RunForEachCommandError: "Command must include %f to represent the selected item",
	}
	if got := runForEachPreviewText(state); got != "" {
		t.Fatalf("preview = %q, want empty when error is shown", got)
	}
	// Error + preview both present must still add only one row (mutually exclusive display).
	if got := runForEachCommandFieldRows(state); got != 5 {
		t.Fatalf("rows = %d, want 5", got)
	}
}
