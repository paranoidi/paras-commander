package pathpick

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestQueryLooksPathlike(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    string
		want bool
	}{
		{"", false},
		{"foo", false},
		{"/abs", true},
		{"~/x", true},
		{"a/b", true},
		{".hidden", true},
	}
	for _, tc := range cases {
		if got := QueryLooksPathlike(tc.q); got != tc.want {
			t.Fatalf("QueryLooksPathlike(%q) = %v want %v", tc.q, got, tc.want)
		}
	}
}

func TestResolveQuery(t *testing.T) {
	panel := "/panel/cwd"
	home := "/home/user"
	if got := ResolveQuery(panel, home, "~/doc"); got != filepath.Join(home, "doc") {
		t.Fatalf("tilde doc: got %q", got)
	}
	if got := ResolveQuery(panel, home, "~"); got != home {
		t.Fatalf("tilde home: got %q", got)
	}
	if got := ResolveQuery(panel, home, "/etc"); got != "/etc" {
		t.Fatalf("abs: got %q", got)
	}
	if got := ResolveQuery(panel, home, "sub"); got != filepath.Join(panel, "sub") {
		t.Fatalf("rel: got %q", got)
	}
}

func TestResolveDestination(t *testing.T) {
	local, err := pathloc.Parse("/srv/panel")
	if err != nil {
		t.Fatal(err)
	}
	remote, err := pathloc.Parse("sftp://user@host/var/www")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		panel pathloc.Path
		raw   string
		want  string
	}{
		{"abs", local, "/opt/meadow", "/opt/meadow"},
		{"tilde", local, "~", "/home/ann"},
		{"tilde-sub", local, " ~/orchard ", "/home/ann/orchard"},
		{"relative", local, "willow/../otter", "/srv/panel/otter"},
		{"sftp", local, "sftp://user@host/tmp/oak", "sftp://user@host/tmp/oak"},
		{"remote-panel-as-given", remote, "sftp://user@host/tmp/oak", "sftp://user@host/tmp/oak"},
		{"remote-panel-tilde-unresolved", remote, "/tmp/oak", "/tmp/oak"},
	}
	for _, tc := range tests {
		got, err := ResolveDestination(tc.panel, "/home/ann", tc.raw)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got.String() != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got.String(), tc.want)
		}
	}
	if _, err := ResolveDestination(local, "/home/ann", "  "); err == nil {
		t.Error("empty destination: want error")
	}
}
