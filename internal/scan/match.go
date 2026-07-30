package scan

import (
	"runtime"
	"sort"
	"sync"

	"github.com/paranoidi/paras-commander/internal/search"
)

func runMatch(
	lines []string,
	isDirs []bool,
	req MatchRequest,
	shouldCancel func() bool,
) MatchOutput {
	q := search.Parse(req.Query)
	maxResults := req.MaxResults
	out := MatchOutput{
		Gen:        req.Gen,
		EntriesLen: len(lines),
		OnlyDirs:   req.OnlyDirs,
		OnlyFiles:  req.OnlyFiles,
	}
	opts := search.Options{CaseInsensitive: req.CaseInsensitive}

	if q.Empty() {
		out.Ranked = emptyDisplayIndices(len(lines), req.OnlyDirs, req.OnlyFiles, isDirs, maxResults)
		out.FullRanked = out.Ranked
		out.DisplayRelLines = relLinesForIndices(lines, out.Ranked)
		return out
	}

	n := len(lines)
	if n == 0 {
		return out
	}

	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > n {
		workers = 1
	}

	type shardMatch struct {
		results []search.RankedResult
	}
	shards := make([]shardMatch, workers)
	var wg sync.WaitGroup
	chunk := (n + workers - 1) / workers
	for w := 0; w < workers; w++ {
		start := w * chunk
		end := start + chunk
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(wi, lo, hi int) {
			defer wg.Done()
			local := make([]search.RankedResult, 0, (hi-lo)/4)
			for i := lo; i < hi; i++ {
				if shouldCancel != nil && i%10000 == 0 && i > lo && shouldCancel() {
					return
				}
				result := q.Match(lines[i], opts)
				if result.Matched {
					local = append(local, search.RankedResult{Index: i, Result: result})
				}
			}
			shards[wi].results = local
		}(w, start, end)
	}
	wg.Wait()
	if shouldCancel != nil && shouldCancel() {
		return MatchOutput{Gen: req.Gen}
	}

	var merged []search.RankedResult
	for _, sh := range shards {
		if len(sh.results) > 0 {
			merged = append(merged, sh.results...)
		}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Result.Score > merged[j].Result.Score
	})

	out.FullRanked = filterRankIndices(indicesFromResults(merged), req.OnlyDirs, req.OnlyFiles, isDirs)

	raw := merged
	if maxResults > 0 && len(raw) > maxResults {
		raw = raw[:maxResults]
	}
	out.Ranked = filterRankIndices(indicesFromResults(raw), req.OnlyDirs, req.OnlyFiles, isDirs)
	out.DisplayRelLines = relLinesForIndices(lines, out.Ranked)

	if len(raw) > 0 {
		out.MatchRanges = make(map[int][]search.Range)
		for _, r := range raw {
			idx := r.Index
			if idx < 0 || idx >= len(lines) || len(r.Result.Ranges) == 0 {
				continue
			}
			if req.OnlyDirs && (idx >= len(isDirs) || !isDirs[idx]) {
				continue
			}
			if req.OnlyFiles && (idx >= len(isDirs) || isDirs[idx]) {
				continue
			}
			out.MatchRanges[idx] = r.Result.Ranges
		}
		if len(out.MatchRanges) == 0 {
			out.MatchRanges = nil
		}
	}
	return out
}

func indicesFromResults(raw []search.RankedResult) []int {
	out := make([]int, len(raw))
	for i, r := range raw {
		out[i] = r.Index
	}
	return out
}

func filterRankIndices(indices []int, onlyDirs, onlyFiles bool, isDirs []bool) []int {
	if !onlyDirs && !onlyFiles {
		return append([]int(nil), indices...)
	}
	filtered := make([]int, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(isDirs) {
			continue
		}
		if onlyDirs && !isDirs[idx] {
			continue
		}
		if onlyFiles && isDirs[idx] {
			continue
		}
		filtered = append(filtered, idx)
	}
	return filtered
}

func emptyDisplayIndices(n int, onlyDirs, onlyFiles bool, isDirs []bool, maxResults int) []int {
	if n == 0 {
		return nil
	}
	cap := n
	if maxResults > 0 && cap > maxResults {
		cap = maxResults
	}
	if !onlyDirs && !onlyFiles {
		out := make([]int, cap)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, cap)
	for i := 0; i < n && len(out) < cap; i++ {
		if onlyDirs && (i >= len(isDirs) || !isDirs[i]) {
			continue
		}
		if onlyFiles && (i >= len(isDirs) || isDirs[i]) {
			continue
		}
		out = append(out, i)
	}
	return out
}

func relLinesForIndices(lines []string, ranked []int) []string {
	if len(ranked) == 0 {
		return nil
	}
	out := make([]string, len(ranked))
	for i, idx := range ranked {
		if idx >= 0 && idx < len(lines) {
			out[i] = lines[idx]
		}
	}
	return out
}
