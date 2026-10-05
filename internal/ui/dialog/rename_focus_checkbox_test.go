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

func TestRenameWhitespaceWarning(t *testing.T) {
	st := func(v string, focus int) FileDialogState {
		return FileDialogState{DialogType: FileDialogRename, Fields: []FileDialogField{{Value: v}}, FocusedField: focus}
	}
	cases := []struct {
		v     string
		focus int
		want  string
	}{
		{" lantern.txt", 1, "Name has leading whitespace"},
		{"lantern.txt ", 1, "Name has trailing whitespace"},
		{" lantern.txt ", 2, "Name has leading and trailing whitespace"},
		{" lantern.txt ", 0, ""},
		{"lantern.txt", 1, ""},
	}
	for _, c := range cases {
		if got := renameWhitespaceWarning(st(c.v, c.focus), 0); got != c.want {
			t.Errorf("%q focus=%d: got %q, want %q", c.v, c.focus, got, c.want)
		}
	}
}
