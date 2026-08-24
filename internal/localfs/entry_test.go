package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListDirHidesDotfiles(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "z-dir"))
	mustMkdir(t, filepath.Join(dir, "a-dir"))
	mustWriteFile(t, filepath.Join(dir, "b.txt"))
	mustWriteFile(t, filepath.Join(dir, "a.txt"))
	mustWriteFile(t, filepath.Join(dir, ".hidden"))

	listing, err := ListDir(dir, DefaultListOptions())
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}

	names := entryNames(listing.Entries)
	if len(names) != 4 {
		t.Fatalf("entry count = %d, want 4 (no hidden): %v", len(names), names)
	}
	for _, name := range names {
		if strings.HasPrefix(name, ".") {
			t.Fatalf("found hidden file %q in listing", name)
		}
	}
}

func TestListDirCanShowHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".hidden"))

	opts := DefaultListOptions()
	opts.ShowHidden = true
	listing, err := ListDir(dir, opts)
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}

	if len(listing.Entries) != 1 || listing.Entries[0].Name != ".hidden" {
		t.Fatalf("entries = %v, want .hidden", entryNames(listing.Entries))
	}
}

func TestListDirReturnsResolvedPath(t *testing.T) {
	dir := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}

	listing, err := ListDir(".", DefaultListOptions())
	if err != nil {
		t.Fatalf("ListDir() error = %v", err)
	}

	if listing.Path != dir {
		t.Fatalf("Path = %q, want %q", listing.Path, dir)
	}
}

func TestListDirRejectsNonDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	mustWriteFile(t, filePath)

	_, err := ListDir(filePath, DefaultListOptions())
	if err == nil {
		t.Fatal("ListDir() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("ListDir() error = %v, want not a directory", err)
	}
}

func TestListDirMissingDirectoryErrorNoDuplicatePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	_, err := ListDir(missing, DefaultListOptions())
	if err == nil {
		t.Fatal("ListDir() error = nil, want error")
	}
	want := fmt.Sprintf(`stat directory %q: no such file or directory`, missing)
	if err.Error() != want {
		t.Fatalf("ListDir() error = %q, want %q", err.Error(), want)
	}
}

func TestEntryFromDirEntrySkipsVanishedName(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "harbor.txt")
	gone := filepath.Join(dir, "willow.txt")
	mustWriteFile(t, keep)
	mustWriteFile(t, gone)

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	var names []string
	for _, de := range dirEntries {
		entry, keepEntry, err := entryFromDirEntry(dir, de)
		if err != nil {
			t.Fatalf("entryFromDirEntry(%q): %v", de.Name(), err)
		}
		if keepEntry {
			names = append(names, entry.Name)
		}
	}
	if len(names) != 1 || names[0] != "harbor.txt" {
		t.Fatalf("names = %v, want [harbor.txt]", names)
	}
}

func entryNames(entries []Entry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func TestEntryFromPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	mustWriteFile(t, p)
	e, err := EntryFromPath(p)
	if err != nil {
		t.Fatalf("EntryFromPath: %v", err)
	}
	if e.Name != "f.txt" {
		t.Fatalf("Name = %q", e.Name)
	}
	if !filepath.IsAbs(e.Path) {
		t.Fatalf("Path = %q want abs", e.Path)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
