package scan

import (
	"fmt"
	"strconv"
	"testing"
)

func TestIndexDedupByRelLine(t *testing.T) {
	t.Parallel()
	idx := newIndex()
	added := idx.Append("", []Entry{
		{RelLine: "a.txt"},
		{RelLine: "a.txt"},
		{RelLine: "b.txt"},
	})
	if len(added) != 2 {
		t.Fatalf("added = %d, want 2", len(added))
	}
	if idx.Len() != 2 {
		t.Fatalf("len = %d, want 2", idx.Len())
	}
}

func TestIndexReplaceEntriesRebuildsDedup(t *testing.T) {
	t.Parallel()
	idx := newIndex()
	idx.Append("", []Entry{{RelLine: "old.txt"}})
	idx.ReplaceEntries("", []Entry{
		{RelLine: "x.txt"},
		{RelLine: "x.txt"},
		{RelLine: "y.txt"},
	})
	if idx.Len() != 2 {
		t.Fatalf("len = %d, want 2", idx.Len())
	}
	if _, ok := idx.EntryMetaForAbs("/root", "/root/x.txt"); !ok {
		t.Fatal("expected x.txt")
	}
}

func BenchmarkIndexAppend(b *testing.B) {
	batch := make([]Entry, 1000)
	for i := range batch {
		batch[i] = Entry{RelLine: "dir/file_" + strconv.Itoa(i) + ".txt", IsDir: false}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := newIndex()
		for j := 0; j < 100; j++ {
			for k := range batch {
				batch[k].RelLine = fmt.Sprintf("tree%d/file_%d.txt", j, k)
			}
			idx.Append("", batch)
		}
	}
}

func BenchmarkMatchInPlace(b *testing.B) {
	const n = 100_000
	entries := make([]Entry, n)
	for i := range entries {
		entries[i] = Entry{RelLine: fmt.Sprintf("project/src/module_%d.go", i), IsDir: i%50 == 0}
	}
	req := MatchRequest{Query: "module", MaxResults: 500, CaseInsensitive: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runMatchInPlace(entries, req, nil)
	}
}
