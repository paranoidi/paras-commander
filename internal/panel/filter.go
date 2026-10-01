package panel

import (
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

// FilterState tracks quick filter state. It is a self-contained engine over a
// list of row labels and is shared by the file list, selections strip and dedup view.
type FilterState struct {
	Query           string
	Cursor          int // rune offset within Query where typed/deleted runes apply
	Active          bool
	Editing         bool
	CaseInsensitive bool
	// CycleMatches is "visual" (default) or "ranked"; empty means visual.
	// It controls Up/Down traversal among quick-filter matches.
	CycleMatches string
	results      []filterResult // score order
	byIndex      []filterResult // results sorted by row index (visual order, row lookups)
}

type filterResult struct {
	Index  int
	Score  int
	Ranges []search.Range
}

// Rebuild re-ranks names against the current query.
func (f *FilterState) Rebuild(names []string) {
	f.clearResults()
	if f.Query == "" {
		f.Active = false
		return
	}
	query := search.Parse(f.Query)
	if query.Empty() {
		f.Active = false
		return
	}
	ranked := query.Rank(names, search.Options{CaseInsensitive: f.CaseInsensitive})
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

func (f *FilterState) clearResults() {
	f.results, f.byIndex = nil, nil
}

// Apply sets the query, re-ranks names and returns the row the cursor should jump to.
// ok is false when nothing matches.
func (f *FilterState) Apply(query string, names []string) (cursor int, ok bool) {
	f.Query = query
	f.Cursor = lineedit.ClampRuneCursor(f.Cursor, len([]rune(query)))
	f.Rebuild(names)
	if len(f.results) == 0 {
		return 0, false
	}
	return primaryFilterMatchIndex(query, f.results), true
}

// InsertRune returns the query with r inserted at the caret and moves the caret.
func (f *FilterState) InsertRune(r rune) string {
	f.Editing = true
	runes := []rune(f.Query)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	next := make([]rune, 0, len(runes)+1)
	next = append(next, runes[:pos]...)
	next = append(next, r)
	next = append(next, runes[pos:]...)
	f.Cursor = pos + 1
	return string(next)
}

// Backspace returns the query with the rune before the caret removed.
func (f *FilterState) Backspace() (next string, changed bool) {
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
	out := make([]rune, 0, len(runes)-1)
	out = append(out, runes[:pos-1]...)
	out = append(out, runes[pos:]...)
	f.Cursor = pos - 1
	return string(out), true
}

// UIActive reports whether the filter is editing or has an active query.
func (f FilterState) UIActive() bool {
	return f.Active || f.Editing
}

// HasMatches reports whether the query matched at least one row.
func (f FilterState) HasMatches() bool {
	return len(f.results) > 0
}

// Ranges returns the highlighted rune ranges for row index.
func (f FilterState) Ranges(index int) []search.Range {
	if !f.Active {
		return nil
	}
	if i, ok := slices.BinarySearchFunc(f.byIndex, index, func(r filterResult, t int) int { return r.Index - t }); ok {
		return f.byIndex[i].Ranges
	}
	return nil
}

// Cycle moves cursor through matches (order per CycleMatches), wrapping at the ends.
// ok is false when there are no matches so the caller falls back to a plain move.
func (f FilterState) Cycle(cursor, delta int) (int, bool) {
	if len(f.results) == 0 {
		return cursor, false
	}
	order := f.cycleOrder()
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

func (f FilterState) cycleOrder() []filterResult {
	if f.cycleMatchesRanked() {
		return f.results
	}
	return f.byIndex
}

func (f FilterState) cycleMatchesRanked() bool {
	return strings.EqualFold(strings.TrimSpace(f.CycleMatches), "ranked")
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
