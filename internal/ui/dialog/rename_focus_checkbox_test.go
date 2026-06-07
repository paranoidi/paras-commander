package dialog

import "testing"

func TestRenameFocusCheckboxLabel(t *testing.T) {
	rename := FileDialogState{DialogType: FileDialogRename}
	if got := renameFocusCheckboxLabel(rename); got != "Focus after rename" {
		t.Fatalf("rename label = %q, want Focus after rename", got)
	}
	copyHere := FileDialogState{DialogType: FileDialogCopyHere}
	if got := renameFocusCheckboxLabel(copyHere); got != "Focus after copy" {
		t.Fatalf("copy-here label = %q, want Focus after copy", got)
	}
}
