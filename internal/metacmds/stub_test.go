package metacmds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteMetaStubCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.toml")

	created, err := WriteMetaStub(path)
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
	if string(b) != MetaStubTOML {
		t.Fatalf("stub content mismatch:\n%s", string(b))
	}
}

func TestRefreshDocumentationPrependsDoc(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.toml")
	entries := `[[entry]]
name = "tally"
description = "Count lines"
file = "wc -l"
`
	if err := os.WriteFile(path, []byte(entries), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshDocumentation(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("want changed=true")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), entries) {
		t.Fatalf("entries not preserved:\n%s", b)
	}
	if !strings.Contains(string(b), "# meta.toml") {
		t.Fatalf("canonical doc missing:\n%s", b)
	}
}

func TestRefreshDocumentationIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meta.toml")
	if _, err := WriteMetaStub(path); err != nil {
		t.Fatal(err)
	}
	changed, err := RefreshDocumentation(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("want changed=false for current stub")
	}
}
