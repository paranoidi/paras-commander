package panel

import (
	"reflect"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestParentDirsOfDedupsSiblings(t *testing.T) {
	t.Parallel()
	got := ParentDirsOf([]string{
		"/tmp/alpha/bravo.txt",
		"/tmp/alpha/charlie.txt",
		"/tmp/delta/echo.txt",
	})
	want := []string{"/tmp/alpha", "/tmp/delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParentDirsOfSkipsRoot(t *testing.T) {
	t.Parallel()
	got := ParentDirsOf([]string{"/", "/tmp/alpha/bravo.txt"})
	want := []string{"/tmp/alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParentDirsOfSFTP(t *testing.T) {
	t.Parallel()
	got := ParentDirsOf([]string{
		"sftp://user@host/foxtrot/golf.txt",
		"sftp://user@host/foxtrot/hotel.txt",
	})
	want := []string{"sftp://user@host/foxtrot"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSelectedParentDirs(t *testing.T) {
	t.Parallel()
	s := &State{SelectedPaths: map[string]bool{
		"/tmp/alpha/bravo.txt": true,
		"/tmp/alpha/charlie":   true,
		"/tmp/delta/echo":      true,
	}}
	got := s.SelectedParentDirs()
	want := []string{"/tmp/alpha", "/tmp/delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestSelectedParentDirsExcludesOwnPath covers the case reported as a UI hang: every
// selected path is a direct child of the panel's own current directory, so their
// unique parent is the directory the panel is already browsing. Selecting that
// directory from within itself doesn't correspond to any listed entry, so it must
// be excluded rather than handed to BulkAddSelections.
func TestSelectedParentDirsExcludesOwnPath(t *testing.T) {
	t.Parallel()
	cur, err := pathloc.File("/tmp/alpha")
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}
	selected := map[string]bool{}
	for _, name := range []string{"bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima"} {
		selected["/tmp/alpha/"+name] = true
	}
	s := &State{Path: cur, SelectedPaths: selected}
	if got := s.SelectedParentDirs(); len(got) != 0 {
		t.Fatalf("got %v, want empty (all parents equal the panel's own path)", got)
	}
}

// TestParentDirsExcludingSelfKeepsOtherDirs covers a selection spanning multiple
// parents where only one of them happens to equal the panel's own current
// directory: that one is dropped, the rest are kept.
func TestParentDirsExcludingSelfKeepsOtherDirs(t *testing.T) {
	t.Parallel()
	cur, err := pathloc.File("/tmp/alpha")
	if err != nil {
		t.Fatalf("pathloc.File: %v", err)
	}
	s := &State{Path: cur}
	got := s.ParentDirsExcludingSelf([]string{
		"/tmp/alpha/bravo",
		"/tmp/delta/echo",
	})
	want := []string{"/tmp/delta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
