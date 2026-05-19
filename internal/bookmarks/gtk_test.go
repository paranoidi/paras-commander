package bookmarks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGTKLine(t *testing.T) {
	m, ok := ParseGTKLine("file:///home/user/Documents")
	if !ok || m.Path != "/home/user/Documents" || m.Name != "Documents" {
		t.Fatalf("uri only: got %#v ok=%v", m, ok)
	}
	m, ok = ParseGTKLine("file:///home/user/Downloads My Downloads")
	if !ok || m.Name != "My Downloads" || m.Path != "/home/user/Downloads" {
		t.Fatalf("labeled: got %#v ok=%v", m, ok)
	}
	m, ok = ParseGTKLine("file:///home/user/My%20Space Dir Label")
	if !ok || m.Path != "/home/user/My Space" || m.Name != "Dir Label" {
		t.Fatalf("encoded space: got %#v ok=%v", m, ok)
	}
	_, ok = ParseGTKLine("")
	if ok {
		t.Fatal("empty should skip")
	}
	_, ok = ParseGTKLine("ftp://example.com/foo")
	if ok {
		t.Fatal("non-file scheme should skip")
	}
	m, ok = ParseGTKLine("file:///\r")
	if !ok || m.Name != "root" || m.Path != string(filepath.Separator) {
		t.Fatalf("root: got %#v ok=%v", m, ok)
	}
}

func TestResolveGTKFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	p, err := ResolveGTKFile("/home/me")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/home/me", ".config", "gtk-3.0", "bookmarks")
	if p != want {
		t.Fatalf("got %q want %q", p, want)
	}
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	p, err = ResolveGTKFile("/home/me")
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Join("/custom/config", "gtk-3.0", "bookmarks")
	if p != want {
		t.Fatalf("XDG: got %q want %q", p, want)
	}
}

func TestLoadAllMergesAndDedupes(t *testing.T) {
	dir := t.TempDir()
	fzfPath := filepath.Join(dir, ".fzf-marks")
	gtkDir := filepath.Join(dir, ".config", "gtk-3.0")
	gtkPath := filepath.Join(gtkDir, "bookmarks")
	if err := os.MkdirAll(gtkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fzfPath, []byte("mine : /shared\nfirst : /a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gtkBody := strings.Join([]string{
		"file:///shared Shared GTK",
		"file:///b",
	}, "\n")
	if err := os.WriteFile(gtkPath, []byte(gtkBody), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv(envMarksFile, "")

	marks, err := LoadAll(fzfPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(marks), marks)
	}
	if marks[0].Name != "mine" || marks[1].Name != "first" {
		t.Fatalf("fzf order: %+v", marks)
	}
	if marks[2].Path != "/b" {
		t.Fatalf("gtk append: %+v", marks[2])
	}
}
