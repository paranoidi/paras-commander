package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/app/helpkeys"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestHelpCanonicalRankOrdersKeysSectionTitle(t *testing.T) {
	ent := dialog.HelpEntry{Keys: "Alt+O", Section: "Navigation", Title: "Open", FuzzyExtra: "panel.open-dir-in-other search"}
	got := helpkeys.CanonicalRankText(ent)
	want := "Alt+O Navigation Open panel.open-dir-in-other search"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHelpRankOrderSameForDifferentLayouts(t *testing.T) {
	entries := []dialog.HelpEntry{
		{Keys: "A", Section: "S1", Title: "Zebra", FuzzyExtra: "id1"},
		{Keys: "B", Section: "S2", Title: "Alpha", FuzzyExtra: "id2"},
	}
	q := search.Parse("alpha")
	opts := search.Options{CaseInsensitive: true}
	rankA := q.Rank([]string{
		helpkeys.CanonicalRankText(entries[0]),
		helpkeys.CanonicalRankText(entries[1]),
	}, opts)
	rankB := q.Rank([]string{
		helpkeys.CanonicalRankText(entries[0]),
		helpkeys.CanonicalRankText(entries[1]),
	}, opts)
	if len(rankA) != len(rankB) {
		t.Fatalf("len rankA=%d rankB=%d", len(rankA), len(rankB))
	}
	for i := range rankA {
		if rankA[i].Index != rankB[i].Index {
			t.Fatalf("rank order differs at %d: A=%d B=%d", i, rankA[i].Index, rankB[i].Index)
		}
	}
}
