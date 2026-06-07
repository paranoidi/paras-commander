package dialog

import (
	"fmt"
	"testing"
)

func TestRankFindEntriesReturnsFullCorpusBeyondDisplayCap(t *testing.T) {
	const n = 250
	entries := make([]FindEntry, n)
	for i := range entries {
		entries[i] = FindEntry{RelLine: fmt.Sprintf("file_%04d.txt", i), IsDir: false}
	}
	ranked, _ := RankFindEntries(entries, "", false, false, false)
	if len(ranked) != n {
		t.Fatalf("ranked len = %d, want %d (no display cap)", len(ranked), n)
	}
}

func TestRankFindEntriesOnlyDirectoriesFilter(t *testing.T) {
	entries := []FindEntry{
		{RelLine: "dir", IsDir: true},
		{RelLine: "file.txt", IsDir: false},
	}
	ranked, _ := RankFindEntries(entries, "", true, false, false)
	if len(ranked) != 1 || ranked[0] != 0 {
		t.Fatalf("onlyDirs ranked = %v, want [0]", ranked)
	}
}

func TestRankFindEntriesOnlyFilesFilter(t *testing.T) {
	entries := []FindEntry{
		{RelLine: "dir", IsDir: true},
		{RelLine: "file.txt", IsDir: false},
	}
	ranked, _ := RankFindEntries(entries, "", false, true, false)
	if len(ranked) != 1 || ranked[0] != 1 {
		t.Fatalf("onlyFiles ranked = %v, want [1]", ranked)
	}
}
