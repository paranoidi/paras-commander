package scan

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
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

func TestIndexAppendCompletesWhileMatchRanksSnapshot(t *testing.T) {
	idx := newIndex()
	idx.Append("", []Entry{
		{RelLine: "alpha.txt"},
		{RelLine: "beta.txt"},
	})
	snapLen := idx.Len()

	inRank := make(chan struct{})
	release := make(chan struct{})
	testHoldMatch = func() {
		close(inRank)
		<-release
	}
	defer func() { testHoldMatch = nil }()

	var matchOut MatchOutput
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		matchOut = idx.RunMatch(MatchRequest{Query: "a", MaxResults: 10}, nil)
	}()

	select {
	case <-inRank:
	case <-time.After(2 * time.Second):
		t.Fatal("match did not reach ranking")
	}

	done := make(chan struct{})
	go func() {
		idx.Append("", []Entry{{RelLine: "gamma.txt"}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		close(release)
		wg.Wait()
		t.Fatal("Append blocked while match ranked")
	}

	if idx.Len() != snapLen+1 {
		t.Fatalf("len after Append = %d, want %d", idx.Len(), snapLen+1)
	}

	close(release)
	wg.Wait()

	for _, i := range matchOut.Ranked {
		if i < 0 || i >= snapLen {
			t.Fatalf("ranked index %d outside snapshot len %d", i, snapLen)
		}
	}
	for _, i := range matchOut.FullRanked {
		if i < 0 || i >= snapLen {
			t.Fatalf("full ranked index %d outside snapshot len %d", i, snapLen)
		}
	}
	if matchOut.EntriesLen != snapLen {
		t.Fatalf("EntriesLen = %d, want snapshot %d", matchOut.EntriesLen, snapLen)
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
