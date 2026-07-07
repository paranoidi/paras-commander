package testutil

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// SyntheticDedupSnapshot builds a completed dedup snapshot with the given
// number of two-file duplicate groups, for benchmarks and tests.
func SyntheticDedupSnapshot(groups int) compare.DedupSnapshot {
	root := pathloc.MustParse("/scan/root")
	snap := compare.DedupSnapshot{
		Root:  root,
		Phase: compare.DedupDone,
	}
	for g := range groups {
		relA := fmt.Sprintf("group-%d/a.bin", g)
		relB := fmt.Sprintf("group-%d/b.bin", g)
		snap.Groups = append(snap.Groups, compare.DedupGroup{
			Size: 4096,
			Files: []compare.DedupFile{
				{Rel: relA, Abs: pathloc.MustParse("/scan/root/" + relA)},
				{Rel: relB, Abs: pathloc.MustParse("/scan/root/" + relB)},
			},
		})
	}
	return snap
}
