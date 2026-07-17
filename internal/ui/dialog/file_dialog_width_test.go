package dialog

import (
	"strings"
	"testing"
)

func TestFileDialogWidthIgnoresFieldValueLength(t *testing.T) {
	const screenW = 80
	shortVal := strings.Repeat("a", 3)
	longVal := strings.Repeat("b", 500)

	cases := []struct {
		name string
		typ  FileDialogType
		base FileDialogState
	}{
		{
			name: "mkdir",
			typ:  FileDialogMkdir,
			base: FileDialogState{
				Fields: []FileDialogField{{Label: "Directory name", Value: ""}},
			},
		},
		{
			name: "symlink",
			typ:  FileDialogSymlink,
			base: FileDialogState{
				Fields: []FileDialogField{
					{Label: "Target", Value: ""},
					{Label: "Link path", Value: ""},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.base
			s.DialogType = tc.typ
			s.Fields[0].Value = shortVal
			for i := range s.Fields {
				s.Fields[i].Value = shortVal
			}
			wShort := fileDialogWidth(screenW, s, 0)
			for i := range s.Fields {
				s.Fields[i].Value = longVal
			}
			wLong := fileDialogWidth(screenW, s, 0)
			if wShort != wLong {
				t.Fatalf("width short=%d long=%d (must not depend on value length)", wShort, wLong)
			}
			if wShort != PreferredFormDialogWidth {
				t.Fatalf("width = %d, want PreferredFormDialogWidth %d", wShort, PreferredFormDialogWidth)
			}
		})
	}
}

func TestFileDialogRenameWidthWidensForLongName(t *testing.T) {
	const screenW = 120
	s := FileDialogState{
		DialogType: FileDialogRename,
		Fields:     []FileDialogField{{Label: "Name", Value: strings.Repeat("a", 10)}},
	}
	if got := fileDialogWidth(screenW, s, 0); got != PreferredFormDialogWidth {
		t.Fatalf("short name width = %d, want %d", got, PreferredFormDialogWidth)
	}
	s.Fields[0].Value = strings.Repeat("b", PreferredFormDialogWidth)
	if got := fileDialogWidth(screenW, s, 0); got != WideDialogWidth(screenW) {
		t.Fatalf("long name width = %d, want %d (80%% of terminal)", got, WideDialogWidth(screenW))
	}
	s.DialogType = FileDialogDuplicate
	if got := fileDialogWidth(screenW, s, 0); got != WideDialogWidth(screenW) {
		t.Fatalf("duplicate long name width = %d, want %d", got, WideDialogWidth(screenW))
	}
}

func TestFileDialogWidthClampsToScreen(t *testing.T) {
	s := FileDialogState{
		DialogType: FileDialogRename,
		Fields:     []FileDialogField{{Label: "Name", Value: "x"}},
	}
	const screenW = 50
	got := fileDialogWidth(screenW, s, 0)
	want := screenW - 4
	if got != want {
		t.Fatalf("fileDialogWidth(%d, rename) = %d, want %d", screenW, got, want)
	}
}

func TestFileDialogAddBookmarkWidthIgnoresMessagePathLength(t *testing.T) {
	shortPath := "/tmp"
	longPath := strings.Repeat("/x", 200)
	base := FileDialogState{
		DialogType: FileDialogAddBookmark,
		Fields:     []FileDialogField{{Label: "Name", Value: "n"}},
		Message:    "",
	}
	wShort := fileDialogWidth(80, withMessage(base, shortPath), 0)
	wLong := fileDialogWidth(80, withMessage(base, longPath), 0)
	if wShort != wLong {
		t.Fatalf("add-bookmark width depends on Message length: short=%d long=%d", wShort, wLong)
	}
}

func withMessage(s FileDialogState, msg string) FileDialogState {
	s.Message = msg
	return s
}

func TestFileDialogDeleteWidthReservesIconStrip(t *testing.T) {
	const name = "45.Years.2015.LIMITED.1080p.BluRay.X264-XXX"
	const iconLead = 2
	state := FileDialogState{
		DialogType:    FileDialogDelete,
		DeleteSummary: "Delete file",
		DeleteEntries: []DeleteListEntry{{Name: name}},
	}
	state.DeleteLayoutMinWidth = ComputeDeleteDialogLayoutMinWidth(state, iconLead)
	const screenW = 120
	got := fileDialogWidth(screenW, state, iconLead)
	want := len([]rune(name)) + 4 + iconLead
	if got != want {
		t.Fatalf("width = %d, want %d (name + padding + icon strip)", got, want)
	}
}

func TestFileDialogDeleteWidthNoIconStripWhenLeadZero(t *testing.T) {
	const name = "short.txt"
	state := FileDialogState{
		DialogType:    FileDialogDelete,
		DeleteSummary: "Delete file",
		DeleteEntries: []DeleteListEntry{{Name: name}},
	}
	state.DeleteLayoutMinWidth = ComputeDeleteDialogLayoutMinWidth(state, 0)
	got := fileDialogWidth(80, state, 0)
	want := len([]rune(name)) + 4
	if got < want {
		t.Fatalf("width = %d, want at least %d", got, want)
	}
}

func TestFileDialogDeleteWidthUsesCachedLayoutMinWidth(t *testing.T) {
	state := FileDialogState{
		DialogType:           FileDialogDelete,
		DeleteSummary:        "summary",
		DeleteLayoutMinWidth: 64,
		DeleteEntries:        make([]DeleteListEntry, 500),
	}
	got := fileDialogWidth(120, state, 0)
	if got != 64 {
		t.Fatalf("width = %d, want cached 64 without scanning entries", got)
	}
}
