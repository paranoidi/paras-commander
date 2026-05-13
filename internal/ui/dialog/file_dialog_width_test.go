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
			name: "rename",
			typ:  FileDialogRename,
			base: FileDialogState{
				Fields: []FileDialogField{{Label: "Name", Value: ""}},
			},
		},
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
			wShort := fileDialogWidth(screenW, s)
			for i := range s.Fields {
				s.Fields[i].Value = longVal
			}
			wLong := fileDialogWidth(screenW, s)
			if wShort != wLong {
				t.Fatalf("width short=%d long=%d (must not depend on value length)", wShort, wLong)
			}
			if wShort != PreferredFormDialogWidth {
				t.Fatalf("width = %d, want PreferredFormDialogWidth %d", wShort, PreferredFormDialogWidth)
			}
		})
	}
}

func TestFileDialogWidthClampsToScreen(t *testing.T) {
	s := FileDialogState{
		DialogType: FileDialogRename,
		Fields:     []FileDialogField{{Label: "Name", Value: "x"}},
	}
	const screenW = 50
	got := fileDialogWidth(screenW, s)
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
	wShort := fileDialogWidth(80, withMessage(base, shortPath))
	wLong := fileDialogWidth(80, withMessage(base, longPath))
	if wShort != wLong {
		t.Fatalf("add-bookmark width depends on Message length: short=%d long=%d", wShort, wLong)
	}
}

func withMessage(s FileDialogState, msg string) FileDialogState {
	s.Message = msg
	return s
}
