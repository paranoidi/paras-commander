package ui

import (
	"testing"
)

func TestPanelTitlePathHomeTilde(t *testing.T) {
	home := "/home/testuser"
	tests := []struct {
		path string
		want string
	}{
		{"/home/testuser", "~/"},
		{"/home/testuser/", "~/"},
		{"/home/testuser/volumes/foo", "~/volumes/foo"},
	}
	for _, tc := range tests {
		got := PanelTitlePath(tc.path, home, 200)
		if got != tc.want {
			t.Fatalf("PanelTitlePath(%q, home=%q, wide) = %q, want %q", tc.path, home, got, tc.want)
		}
	}
}

func TestPanelTitlePathProgressiveAbbrev(t *testing.T) {
	home := "/home/nobody"
	path := "/home/alice/work/synthetic-repo"
	// No tilde — different user
	got := PanelTitlePath(path, home, len("/home/alice/work/synthetic-repo")-1)
	want := "/h/alice/work/synthetic-repo"
	if got != want {
		t.Fatalf("first abbrev: got %q, want %q", got, want)
	}
	got2 := PanelTitlePath(path, home, len(want)-1)
	want2 := "/h/a/work/synthetic-repo"
	if got2 != want2 {
		t.Fatalf("second abbrev: got %q, want %q", got2, want2)
	}
}

func TestPanelTitlePathTildeThenAbbrev(t *testing.T) {
	home := "/home/testuser"
	path := "/home/testuser/volumes/synthetic-repo"
	max := len("~/volumes/synthetic-repo") - 1
	got := PanelTitlePath(path, home, max)
	want := "~/v/synthetic-repo"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPanelTitlePathEmptyHomeNoTilde(t *testing.T) {
	path := "/home/testuser/foo"
	got := PanelTitlePath(path, "", 200)
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}
