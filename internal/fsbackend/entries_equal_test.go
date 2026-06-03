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
