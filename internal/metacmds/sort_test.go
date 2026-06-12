package metacmds_test

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/metacmds"
)

func TestDecode_columnAndOrder(t *testing.T) {
	toml := `
[[entry]]
name = "line-count"
column = "Lines"
order = 10
description = "Line count"
file = "wc -l < %f"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := mf.Entries[0]
	if e.Column != "Lines" {
		t.Errorf("Column = %q, want Lines", e.Column)
	}
	if e.Order != 10 {
		t.Errorf("Order = %d, want 10", e.Order)
	}
}

func TestDecode_columnDefaultsToName(t *testing.T) {
	toml := `
[[entry]]
name = "size"
description = "File size"
file = "stat -c '%s' %f"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mf.Entries[0].Column != "size" {
		t.Errorf("Column = %q, want size", mf.Entries[0].Column)
	}
	if mf.Entries[0].Order != 0 {
		t.Errorf("Order = %d, want 0", mf.Entries[0].Order)
	}
}

func TestDecode_negativeOrder(t *testing.T) {
	toml := `
[[entry]]
name = "bad"
description = "Bad order"
order = -1
file = "echo test"
`
	_, err := metacmds.Decode([]byte(toml))
	if err == nil {
		t.Fatal("expected error for negative order, got nil")
	}
}

func TestSortEntriesForDisplay(t *testing.T) {
	toml := `
[[entry]]
name = "zeta"
description = "Z"
order = 20
file = "true"

[[entry]]
name = "alpha"
description = "A"
order = 10
file = "true"

[[entry]]
name = "beta"
description = "B"
order = 10
file = "true"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := metacmds.SortEntriesForDisplay([]string{"zeta", "alpha", "beta"}, mf)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3", len(got))
	}
	want := []string{"alpha", "beta", "zeta"}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestSortedEntries(t *testing.T) {
	toml := `
[[entry]]
name = "b"
description = "B"
order = 1
file = "true"

[[entry]]
name = "a"
description = "A"
order = 0
file = "true"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := metacmds.SortedEntries(mf)
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("got order %v, want [a b]", []string{got[0].Name, got[1].Name})
	}
}
