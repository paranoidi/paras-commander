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
