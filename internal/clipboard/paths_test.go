package clipboard

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestBuildFileURLs(t *testing.T) {
	got := BuildFileURLs([]string{"/tmp/meadow/report.txt", "/tmp/harbor/note.md"})
	want := "/tmp/meadow/report.txt\n/tmp/harbor/note.md"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildDirURLs(t *testing.T) {
	t.Run("parent per file", func(t *testing.T) {
		got := BuildDirURLs([]string{"/tmp/meadow/report.txt", "/tmp/harbor/note.md"}, "/tmp/panel")
		want := "/tmp/meadow\n/tmp/harbor"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("fallback panel dir when empty", func(t *testing.T) {
		got := BuildDirURLs(nil, "/tmp/panel")
		if got != "/tmp/panel" {
			t.Fatalf("got %q, want panel dir", got)
		}
	})
}

func TestBuildFilenames(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "report.txt", Path: "/tmp/meadow/report.txt"},
		{Name: "..", Path: "/tmp"},
		{Name: "note.md", Path: "/tmp/harbor/note.md"},
	}
	got := BuildFilenames(entries)
	want := "report.txt\nnote.md"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildFilenamesWithoutExt(t *testing.T) {
	entries := []localfs.Entry{
		{Name: "report.txt", Path: "/tmp/meadow/report.txt"},
		{Name: "archive.tar.gz", Path: "/tmp/harbor/archive.tar.gz"},
	}
	got := BuildFilenamesWithoutExt(entries)
	want := "report\narchive.tar"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildDirURLsSFTP(t *testing.T) {
	got := BuildDirURLs([]string{"sftp://user@host/tmp/meadow/report.txt"}, "")
	want := "sftp://user@host/tmp/meadow"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
