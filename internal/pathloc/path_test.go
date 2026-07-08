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

func TestSFTPRemotePathHome(t *testing.T) {
	t.Parallel()
	p := MustParse("sftp://user@host/~")
	remote, err := SFTPRemotePath(p)
	if err != nil {
		t.Fatal(err)
	}
	if remote != "~" {
		t.Fatalf("SFTPRemotePath = %q, want ~", remote)
	}
}

func TestSFTPJoinFromHome(t *testing.T) {
	t.Parallel()
	home := MustParse("sftp://user@host/~")
	child, err := home.Join("apps")
	if err != nil {
		t.Fatal(err)
	}
	want := "sftp://user@host/~/apps"
	if child.String() != want {
		t.Fatalf("Join from home: got %q want %q", child.String(), want)
	}
	remote, err := SFTPRemotePath(child)
	if err != nil {
		t.Fatal(err)
	}
	if remote != "~/apps" {
		t.Fatalf("SFTPRemotePath = %q, want ~/apps", remote)
	}
	nested, err := child.Join("bin")
	if err != nil {
		t.Fatal(err)
	}
	if nested.String() != "sftp://user@host/~/apps/bin" {
		t.Fatalf("nested join: got %q", nested.String())
	}
}

func TestSFTPParentFromHomeSubdir(t *testing.T) {
	t.Parallel()
	sub := MustParse("sftp://user@host/~/apps")
	parent := sub.Parent()
	if parent.String() != "sftp://user@host/~" {
		t.Fatalf("parent = %q, want sftp://user@host/~", parent.String())
	}
}

func TestSFTPTildeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		"sftp://user@host/~",
		"sftp://user@host/~/apps",
		"sftp://user@host/~/apps/bin",
		"sftp://user@host/var",
	} {
		p := MustParse(in)
		if p.String() != in {
			t.Fatalf("parse round-trip %q got %q", in, p.String())
		}
		again := MustParse(p.String())
		if !again.Equal(p) {
			t.Fatalf("re-parse %q: got %q", in, again.String())
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

func TestCommonAncestor(t *testing.T) {
	cases := []struct {
		a, b string
		want string
		ok   bool
	}{
		{"/tmp/a/x", "/tmp/a/y", "/tmp/a", true},
		{"/tmp/a", "/tmp/a/y/z", "/tmp/a", true},
		{"/tmp/a", "/tmp/a", "/tmp/a", true},
		{"/tmp/a/x", "/var/b", "/", true},
		{"sftp://host/a/x", "sftp://host/a/y", "sftp://host/a", true},
	}
	for _, c := range cases {
		got, ok := CommonAncestor(MustParse(c.a), MustParse(c.b))
		if ok != c.ok || got.String() != c.want {
			t.Fatalf("CommonAncestor(%q, %q) = %q, %v; want %q, %v", c.a, c.b, got.String(), ok, c.want, c.ok)
		}
	}
	if _, ok := CommonAncestor(MustParse("/tmp/a"), MustParse("sftp://host/a")); ok {
		t.Fatal("mixed schemes must have no common ancestor")
	}
	if _, ok := CommonAncestor(MustParse("sftp://host1/a"), MustParse("sftp://host2/a")); ok {
		t.Fatal("different hosts must have no common ancestor")
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
