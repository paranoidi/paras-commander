package pools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritePoolsStubCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pools.toml")
	created, err := WritePoolsStub(path)
	if err != nil {
		t.Fatalf("WritePoolsStub: %v", err)
	}
	if !created {
		t.Fatal("want created=true")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != PoolsStubTOML {
		t.Fatalf("stub content mismatch:\n%s", string(b))
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(f.Pools) != 0 {
		t.Fatalf("stub should decode to zero pools, got %d", len(f.Pools))
	}
}

func TestRefreshDocumentationPrependsDoc(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pools.toml")
	entries := `[[pools]]
name = "cpu"
max_parallel = 4
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
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Pools) != 1 || f.Pools[0].Name != "cpu" {
		t.Fatalf("pools = %+v, want one cpu pool", f.Pools)
	}
}

func TestRefreshDocumentationIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pools.toml")
	if _, err := WritePoolsStub(path); err != nil {
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

func TestWritePoolsStubIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pools.toml")
	if _, err := WritePoolsStub(path); err != nil {
		t.Fatalf("first WritePoolsStub: %v", err)
	}
	created, err := WritePoolsStub(path)
	if err != nil {
		t.Fatalf("second WritePoolsStub: %v", err)
	}
	if created {
		t.Fatal("want created=false on second call")
	}
}
