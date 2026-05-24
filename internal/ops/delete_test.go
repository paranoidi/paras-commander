package ops

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestDeleteConfirmContentSingleFile(t *testing.T) {
	source := Source{Entries: []localfs.Entry{{Name: "keep.txt", Type: localfs.EntryFile}}}
	summary, warning := DeleteConfirmContent(source)
	if summary != "Delete file" {
		t.Fatalf("summary = %q, want %q", summary, "Delete file")
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

func TestDeleteConfirmContentSingleDirectory(t *testing.T) {
	source := Source{Entries: []localfs.Entry{{Name: "proj", Type: localfs.EntryDirectory}}}
	summary, warning := DeleteConfirmContent(source)
	if summary != "Delete directory" {
		t.Fatalf("summary = %q, want %q", summary, "Delete directory")
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

func TestDeleteConfirmContentMultipleSelections(t *testing.T) {
	source := Source{Entries: []localfs.Entry{
		{Name: "a.txt", Type: localfs.EntryFile},
		{Name: "b.txt", Type: localfs.EntryFile},
	}}
	summary, warning := DeleteConfirmContent(source)
	if summary != "Delete 2 selections?" {
		t.Fatalf("summary = %q, want %q", summary, "Delete 2 selections?")
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

func TestDeleteConfirmContentMultipleWithDirectoriesWarns(t *testing.T) {
	source := Source{Entries: []localfs.Entry{
		{Name: "a", Type: localfs.EntryDirectory},
		{Name: "b", Type: localfs.EntryDirectory},
	}}
	summary, warning := DeleteConfirmContent(source)
	if summary != "Delete 2 selections?" {
		t.Fatalf("summary = %q, want %q", summary, "Delete 2 selections?")
	}
	want := "Warning: 2 directories will be removed recursively!"
	if warning != want {
		t.Fatalf("warning = %q, want %q", warning, want)
	}
}
