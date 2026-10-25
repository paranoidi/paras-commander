package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

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

// The row under the name input holds the whitespace warning once focus has left the name.
func TestRenameWhitespaceWarning(t *testing.T) {
	cases := []struct {
		v     string
		focus dlg.ID
		want  string
	}{
		{" lantern.txt", FileRenameFocusAfter, "Name has leading whitespace"},
		{"lantern.txt ", FileRenameFocusAfter, "Name has trailing whitespace"},
		{" lantern.txt ", dlg.OK, "Name has leading and trailing whitespace"},
		{" lantern.txt ", FileFieldID(0), ""},
		{" lantern.txt ", "", ""},
		{"lantern.txt", FileRenameFocusAfter, ""},
	}
	for _, c := range cases {
		st := FileDialogState{
			DialogType: FileDialogRename, Fields: []FileDialogField{{Label: "Name", Value: c.v}}, Focus: c.focus,
		}
		items := FileDialogView(st, FileDialogViewContext{Layout: Layout{Width: 80, Height: 24}}).Groups[0].Columns[0]
		if got := items[2].Label; got != c.want {
			t.Errorf("%q focus=%q: warning row %q, want %q", c.v, c.focus, got, c.want)
		}
	}
}
