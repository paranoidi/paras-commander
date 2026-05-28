package bookmarks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOriginPathPickerSource(t *testing.T) {
	if got := OriginFZFMarks.PathPickerSource(); got != "fzf-marks" {
		t.Fatalf("FZF = %q", got)
	}
	if got := OriginGTK.PathPickerSource(); got != "gnome" {
		t.Fatalf("GTK = %q", got)
	}
}

func TestParseLine(t *testing.T) {
	m, ok := ParseLine("proj : /home/user/proj")
	if !ok || m.Name != "proj" || m.Path != "/home/user/proj" || m.Line != "proj : /home/user/proj" || m.Origin != OriginFZFMarks {
		t.Fatalf("got %#v ok=%v", m, ok)
	}
	_, ok = ParseLine("")
	if ok {
		t.Fatal("empty should skip")
	}
	_, ok = ParseLine("   ")
	if ok {
		t.Fatal("blank should skip")
	}
	_, ok = ParseLine("# comment")
	if ok {
		t.Fatal("comment should skip")
	}
	_, ok = ParseLine("nopath")
	if ok {
		t.Fatal("no delimiter should skip")
	}
	m, ok = ParseLine("a : /with : colons : in path")
	if !ok || m.Name != "a" || m.Path != "/with : colons : in path" {
		t.Fatalf("first delimiter only: got %#v", m)
	}
	m, ok = ParseLine("x : /trail\r")
	if !ok || m.Path != "/trail" {
		t.Fatalf("CRLF: got %#v", m)
	}
	m, ok = ParseLine(" : /home/user/unnamed")
	if !ok || m.Name != "" || m.Path != "/home/user/unnamed" || m.Line != " : /home/user/unnamed" || m.Origin != OriginFZFMarks {
		t.Fatalf("empty label: got %#v ok=%v", m, ok)
	}
	m, ok = ParseLine("  spaced : /tmp/z  ")
	if !ok || m.Name != "spaced" || m.Path != "/tmp/z" {
		t.Fatalf("leading/trailing space on line: got %#v", m)
	}
}

func TestParseReaderOrder(t *testing.T) {
	r := strings.NewReader("first : /a\n# skip\nsecond : /b\n")
	marks, err := ParseReader(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != 2 || marks[0].Name != "first" || marks[1].Name != "second" {
		t.Fatalf("%+v", marks)
	}
}

func TestResolveFile(t *testing.T) {
	t.Setenv(envMarksFile, "")
	p, err := ResolveFile("/abs/custom", "/home/u")
	if err != nil || p != filepath.Clean("/abs/custom") {
		t.Fatalf("got %q %v", p, err)
	}
	t.Setenv(envMarksFile, "/env/marks")
	p, err = ResolveFile("", "/home/u")
	if err != nil || p != filepath.Clean("/env/marks") {
		t.Fatalf("env: got %q %v", p, err)
	}
	t.Setenv(envMarksFile, "")
	p, err = ResolveFile("", "/home/me")
	if err != nil || p != filepath.Join("/home/me", ".fzf-marks") {
		t.Fatalf("default: got %q %v", p, err)
	}
	p, err = ResolveFile("~/marks.txt", "/home/me")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/home/me", "marks.txt"); p != want {
		t.Fatalf("tilde: got %q want %q", p, want)
	}
}

func TestAppendAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "marks")
	if err := Append(path, Mark{Name: "a", Path: "/x", Line: "a : /x"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "a : /x\n" {
		t.Fatalf("got %q", string(b))
	}
	if err := Append(path, Mark{Name: "b", Path: "/y"}); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "a : /x\nb : /y\n"
	if string(b) != want {
		t.Fatalf("got %q want %q", string(b), want)
	}
}
