package scan

import (
	"path/filepath"
	"testing"
)

func TestEntryPathHidden(t *testing.T) {
	t.Parallel()
	root := "/home/user"
	cases := []struct {
		path string
		want bool
	}{
		{"/home/user/visible.txt", false},
		{"/home/user/.hidden", true},
		{"/home/user/.cache/foo", true},
		{"/home/user/project/.git/config", true},
		{"/home/user/readme", false},
	}
	for _, tc := range cases {
		if got := entryPathHidden(root, tc.path); got != tc.want {
			t.Fatalf("entryPathHidden(%q) = %v want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterEntriesToScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	entries := []Entry{
		{RelLine: "a/in-a.txt"},
		{RelLine: "b/in-b.txt"},
	}
	got := filterEntriesToScope(entries, root, []string{dirA})
	if len(got) != 1 || got[0].RelLine != "a/in-a.txt" {
		t.Fatalf("filterEntriesToScope = %+v", got)
	}
}

func TestStripHiddenEntriesByName(t *testing.T) {
	t.Parallel()
	root := "/home/user"
	entries := []Entry{
		{RelLine: "visible.txt"},
		{RelLine: ".gitignore"},
		{RelLine: ".git/config"},
	}
	got := stripHiddenEntriesByName(entries, root)
	if len(got) != 1 || got[0].RelLine != "visible.txt" {
		t.Fatalf("stripHiddenEntriesByName = %+v, want only visible.txt", got)
	}
}
