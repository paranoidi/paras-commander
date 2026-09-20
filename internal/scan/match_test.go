package scan

import (
	"fmt"
	"testing"
)

func TestRunMatchFiltersTypeBeforeMaxResults(t *testing.T) {
	t.Parallel()
	const capN = 3
	entries := make([]Entry, capN+1)
	for i := 0; i < capN; i++ {
		entries[i] = Entry{RelLine: fmt.Sprintf("needle_file_%d.txt", i), IsDir: false}
	}
	entries[capN] = Entry{RelLine: "needle_dir", IsDir: true}

	out := runMatchInPlace(entries, MatchRequest{
		Query:      "needle",
		OnlyDirs:   true,
		MaxResults: capN,
	}, nil)
	if len(out.Ranked) == 0 {
		t.Fatal("OnlyDirs Ranked is empty; eligible dir ranked past MaxResults should still appear")
	}
	if len(out.Ranked) != 1 || out.Ranked[0] != capN {
		t.Fatalf("OnlyDirs Ranked = %v, want [%d] (the directory)", out.Ranked, capN)
	}
	if len(out.FullRanked) != 1 || out.FullRanked[0] != capN {
		t.Fatalf("OnlyDirs FullRanked = %v, want [%d]", out.FullRanked, capN)
	}

	filesFirst := make([]Entry, capN+1)
	for i := 0; i < capN; i++ {
		filesFirst[i] = Entry{RelLine: fmt.Sprintf("needle_dir_%d", i), IsDir: true}
	}
	filesFirst[capN] = Entry{RelLine: "needle_file.txt", IsDir: false}
	out = runMatchInPlace(filesFirst, MatchRequest{
		Query:      "needle",
		OnlyFiles:  true,
		MaxResults: capN,
	}, nil)
	if len(out.Ranked) == 0 {
		t.Fatal("OnlyFiles Ranked is empty; eligible file ranked past MaxResults should still appear")
	}
	if len(out.Ranked) != 1 || out.Ranked[0] != capN {
		t.Fatalf("OnlyFiles Ranked = %v, want [%d] (the file)", out.Ranked, capN)
	}
}

func TestRunMatchEmptyQueryFiltersThenCaps(t *testing.T) {
	t.Parallel()
	entries := []Entry{
		{RelLine: "a.txt"},
		{RelLine: "b.txt"},
		{RelLine: "dir", IsDir: true},
		{RelLine: "c.txt"},
		{RelLine: "other", IsDir: true},
	}
	out := runMatchInPlace(entries, MatchRequest{
		OnlyDirs:   true,
		MaxResults: 1,
	}, nil)
	if len(out.Ranked) != 1 || out.Ranked[0] != 2 {
		t.Fatalf("empty-query OnlyDirs Ranked = %v, want [2]", out.Ranked)
	}
}
