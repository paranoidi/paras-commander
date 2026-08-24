package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestMkdirPrefillName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		entry localfs.Entry
		want  string
	}{
		{
			name:  "file strips last extension",
			entry: localfs.Entry{Name: "cursor-name.txt", Type: localfs.EntryFile},
			want:  "cursor-name",
		},
		{
			name:  "compound extension keeps inner suffix",
			entry: localfs.Entry{Name: "archive.tar.gz", Type: localfs.EntryFile},
			want:  "archive.tar",
		},
		{
			name:  "file without extension unchanged",
			entry: localfs.Entry{Name: "README", Type: localfs.EntryFile},
			want:  "README",
		},
		{
			name:  "dotfile with no stem unchanged",
			entry: localfs.Entry{Name: ".gitignore", Type: localfs.EntryFile},
			want:  ".gitignore",
		},
		{
			name:  "directory keeps dotted name",
			entry: localfs.Entry{Name: "photos.backup", Type: localfs.EntryDirectory},
			want:  "photos.backup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mkdirPrefillName(tt.entry); got != tt.want {
				t.Fatalf("mkdirPrefillName() = %q, want %q", got, tt.want)
			}
		})
	}
}
