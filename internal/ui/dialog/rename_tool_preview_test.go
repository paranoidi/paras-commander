package dialog

import "testing"

func TestRenameToolPreviewTextSanitize(t *testing.T) {
	st := FileDialogState{
		DialogType:                FileDialogRename,
		RenamePhase:               RenamePhaseSanitize,
		RenameSanitizeDots:        true,
		RenameSanitizeUnderscores: true,
		Fields:                    []FileDialogField{{Value: "a.b_c"}},
	}
	if got := renameToolPreviewText(st); got != "a b c" {
		t.Fatalf("got %q want %q", got, "a b c")
	}
	st.RenameSanitizeDots = false
	if got := renameToolPreviewText(st); got != "a.b c" {
		t.Fatalf("dots off: got %q", got)
	}
}

func TestRenameToolPreviewTextSlugify(t *testing.T) {
	st := FileDialogState{
		DialogType:       FileDialogRename,
		RenamePhase:      RenamePhaseSlugify,
		RenameSlugifySep: RenameSlugifyDot,
		Fields:           []FileDialogField{{Value: "my file"}},
	}
	if got := renameToolPreviewText(st); got != "my.file" {
		t.Fatalf("got %q want my.file", got)
	}
	st.RenameSlugifySep = RenameSlugifyUnderscore
	if got := renameToolPreviewText(st); got != "my_file" {
		t.Fatalf("got %q want my_file", got)
	}
}
