//go:build linux

package localfs

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestListDirFillsFileColumnFacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "orchard.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	listing, err := ListDir(dir, ListOptions{})
	if err != nil || len(listing.Entries) != 1 {
		t.Fatalf("ListDir = %+v, %v", listing, err)
	}
	e := listing.Entries[0]
	if e.Links != 1 || e.ChangedAt.IsZero() || e.AccessedAt.IsZero() {
		t.Fatalf("entry = %+v", e)
	}
	if e.Owner == "" || e.Group == "" {
		t.Fatalf("owner = %q group = %q", e.Owner, e.Group)
	}
	if u, err := user.Current(); err == nil && e.Owner != u.Username && e.Owner != u.Uid {
		t.Fatalf("owner = %q, want %q or %q", e.Owner, u.Username, u.Uid)
	}
	if !e.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt = %v without BirthTime", e.CreatedAt)
	}
}

func TestIDNameCacheLooksUpOncePerIDAndFallsBackToNumber(t *testing.T) {
	calls := 0
	c := idNameCache{names: map[uint32]string{}, lookup: func(id string) (string, error) {
		calls++
		if id == "7" {
			return "wren", nil
		}
		return "", os.ErrNotExist
	}}
	for range 2 {
		if got := c.name(7); got != "wren" {
			t.Fatalf("name = %q", got)
		}
		if got := c.name(9); got != "9" {
			t.Fatalf("unknown id = %q", got)
		}
	}
	if calls != 2 {
		t.Fatalf("lookups = %d, want one per id", calls)
	}
}
