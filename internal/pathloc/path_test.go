package pathloc

import (
	"path/filepath"
	"testing"
)

func TestParseFileAbsolute(t *testing.T) {
	dir := t.TempDir()
	p, err := Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme() != SchemeFile {
		t.Fatalf("scheme = %q", p.Scheme())
	}
	want, _ := filepath.Abs(dir)
	if p.String() != filepath.Clean(want) {
		t.Fatalf("got %q want %q", p.String(), want)
	}
}

func TestParseSFTP(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"sftp://host/", "sftp://host/"},
		{"sftp://user@host/~", "sftp://user@host/~"},
		{"sftp://user@host/var/www", "sftp://user@host/var/www"},
		{"sftp://host:2222/tmp", "sftp://host:2222/tmp"},
	}
	for _, tc := range tests {
		p, err := Parse(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if p.Scheme() != SchemeSFTP {
			t.Fatalf("%q scheme %q", tc.in, p.Scheme())
		}
		if p.String() != tc.want {
			t.Fatalf("%q got %q want %q", tc.in, p.String(), tc.want)
		}
		if !p.IsRemote() {
			t.Fatalf("%q should be remote", tc.in)
		}
	}
}

func TestHasPrefixFile(t *testing.T) {
	root := FileMust("/tmp/a")
	child := FileMust("/tmp/a/b/c.txt")
	if !child.HasPrefix(root) {
		t.Fatal("child should have root prefix")
	}
	other := FileMust("/tmp/b")
	if child.HasPrefix(other) {
		t.Fatal("child should not be under /tmp/b")
	}
}

func TestParentFile(t *testing.T) {
	p := FileMust("/tmp/a/b")
	parent := p.Parent()
	if parent.String() != "/tmp/a" {
		t.Fatalf("parent = %q", parent.String())
	}
}

func TestEqual(t *testing.T) {
	a := FileMust("/tmp")
	b := FileMust("/tmp")
	if !a.Equal(b) {
		t.Fatal("expected equal")
	}
}
