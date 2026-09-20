package scan

import (
	"runtime"
	"sort"
	"sync"

	"github.com/paranoidi/paras-commander/internal/search"
)

// testHoldMatch, if set, runs at the start of runMatchInPlace so tests can
// observe whether Index.RunMatch still holds the corpus lock while ranking.
var testHoldMatch func()

func runMatchInPlace(entries []Entry, req MatchRequest, shouldCancel func() bool) MatchOutput {
	if testHoldMatch != nil {
		testHoldMatch()
	}
	n := len(entries)
	q := search.Parse(req.Query)
	maxResults := req.MaxResults
	out := MatchOutput{
		Gen:        req.Gen,
		EntriesLen: n,
		OnlyDirs:   req.OnlyDirs,
		OnlyFiles:  req.OnlyFiles,
	}
	opts := search.Options{CaseInsensitive: req.CaseInsensitive}

	if q.Empty() {
		out.Ranked = emptyDisplayIndicesInPlace(entries, req.OnlyDirs, req.OnlyFiles, maxResults)
		out.FullRanked = out.Ranked
		out.DisplayRelLines = relLinesForIndicesInPlace(entries, out.Ranked)
		return out
	}

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
				result := q.Match(entries[i].RelLine, opts)
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

	filtered := filterRankedResults(entries, merged, req.OnlyDirs, req.OnlyFiles)
	out.FullRanked = indicesFromResults(filtered)

	display := filtered
	if maxResults > 0 && len(display) > maxResults {
		display = display[:maxResults]
	}
	out.Ranked = indicesFromResults(display)
	out.DisplayRelLines = relLinesForIndicesInPlace(entries, out.Ranked)

	if len(display) > 0 {
		out.MatchRanges = make(map[int][]search.Range)
		for _, r := range display {
			idx := r.Index
			if idx < 0 || idx >= n || len(r.Result.Ranges) == 0 {
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

func filterRankedResults(entries []Entry, raw []search.RankedResult, onlyDirs, onlyFiles bool) []search.RankedResult {
	if !onlyDirs && !onlyFiles {
		return raw
	}
	filtered := make([]search.RankedResult, 0, len(raw))
	for _, r := range raw {
		idx := r.Index
		if idx < 0 || idx >= len(entries) {
			continue
		}
		if onlyDirs && !entries[idx].IsDir {
			continue
		}
		if onlyFiles && entries[idx].IsDir {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func indicesFromResults(raw []search.RankedResult) []int {
	out := make([]int, len(raw))
	for i, r := range raw {
		out[i] = r.Index
	}
	return out
}

func emptyDisplayIndicesInPlace(entries []Entry, onlyDirs, onlyFiles bool, maxResults int) []int {
	n := len(entries)
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
		if onlyDirs && !entries[i].IsDir {
			continue
		}
		if onlyFiles && entries[i].IsDir {
			continue
		}
		out = append(out, i)
	}
	return out
}

func relLinesForIndicesInPlace(entries []Entry, ranked []int) []string {
	if len(ranked) == 0 {
		return nil
	}
	out := make([]string, len(ranked))
	for i, idx := range ranked {
		if idx >= 0 && idx < len(entries) {
			out[i] = entries[idx].RelLine
		}
	}
	return out
}
