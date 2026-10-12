package ops

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestMassRenamePatternsResolveFile(t *testing.T) {
	if got := MassRenamePatternsResolveFile("", "/home/walrus/.config/pc"); got != filepath.Join("/home/walrus/.config/pc", "patterns.toml") {
		t.Fatalf("resolve default = %q", got)
	}
	if got := MassRenamePatternsResolveFile("/custom/lantern.toml", "/home/walrus/.config/pc"); got != "/custom/lantern.toml" {
		t.Fatalf("resolve explicit = %q, want /custom/lantern.toml", got)
	}
}

func TestLoadMassRenamePatternsMissingFile(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadMassRenamePatterns(filepath.Join(dir, "missing.toml"))
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if !slices.Equal(got, DefaultMassRenamePatterns) {
		t.Fatalf("got = %#v, want DefaultMassRenamePatterns", got)
	}
}

func TestUpsertMassRenamePatternSeedsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.toml")
	if err := UpsertMassRenamePattern(path, MassRenamePattern{Name: "meadow", Mode: "simple"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(got) != len(DefaultMassRenamePatterns)+1 || got[len(got)-1].Name != "meadow" {
		t.Fatalf("got = %+v, want defaults followed by meadow", got)
	}
}

func TestDefaultMassRenamePatternsPadLastNumber(t *testing.T) {
	want := []string{"walnut 7 pear 05 plum", "walnut 7 pear 005 plum"}
	for i, p := range DefaultMassRenamePatterns {
		rx, err := MassRenameCompileRegex(p.Find, p.CaseFold)
		if err != nil {
			t.Fatalf("%s: compile: %v", p.Name, err)
		}
		if got := massRenameRegexReplace(rx, "walnut 7 pear 5 plum", p.Replace); got != want[i] {
			t.Fatalf("%s: got %q, want %q", p.Name, got, want[i])
		}
	}
}

func TestSaveLoadMassRenamePatternsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patterns.toml")
	want := []MassRenamePattern{
		{Name: "harbor", Description: "strip camera prefix", Mode: "simple", Find: "IMG_", Replace: "", CaseFold: true, StripSpaces: true, IgnoreExt: true},
		{Name: "lantern", Description: "regex cleanup", Mode: "regex", Find: `\d+`, Replace: "#", CaseFold: false},
	}
	if err := SaveMassRenamePatterns(path, want); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}
	got, err := LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pattern %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestUpsertMassRenamePatternAppendsAndOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patterns.toml")
	// An existing (empty) file is never seeded with DefaultMassRenamePatterns.
	if err := SaveMassRenamePatterns(path, nil); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}

	if err := UpsertMassRenamePattern(path, MassRenamePattern{Name: "meadow", Description: "first", Mode: "simple"}); err != nil {
		t.Fatalf("Upsert 1: %v", err)
	}
	if err := UpsertMassRenamePattern(path, MassRenamePattern{Name: "orchid", Description: "second", Mode: "regex"}); err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}
	got, err := LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	// Overwriting "meadow" replaces its entry in place rather than appending a third.
	if err := UpsertMassRenamePattern(path, MassRenamePattern{Name: "meadow", Description: "replaced", Mode: "capitalize"}); err != nil {
		t.Fatalf("Upsert overwrite: %v", err)
	}
	got, err = LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) after overwrite = %d, want 2", len(got))
	}
	if got[0].Description != "replaced" || got[0].Mode != "capitalize" {
		t.Fatalf("meadow entry = %+v, want overwritten", got[0])
	}
}

func TestRemoveMassRenamePattern(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patterns.toml")
	patterns := []MassRenamePattern{
		{Name: "harbor", Description: "a"},
		{Name: "lantern", Description: "b"},
	}
	if err := SaveMassRenamePatterns(path, patterns); err != nil {
		t.Fatalf("SaveMassRenamePatterns: %v", err)
	}
	if err := RemoveMassRenamePattern(path, "harbor"); err != nil {
		t.Fatalf("RemoveMassRenamePattern: %v", err)
	}
	got, err := LoadMassRenamePatterns(path)
	if err != nil {
		t.Fatalf("LoadMassRenamePatterns: %v", err)
	}
	if len(got) != 1 || got[0].Name != "lantern" {
		t.Fatalf("got = %+v, want only lantern", got)
	}
}
