package ops

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestDeleteConfirmMessageSingleFile(t *testing.T) {
	source := Source{Entries: []localfs.Entry{{Name: "keep.txt", Type: localfs.EntryFile}}}
	got := DeleteConfirmMessage(source)
	want := "Delete file\nkeep.txt"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeleteConfirmMessageSingleDirectory(t *testing.T) {
	source := Source{Entries: []localfs.Entry{{Name: "proj", Type: localfs.EntryDirectory}}}
	got := DeleteConfirmMessage(source)
	want := "Delete directory\nproj"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeleteConfirmMessageMultipleSelections(t *testing.T) {
	source := Source{Entries: []localfs.Entry{
		{Name: "a.txt", Type: localfs.EntryFile},
		{Name: "b.txt", Type: localfs.EntryFile},
	}}
	got := DeleteConfirmMessage(source)
	want := "Delete 2 selections?"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDeleteConfirmMessageMultipleWithDirectoriesWarns(t *testing.T) {
	source := Source{Entries: []localfs.Entry{
		{Name: "a", Type: localfs.EntryDirectory},
		{Name: "b", Type: localfs.EntryDirectory},
	}}
	got := DeleteConfirmMessage(source)
	want := "Delete 2 selections?\nWarning: 2 directories will be removed recursively!"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
