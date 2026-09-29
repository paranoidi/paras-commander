package dialog

import "testing"

func TestRenameOpenOtherCheckboxShiftsButtons(t *testing.T) {
	t.Parallel()
	st := FileDialogState{
		Open: true, DialogType: FileDialogRename, RenamePhase: RenamePhaseMain,
		Fields: []FileDialogField{{Label: "Name", Value: "alpha"}},
	}
	okFile := FileDialogOKFocusIndex(st)
	st.RenameSourceIsDir = true
	if got := FileDialogOKFocusIndex(st); got != okFile+1 {
		t.Fatalf("OK focus = %d, want %d", got, okFile+1)
	}
	st.DialogType = FileDialogDuplicate
	if renameHasOpenOtherCheckbox(st) {
		t.Fatal("duplicate must not offer open-in-other checkbox")
	}
}
