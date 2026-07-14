package dialog

import "testing"

func TestRenameFocusCheckboxLabel(t *testing.T) {
	rename := FileDialogState{DialogType: FileDialogRename}
	if got := renameFocusCheckboxLabel(rename); got != "Focus after rename" {
		t.Fatalf("rename label = %q, want Focus after rename", got)
	}
	duplicate := FileDialogState{DialogType: FileDialogDuplicate}
	if got := renameFocusCheckboxLabel(duplicate); got != "Focus after duplicate" {
		t.Fatalf("duplicate label = %q, want Focus after copy", got)
	}
}
