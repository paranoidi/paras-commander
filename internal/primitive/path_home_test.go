package primitive

import "testing"

func TestPathWithHomeTilde(t *testing.T) {
	home := "/home/paranoidi"
	tests := []struct {
		path string
		want string
	}{
		{"/home/paranoidi", "~/"},
		{"/home/paranoidi/", "~/"},
		{"/home/paranoidi/projects/foo", "~/projects/foo"},
		{"/other/foo", "/other/foo"},
	}
	for _, tc := range tests {
		got := PathWithHomeTilde(tc.path, home)
		if got != tc.want {
			t.Fatalf("PathWithHomeTilde(%q, home=%q) = %q, want %q", tc.path, home, got, tc.want)
		}
	}
}

func TestPathWithHomeTildeEmptyHome(t *testing.T) {
	path := "/home/paranoidi/foo"
	got := PathWithHomeTilde(path, "")
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}
