package panel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
)

// TestFirstListingDoesNotMarkEntriesNew guards the startup path: panels are built by NewDeferred
// with Path already set, so the first listing looks like a same-directory reload. Diffing it
// against the still-empty Entries would mark every file in the directory as newly created.
func TestFirstListingDoesNotMarkEntriesNew(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"harbor.txt", "meadow.txt", "walnut.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := NewDeferred(dir, localfs.DefaultListOptions(), nil)
	if err != nil {
		t.Fatalf("NewDeferred: %v", err)
	}
	if p.PathString() != dir {
		t.Fatalf("PathString() = %q, want %q", p.PathString(), dir)
	}
	if len(p.Entries) != 0 {
		t.Fatalf("NewDeferred must not read the directory, got %d entries", len(p.Entries))
	}
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(p.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(p.Entries))
	}
	for _, e := range p.Entries {
		if tier := p.NewFileMarkTier(e); tier != panellist.NewFileMarkNone {
			t.Fatalf("%s marked new (tier %v) after the first listing", e.Name, tier)
		}
	}

	// A file that actually appears after the first listing must still be marked.
	if err := os.WriteFile(filepath.Join(dir, "orchid.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Refresh(0); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tier := p.NewFileMarkTier(localfs.Entry{Name: "orchid.txt"}); tier != panellist.NewFileMarkLatest {
		t.Fatalf("orchid.txt tier = %v, want latest after appearing in a later listing", tier)
	}
	if tier := p.NewFileMarkTier(localfs.Entry{Name: "harbor.txt"}); tier != panellist.NewFileMarkNone {
		t.Fatalf("pre-existing harbor.txt tier = %v, want none", tier)
	}
}
