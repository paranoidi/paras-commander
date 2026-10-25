package dialog

import "testing"

func TestRenameOpenOtherCheckboxOnlyForDirectoryRename(t *testing.T) {
	t.Parallel()
	st := FileDialogState{
		Open: true, DialogType: FileDialogRename, RenamePhase: RenamePhaseMain,
		Fields: []FileDialogField{{Label: "Name", Value: "alpha"}},
	}
	if renameHasOpenOtherCheckbox(st) {
		t.Fatal("a file rename must not offer the open-in-other checkbox")
	}
	st.RenameSourceIsDir = true
	if !renameHasOpenOtherCheckbox(st) {
		t.Fatal("a directory rename offers the open-in-other checkbox")
	}
	st.DialogType = FileDialogDuplicate
	if renameHasOpenOtherCheckbox(st) {
		t.Fatal("duplicate must not offer open-in-other checkbox")
	}
}
