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

func TestPathWithHomeTildeURIs(t *testing.T) {
	t.Cleanup(func() { SetPathDisplayPrefixes(nil) })
	SetPathDisplayPrefixes(nil)
	if got := PathWithHomeTilde("fileindex:///home/wren/orchard", "/home/wren"); got != "fileindex:///home/wren/orchard" {
		t.Fatalf("unregistered URI = %q, want it unchanged", got)
	}
	if got := PathWithHomeTilde("sftp://user@harbor/meadow", "/home/wren"); got != "sftp://user@harbor/meadow" {
		t.Fatalf("sftp = %q", got)
	}
	SetPathDisplayPrefixes(map[string]string{"fileindex://": "://"})
	if got := PathWithHomeTilde("fileindex:///home/wren/orchard", "/home/wren"); got != "://~/orchard" {
		t.Fatalf("registered = %q", got)
	}
	if got := PathWithHomeTilde("fileindex:///", "/home/wren"); got != ":///" {
		t.Fatalf("index root = %q", got)
	}
}
