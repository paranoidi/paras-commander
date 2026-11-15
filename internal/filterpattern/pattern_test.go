package filterpattern

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileHoldsNone(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "filters.toml"))
	if err != nil || len(got) != 0 {
		t.Fatalf("Load = %v, %v; want none", got, err)
	}
	if got, err := Load(""); err != nil || got != nil {
		t.Fatalf("Load(\"\") = %v, %v", got, err)
	}
}

func TestUpsertRemoveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "filters.toml")
	lantern := Pattern{Name: "lantern", Description: "harbor clips", Pattern: `^meadow.*`, Mode: ModeRegex,
		Types: []string{"videos", "audio"}, Scope: ScopeBoth, CaseSensitive: true, Invert: true}
	orchard := Pattern{Name: "orchard", Pattern: "*.pebble", Mode: ModeShell, Scope: ScopeDirs, IncludeMeta: true, OnlyMeta: true}
	for _, p := range []Pattern{lantern, orchard} {
		if err := Upsert(path, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Load(path)
	if err != nil || len(got) != 2 || !got[0].Equal(lantern) || !got[1].Equal(orchard) {
		t.Fatalf("Load = %+v, %v", got, err)
	}

	lantern.Description = "willow"
	if err := Upsert(path, lantern); err != nil {
		t.Fatal(err)
	}
	got, _ = Load(path)
	if len(got) != 2 || got[0].Description != "willow" {
		t.Fatalf("Upsert did not replace in place: %+v", got)
	}

	if err := Remove(path, "lantern"); err != nil {
		t.Fatal(err)
	}
	got, _ = Load(path)
	if len(got) != 1 || got[0].Name != "orchard" {
		t.Fatalf("Remove left %+v", got)
	}
}

func TestSaveWithoutLocationFails(t *testing.T) {
	if err := Save("", nil); err == nil {
		t.Fatal("Save(\"\") succeeded")
	}
}

func TestResolveFile(t *testing.T) {
	if got := ResolveFile(" /tmp/x.toml ", "/cfg"); got != "/tmp/x.toml" {
		t.Fatalf("got %q", got)
	}
	if got := ResolveFile("", "/cfg"); got != filepath.Join("/cfg", "filters.toml") {
		t.Fatalf("got %q", got)
	}
	if got := ResolveFile("", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestEncodeStubParses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.toml")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := EncodeStub(f); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if got, err := Load(path); err != nil || len(got) != 0 {
		t.Fatalf("stub Load = %v, %v", got, err)
	}
}
