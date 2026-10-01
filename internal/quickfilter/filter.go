package quickfilter

import (
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

// Options are the live settings the engine ranks and cycles with. Callers
// pass them on every call so config changes apply without rebuilding a Filter.
type Options struct {
	CaseInsensitive bool
	// CycleRanked cycles matches in score order instead of row order.
	CycleRanked bool
}

// OptionsFrom builds Options from the config filter settings; it is the one
// place the cycle_matches string ("ranked", case-insensitive, trimmed) is parsed.
func OptionsFrom(caseInsensitive bool, cycleMatches string) Options {
	return Options{
		CaseInsensitive: caseInsensitive,
		CycleRanked:     strings.EqualFold(strings.TrimSpace(cycleMatches), "ranked"),
	}
}

// Filter tracks quick filter state. It is a self-contained engine over a
// list of row labels and is shared by the file list, selections strip and dedup view.
type Filter struct {
	Query   string
	Cursor  int // rune offset within Query where typed/deleted runes apply
	Active  bool
	Editing bool
	results []filterResult // score order
	byIndex []filterResult // results sorted by row index (visual order, row lookups)
}

type filterResult struct {
	Index  int
	Score  int
	Ranges []search.Range
}

// Rebuild re-ranks names against the current query.
func (f *Filter) Rebuild(names []string, opts Options) {
	f.ClearResults()
	if f.Query == "" {
		f.Active = false
		return
	}
	query := search.Parse(f.Query)
	if query.Empty() {
		f.Active = false
		return
	}
	ranked := query.Rank(names, search.Options{CaseInsensitive: opts.CaseInsensitive})
	f.results = make([]filterResult, 0, len(ranked))
	for _, result := range ranked {
		f.results = append(f.results, filterResult{
			Index:  result.Index,
			Score:  result.Result.Score,
			Ranges: result.Result.Ranges,
		})
	}
	f.byIndex = slices.Clone(f.results)
	slices.SortFunc(f.byIndex, func(a, b filterResult) int { return a.Index - b.Index })
	f.Active = true
}

// ClearResults drops the ranked matches.
func (f *Filter) ClearResults() {
	f.results, f.byIndex = nil, nil
}

// Apply sets the query, re-ranks names and returns the row the cursor should jump to.
// ok is false when nothing matches.
func (f *Filter) Apply(query string, names []string, opts Options) (cursor int, ok bool) {
	f.Query = query
	f.Cursor = lineedit.ClampRuneCursor(f.Cursor, len([]rune(query)))
	f.Rebuild(names, opts)
	if len(f.results) == 0 {
		return 0, false
	}
	return primaryFilterMatchIndex(query, f.results), true
}

// InsertRune returns the query with r inserted at the caret and moves the caret.
func (f *Filter) InsertRune(r rune) string {
	f.Editing = true
	runes := []rune(f.Query)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	next, cur := lineedit.InsertRune(runes, pos, r)
	f.Cursor = cur
	return string(next)
}

// Backspace returns the query with the rune before the caret removed.
func (f *Filter) Backspace() (next string, changed bool) {
	runes := []rune(f.Query)
	if len(runes) == 0 {
		f.Editing = false
		return "", false
	}
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	if pos == 0 {
		return f.Query, false
	}
	f.Editing = true
	out, cur := lineedit.DeleteBefore(runes, pos)
	f.Cursor = cur
	return string(out), true
}

// UIActive reports whether the filter is editing or has an active query.
func (f Filter) UIActive() bool {
	return f.Active || f.Editing
}

// MatchCount is the number of matched rows.
func (f Filter) MatchCount() int { return len(f.results) }

// HasMatches reports whether the query matched at least one row.
func (f Filter) HasMatches() bool {
	return len(f.results) > 0
}

// Ranges returns the highlighted rune ranges for row index.
func (f Filter) Ranges(index int) []search.Range {
	if !f.Active {
		return nil
	}
	if i, ok := slices.BinarySearchFunc(f.byIndex, index, func(r filterResult, t int) int { return r.Index - t }); ok {
		return f.byIndex[i].Ranges
	}
	return nil
}

// Cycle moves cursor through matches (order per opts.CycleRanked), wrapping at the ends.
// ok is false when there are no matches so the caller falls back to a plain move.
func (f Filter) Cycle(cursor, delta int, opts Options) (int, bool) {
	if len(f.results) == 0 {
		return cursor, false
	}
	order := f.cycleOrder(opts)
	n := len(order)
	cur := -1
	for i := range order {
		if order[i].Index == cursor {
			cur = i
			break
		}
	}
	if cur < 0 {
		if delta > 0 {
			cur = nextFilterMatchIndex(order, cursor)
		} else {
			cur = previousFilterMatchIndex(order, cursor)
		}
	} else {
		cur = (cur + delta) % n
		if cur < 0 {
			cur += n
		}
	}
	return order[cur].Index, true
}

func (f Filter) cycleOrder(opts Options) []filterResult {
	if opts.CycleRanked {
		return f.results
	}
	return f.byIndex
}

func nextFilterMatchIndex(results []filterResult, cursor int) int {
	for i, result := range results {
		if result.Index > cursor {
			return i
		}
	}
	return 0
}

func previousFilterMatchIndex(results []filterResult, cursor int) int {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Index < cursor {
			return i
		}
	}
	return len(results) - 1
}

// primaryFilterMatchIndex picks the cursor row after the query changes.
// A single typed letter (after trim) uses the first visible match; longer queries use the best ranked match.
func primaryFilterMatchIndex(query string, ranked []filterResult) int {
	if len(ranked) == 0 {
		return 0
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return ranked[0].Index
	}
	if len([]rune(q)) == 1 {
		best := ranked[0].Index
		for _, r := range ranked[1:] {
			if r.Index < best {
				best = r.Index
			}
		}
		return best
	}
	return ranked[0].Index
}
