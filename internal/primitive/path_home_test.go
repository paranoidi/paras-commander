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
	SetPathDisplayPrefixes(map[string]string{"fileindex://": " "})
	if got := PathWithHomeTilde("fileindex:///home/wren/orchard", "/home/wren"); got != " ~/orchard" {
		t.Fatalf("registered = %q", got)
	}
	if got := PathWithHomeTilde("fileindex:///", "/home/wren"); got != " /" {
		t.Fatalf("index root = %q", got)
	}
}

func TestPathWithHomeTildeTagViews(t *testing.T) {
	t.Cleanup(func() { SetPathDisplayPrefixes(nil) })
	const raw = "fileindex(clover&thistle):///home/wren/orchard"
	SetPathDisplayPrefixes(nil)
	if got := PathWithHomeTilde(raw, "/home/wren"); got != raw {
		t.Fatalf("unregistered tag view = %q, want it unchanged", got)
	}
	SetPathDisplayPrefixes(map[string]string{"fileindex://": "# "})
	for in, want := range map[string]string{
		raw:                                        "#(clover&thistle) ~/orchard",
		"fileindex@(clover|thistle):///":           "#@(clover|thistle) /",
		"fileindex@(clover):///home/wren/a//b/c":   "#@(clover) ~/a/b/c",
		"fileindex(a%20b):///home/wren":            "#(a%20b) ~/",
		"harbor(x)://srv":                          "harbor(x)://srv",
		"fileindex(~tilde):///home/wren/orchard":   "#(~tilde) ~/orchard",
		"/home/wren/fileindex(clover):///orchard":  "/home/wren/fileindex(clover):///orchard",
		"fileindex(clover):///home/wren/ore/more/": "#(clover) ~/ore/more",
	} {
		if got := PathWithHomeTilde(in, "/home/wren"); got != want {
			t.Errorf("PathWithHomeTilde(%q) = %q, want %q", in, got, want)
		}
	}
}
