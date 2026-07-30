package scan

import "testing"

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

func TestStripHiddenEntriesByName(t *testing.T) {
	t.Parallel()
	root := "/home/user"
	entries := []Entry{
		{Path: "/home/user/visible.txt", RelLine: "visible.txt"},
		{Path: "/home/user/.gitignore", RelLine: ".gitignore"},
		{Path: "/home/user/.git/config", RelLine: ".git/config"},
	}
	got := stripHiddenEntriesByName(entries, root)
	if len(got) != 1 || got[0].RelLine != "visible.txt" {
		t.Fatalf("stripHiddenEntriesByName = %+v, want only visible.txt", got)
	}
}
