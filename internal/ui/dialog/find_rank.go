package dialog

import (
	"github.com/paranoidi/paras-commander/internal/search"
)

// RankFindEntries ranks all indexed find entries against query without a display cap.
// onlyDirs / onlyFiles mirror FindDialogState filters.
func RankFindEntries(entries []FindEntry, query string, onlyDirs, onlyFiles, caseInsensitive bool) (ranked []int, matchRanges map[int][]search.Range) {
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.RelLine
	}
	q := search.Parse(query)
	opts := search.Options{CaseInsensitive: caseInsensitive}
	raw := q.Rank(lines, opts)
	ranked = make([]int, len(raw))
	if q.Empty() {
		matchRanges = nil
	} else {
		matchRanges = make(map[int][]search.Range)
	}
	for i, r := range raw {
		ranked[i] = r.Index
		if matchRanges != nil && r.Index >= 0 && r.Index < len(lines) && len(r.Result.Ranges) > 0 {
			matchRanges[r.Index] = r.Result.Ranges
		}
	}
	if onlyDirs {
		filtered := ranked[:0]
		for _, idx := range ranked {
			if idx >= 0 && idx < len(entries) && entries[idx].IsDir {
				filtered = append(filtered, idx)
			}
		}
		ranked = filtered
	} else if onlyFiles {
		filtered := ranked[:0]
		for _, idx := range ranked {
			if idx >= 0 && idx < len(entries) && !entries[idx].IsDir {
				filtered = append(filtered, idx)
			}
		}
		ranked = filtered
	}
	return ranked, matchRanges
}
