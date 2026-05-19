package pathpick

import (
	"path/filepath"
	"testing"
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
