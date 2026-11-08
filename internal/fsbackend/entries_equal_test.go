package fsbackend

import (
	"testing"
	"time"
)

func TestEntriesListingEqual(t *testing.T) {
	mod := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	a := []Entry{
		{Name: "a.txt", Type: EntryFile, Size: 1, ModifiedAt: mod},
		{Name: "dir", Type: EntryDirectory, Size: 0, ModifiedAt: mod},
	}
	b := []Entry{
		{Name: "dir", Type: EntryDirectory, Size: 0, ModifiedAt: mod},
		{Name: "a.txt", Type: EntryFile, Size: 1, ModifiedAt: mod},
	}
	if !EntriesListingEqual(a, b) {
		t.Fatal("order-independent equal listings reported as different")
	}
	c := append([]Entry(nil), a...)
	c[0].Size = 2
	if EntriesListingEqual(a, c) {
		t.Fatal("size change not detected")
	}
	if EntriesListingEqual(a, a[:1]) {
		t.Fatal("length mismatch not detected")
	}
}

func TestEntriesListingEqualIndexState(t *testing.T) {
	mod := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	file := func(status string) []Entry {
		return []Entry{{Name: "willow.txt", Type: EntryFile, Size: 7, ModifiedAt: mod, IndexStatus: status}}
	}
	dir := func(status string) []Entry {
		return []Entry{{Name: "orchard", Type: EntryDirectory, ModifiedAt: mod, IndexStatus: status}}
	}
	// A file that became a ghost row keeps its indexed size and mtime; only the state differs.
	if EntriesListingEqual(file("indexed"), file("missing")) || EntriesListingEqual(file("missing"), file("indexed")) {
		t.Fatal("file indexed <-> missing not detected")
	}
	// The drift pass owns these directory states, so a fresh overlay must equal any of them.
	for _, st := range []string{"indexed", "tagged", "unchecked", "differs"} {
		if !EntriesListingEqual(dir("indexed"), dir(st)) {
			t.Fatalf("directory state %q counted as a change", st)
		}
	}
	if EntriesListingEqual(dir("indexed"), dir("missing")) || EntriesListingEqual(dir("new"), dir("indexed")) {
		t.Fatal("directory ghost/new transition not detected")
	}
}

func TestEntriesListingEqualSeesMetadataButNotAccessTime(t *testing.T) {
	stamp := time.Unix(1700000000, 0)
	base := Entry{Name: "orchard.txt", Mode: 0o644, Owner: "wren", Group: "wren", Links: 1, ChangedAt: stamp, AccessedAt: stamp}
	for name, change := range map[string]func(*Entry){
		"mode":    func(e *Entry) { e.Mode = 0o600 },
		"owner":   func(e *Entry) { e.Owner = "heron" },
		"group":   func(e *Entry) { e.Group = "heron" },
		"links":   func(e *Entry) { e.Links = 2 },
		"changed": func(e *Entry) { e.ChangedAt = stamp.Add(time.Second) },
	} {
		other := base
		change(&other)
		if EntriesListingEqual([]Entry{base}, []Entry{other}) {
			t.Fatalf("%s change not detected", name)
		}
	}
	other := base
	other.AccessedAt = stamp.Add(time.Hour)
	if !EntriesListingEqual([]Entry{base}, []Entry{other}) {
		t.Fatal("an access time change must not count")
	}
}
