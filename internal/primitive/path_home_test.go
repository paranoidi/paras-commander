package primitive

import "testing"

func TestPathWithHomeTilde(t *testing.T) {
	home := "/home/testuser"
	tests := []struct {
		path string
		want string
	}{
		{"/home/testuser", "~/"},
		{"/home/testuser/", "~/"},
		{"/home/testuser/volumes/foo", "~/volumes/foo"},
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
	path := "/home/testuser/foo"
	got := PathWithHomeTilde(path, "")
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}
