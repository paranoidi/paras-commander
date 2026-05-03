package ui

import (
	"testing"
)

func TestPanelTitlePathHomeTilde(t *testing.T) {
	home := "/home/paranoidi"
	tests := []struct {
		path string
		want string
	}{
		{"/home/paranoidi", "~/"},
		{"/home/paranoidi/", "~/"},
		{"/home/paranoidi/projects/foo", "~/projects/foo"},
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
	path := "/home/paranoidi/projects/paras-commander"
	// No tilde — different user
	got := PanelTitlePath(path, home, len("/home/paranoidi/projects/paras-commander")-1)
	want := "/h/paranoidi/projects/paras-commander"
	if got != want {
		t.Fatalf("first abbrev: got %q, want %q", got, want)
	}
	got2 := PanelTitlePath(path, home, len(want)-1)
	want2 := "/h/p/projects/paras-commander"
	if got2 != want2 {
		t.Fatalf("second abbrev: got %q, want %q", got2, want2)
	}
}

func TestPanelTitlePathTildeThenAbbrev(t *testing.T) {
	home := "/home/paranoidi"
	path := "/home/paranoidi/projects/paras-commander"
	max := len("~/projects/paras-commander") - 1
	got := PanelTitlePath(path, home, max)
	want := "~/p/paras-commander"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPanelTitlePathEmptyHomeNoTilde(t *testing.T) {
	path := "/home/paranoidi/foo"
	got := PanelTitlePath(path, "", 200)
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}
