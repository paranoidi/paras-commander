package previewpanel

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// SearchMatch is a half-open rune range [Start,End) into the flat pre-wrap body cell
// stream (same indexing as bodyCells), plus the 0-based source line it starts on.
type SearchMatch struct {
	Start, End, Line int
}

// SearchState tracks incremental "/" search inside the fullscreen preview.
type SearchState struct {
	Query   string
	Active  bool // a search has been started; matches (if any) stay highlighted
	Editing bool // still capturing keystrokes (before Enter/Esc)
	Matches []SearchMatch
	Current int // index into Matches; -1 when none

	// MatchStyle/CurrentStyle are resolved once at search-start from the app's theme
	// (FuzzyHighlight/FuzzyHighlightCursor) — previewpanel has no theme dependency,
	// so the caller supplies concrete styles rather than this package resolving them.
	MatchStyle, CurrentStyle tcell.Style
}

// cacheKey fingerprints the parts of SearchState that affect rendered highlighting, for
// wrap-cache invalidation. Zero when there is nothing to highlight.
func (s SearchState) cacheKey() uint64 {
	if !s.Active || len(s.Matches) == 0 {
		return 0
	}
	h := uint64(1469598103934665603) // FNV offset basis
	for _, r := range s.Query {
		h ^= uint64(r)
		h *= 1099511628211
	}
	h ^= uint64(len(s.Matches))
	h *= 1099511628211
	h ^= uint64(s.Current + 1)
	h *= 1099511628211
	return h
}

// FindMatches returns all non-overlapping case-insensitive literal occurrences of query
// in st's body text.
// ponytail: O(N*M) rune scan — fine for interactive query-sized needles; revisit only if
// profiling ever shows this is hot.
func (st State) FindMatches(query string) []SearchMatch {
	q := []rune(query)
	if len(q) == 0 {
		return nil
	}
	cells := st.bodyCellsRaw(tcell.StyleDefault) // style is irrelevant; only .R is read
	if len(cells) < len(q) {
		return nil
	}
	line := make([]int, len(cells))
	l := 0
	for i, c := range cells {
		line[i] = l
		if c.R == '\n' {
			l++
		}
	}
	var out []SearchMatch
	for i := 0; i+len(q) <= len(cells); {
		if runesEqualFold(cells[i:i+len(q)], q) {
			out = append(out, SearchMatch{Start: i, End: i + len(q), Line: line[i]})
			i += len(q)
			continue
		}
		i++
	}
	return out
}

func runesEqualFold(cells []AnsiCell, q []rune) bool {
	for i, r := range q {
		if unicode.ToLower(cells[i].R) != unicode.ToLower(r) {
			return false
		}
	}
	return true
}

// StartSearch begins an incremental "/" search: query capture with live highlighting.
func (st *State) StartSearch() {
	st.Search = SearchState{Current: -1, Active: true, Editing: true}
	st.clearWrapCache()
}

// AppendSearchRune appends r to the query and recomputes matches.
func (st *State) AppendSearchRune(r rune) {
	st.Search.Query += string(r)
	st.RecomputeSearch()
}

// BackspaceSearch removes the last rune of the query and recomputes matches.
func (st *State) BackspaceSearch() {
	q := []rune(st.Search.Query)
	if len(q) == 0 {
		return
	}
	st.Search.Query = string(q[:len(q)-1])
	st.RecomputeSearch()
}

// RecomputeSearch re-runs FindMatches against the current query and selects the first
// match in document order, so typing highlights and reveals matches incrementally.
func (st *State) RecomputeSearch() {
	st.Search.Matches = st.FindMatches(st.Search.Query)
	st.Search.Current = -1
	if len(st.Search.Matches) > 0 {
		st.Search.Current = 0
	}
	st.clearWrapCache()
}

// AcceptSearch stops capturing keystrokes while keeping highlights visible (if any matches).
func (st *State) AcceptSearch() {
	st.Search.Editing = false
	st.Search.Active = len(st.Search.Matches) > 0
	st.clearWrapCache()
}

// CancelSearch clears search state and any highlighting.
func (st *State) CancelSearch() {
	st.Search = SearchState{Current: -1}
	st.clearWrapCache()
}
