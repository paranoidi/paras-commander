package usermenu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMenuStubCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "menu.toml")

	created, err := WriteMenuStub(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("want created true on first write")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != MenuStubTOML {
		t.Fatalf("stub content mismatch:\n%s", string(b))
	}
	mf, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 0 {
		t.Fatalf("stub should decode to zero entries, got %d", len(mf.Entries))
	}
}

func TestWriteMenuStubIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "menu.toml")

	if _, err := WriteMenuStub(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	created, err := WriteMenuStub(path)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("want created false when file already exists")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "custom\n" {
		t.Fatalf("existing file overwritten: %q", string(b))
	}
}

func TestWriteMenuStubEmptyPath(t *testing.T) {
	if _, err := WriteMenuStub(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}
