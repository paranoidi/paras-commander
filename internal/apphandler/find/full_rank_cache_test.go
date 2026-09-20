package find

import (
	"fmt"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestFindDialogResultIndicesCachedZeroMatch(t *testing.T) {
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:                 true,
			Query:                "zzzz-nomatch",
			RootPath:             "/root",
			Entries:              makeFindEntries(50),
			FullRanked:           nil,
			FullRankedGen:        4,
			FullRankedEntriesLen: 50,
		},
	}
	h := newTestFindHandler(host, model)
	h.rankGen = 4
	st := &h.model.FindDialog

	called := false
	testOnRankFindCorpus = func() { called = true }
	defer func() { testOnRankFindCorpus = nil }()

	got := h.findDialogResultIndices(st)
	if called {
		t.Fatal("cached zero-match FullRanked reranked the corpus on the event loop")
	}
	if got == nil {
		t.Fatal("zero-match cache returned nil indices (full-corpus fallthrough)")
	}
	if len(got) != 0 {
		t.Fatalf("zero-match indices = %v, want empty", got)
	}

	files, dirs := h.CountGroupMatches(GroupSelectRequest{
		Pattern:     "*",
		PatternMode: panel.GroupPatternShell,
	}, false)
	if files != 0 || dirs != 0 {
		t.Fatalf("group preview = files=%d dirs=%d, want 0, 0 for cached zero-match", files, dirs)
	}
}

func TestFindDialogResultIndicesCorpusGrowthInvalidates(t *testing.T) {
	entries := makeFindEntries(8)
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:                 true,
			Query:                "file",
			RootPath:             "/root",
			Entries:              entries,
			FullRanked:           []int{0},
			FullRankedGen:        2,
			FullRankedEntriesLen: 1,
		},
	}
	h := newTestFindHandler(host, model)
	h.rankGen = 2
	st := &h.model.FindDialog

	got := h.findDialogResultIndices(st)
	if len(got) == 1 && got[0] == 0 {
		t.Fatal("used stale one-hit FullRanked after corpus growth")
	}
	if !st.RankPending {
		t.Fatal("corpus growth should refresh FullRanked asynchronously")
	}
}

func makeFindEntries(n int) []dialog.FindEntry {
	out := make([]dialog.FindEntry, n)
	for i := range out {
		out[i] = dialog.FindEntry{RelLine: fmt.Sprintf("file_%02d.txt", i)}
	}
	return out
}
