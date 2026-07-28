package search

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Range identifies a half-open rune range in the matched value.
type Range struct {
	Start int
	End   int
}

// Options controls query matching.
type Options struct {
	CaseInsensitive bool
}

// Query is a parsed subset-fzf query.
type Query struct {
	terms []term
}

type termKind int

const (
	termFuzzy termKind = iota
	termPrefix
	termSuffix
	termExact
	termExactBoundary
	termEqual
)

type term struct {
	Kind    termKind
	Text    string
	Inverse bool
}

// Result describes the result of matching a query against one value.
type Result struct {
	Matched bool
	Score   int
	Ranges  []Range
}

// RankedResult records a matching item and its score metadata.
type RankedResult struct {
	Index  int
	Result Result
}

// Parse converts a whitespace-separated subset-fzf query into match terms.
func Parse(value string) Query {
	fields := strings.Fields(value)
	terms := make([]term, 0, len(fields))
	for _, field := range fields {
		parsed, ok := parseTerm(field)
		if ok {
			terms = append(terms, parsed)
		}
	}
	return Query{terms: terms}
}

// Empty reports whether the query has no effective terms.
func (q Query) Empty() bool {
	return len(q.terms) == 0
}

// Match reports whether value satisfies all query terms.
func (q Query) Match(value string, opts Options) Result {
	if len(q.terms) == 0 {
		return Result{Matched: true}
	}

	totalScore := 0
	var ranges []Range
	for _, term := range q.terms {
		result := matchTerm(term, value, opts)
		if term.Inverse {
			if result.Matched {
				return Result{}
			}
			continue
		}
		if !result.Matched {
			return Result{}
		}
		totalScore += result.Score
		ranges = append(ranges, result.Ranges...)
	}

	return Result{
		Matched: true,
		Score:   totalScore,
		Ranges:  mergeRanges(ranges),
	}
}

// Rank filters values by query and orders matches by descending score.
func (q Query) Rank(values []string, opts Options) []RankedResult {
	results, _ := q.RankCancellable(values, opts, nil)
	return results
}

// RankCancellable is like Rank but calls shouldCancel() every ~10 000 items.
// If shouldCancel returns true the computation is abandoned and (nil, true) is returned.
// Pass nil for shouldCancel to disable cancellation (equivalent to Rank).
func (q Query) RankCancellable(values []string, opts Options, shouldCancel func() bool) ([]RankedResult, bool) {
	if len(q.terms) == 0 {
		results := make([]RankedResult, len(values))
		for i := range values {
			results[i] = RankedResult{Index: i, Result: Result{Matched: true}}
		}
		return results, false
	}

	results := make([]RankedResult, 0, len(values))
	for i, value := range values {
		if shouldCancel != nil && i%10000 == 0 && i > 0 && shouldCancel() {
			return nil, true
		}
		result := q.Match(value, opts)
		if result.Matched {
			results = append(results, RankedResult{Index: i, Result: result})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Result.Score > results[j].Result.Score
	})
	return results, false
}

// parseTerm follows fzf extended-search token order: ! then $, then '…'/', then ^.
func parseTerm(field string) (term, bool) {
	kind := termFuzzy
	inverse := false
	if strings.HasPrefix(field, "!") {
		inverse = true
		kind = termExact // fzf: negation defaults to exact
		field = field[1:]
	}
	if field == "" {
		return term{}, false
	}

	if field != "$" && strings.HasSuffix(field, "$") {
		kind = termSuffix
		field = field[:len(field)-1]
	}

	switch {
	case len(field) > 2 && strings.HasPrefix(field, "'") && strings.HasSuffix(field, "'"):
		kind = termExactBoundary
		field = field[1 : len(field)-1]
	case strings.HasPrefix(field, "'"):
		kind = termExact // overwrites suffix — fzf: '.git$' ≡ '.git'
		field = field[1:]
	case strings.HasPrefix(field, "^"):
		if kind == termSuffix {
			kind = termEqual
		} else {
			kind = termPrefix
		}
		field = field[1:]
	}

	if field == "" {
		return term{}, false
	}
	return term{Kind: kind, Text: field, Inverse: inverse}, true
}

func matchTerm(term term, value string, opts Options) Result {
	switch term.Kind {
	case termPrefix:
		return matchPrefix(term.Text, value, opts)
	case termSuffix:
		return matchSuffix(term.Text, value, opts)
	case termExact:
		return matchExact(term.Text, value, opts)
	case termExactBoundary:
		return matchExactBoundary(term.Text, value, opts)
	case termEqual:
		return matchEqual(term.Text, value, opts)
	default:
		return matchFuzzy(term.Text, value, opts)
	}
}

func matchPrefix(needle, value string, opts Options) Result {
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) == 0 {
		return Result{Matched: true}
	}
	i := 0
	for _, r := range value {
		if i >= len(needleRunes) {
			break
		}
		if lowerRune(r, opts.CaseInsensitive) != needleRunes[i] {
			return Result{}
		}
		i++
	}
	if i < len(needleRunes) {
		return Result{}
	}
	return Result{
		Matched: true,
		Score:   900 + 30 + len(needleRunes)*4, // boundary at position 0 is always +30
		Ranges:  []Range{{Start: 0, End: len(needleRunes)}},
	}
}

func matchSuffix(needle, value string, opts Options) Result {
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) == 0 {
		return Result{Matched: true}
	}
	nRunes := utf8.RuneCountInString(value)
	start := nRunes - len(needleRunes)
	if start < 0 {
		return Result{}
	}
	i := 0
	runePos := 0
	for _, r := range value {
		if runePos >= start {
			if lowerRune(r, opts.CaseInsensitive) != needleRunes[i] {
				return Result{}
			}
			i++
		}
		runePos++
	}
	if i < len(needleRunes) {
		return Result{}
	}
	return Result{
		Matched: true,
		Score:   800 + len(needleRunes)*4 - start,
		Ranges:  []Range{{Start: start, End: nRunes}},
	}
}

func matchExact(needle, value string, opts Options) Result {
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) == 0 {
		return Result{Matched: true}
	}
	start, prevR, firstR := indexRunesInString(value, needleRunes, opts)
	if start < 0 {
		return Result{}
	}
	bonus := boundaryBonus(prevR, firstR, start == 0)
	return Result{
		Matched: true,
		Score:   700 + bonus + len(needleRunes)*5 - start*2,
		Ranges:  []Range{{Start: start, End: start + len(needleRunes)}},
	}
}

func matchEqual(needle, value string, opts Options) Result {
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) == 0 {
		return Result{Matched: true}
	}
	valueRunes := normalizeRunes(value, opts)
	if len(valueRunes) != len(needleRunes) {
		return Result{}
	}
	for i := range needleRunes {
		if valueRunes[i] != needleRunes[i] {
			return Result{}
		}
	}
	return Result{
		Matched: true,
		Score:   950 + len(needleRunes)*5,
		Ranges:  []Range{{Start: 0, End: len(needleRunes)}},
	}
}

// matchExactBoundary is fzf 'word' — exact substring with both ends on word boundaries.
// Underscore counts as a boundary (non-word), same as fzf.
func matchExactBoundary(needle, value string, opts Options) Result {
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) == 0 {
		return Result{Matched: true}
	}
	n := len(needleRunes)
	valueRunes := []rune(value)
	if len(valueRunes) < n {
		return Result{}
	}

	bestStart := -1
	bestScore := 0
	for start := 0; start <= len(valueRunes)-n; start++ {
		if !runesEqualAt(valueRunes, start, needleRunes, opts) {
			continue
		}
		end := start + n
		if start > 0 && isWordChar(valueRunes[start-1]) {
			continue
		}
		if end < len(valueRunes) && isWordChar(valueRunes[end]) {
			continue
		}
		score := 750 + n*5 - start*2
		// ponytail: underscore boundaries rank below whitespace/punct, like fzf
		if start > 0 && valueRunes[start-1] == '_' {
			score -= 20
		}
		if end < len(valueRunes) && valueRunes[end] == '_' {
			score -= 10
		}
		if bestStart < 0 || score > bestScore {
			bestStart = start
			bestScore = score
		}
	}
	if bestStart < 0 {
		return Result{}
	}
	return Result{
		Matched: true,
		Score:   bestScore,
		Ranges:  []Range{{Start: bestStart, End: bestStart + n}},
	}
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func runesEqualAt(value []rune, start int, needle []rune, opts Options) bool {
	for i, nr := range needle {
		if lowerRune(value[start+i], opts.CaseInsensitive) != nr {
			return false
		}
	}
	return true
}

func matchFuzzy(needle, value string, opts Options) Result {
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) == 0 {
		return Result{Matched: true}
	}

	// hit records one matched needle rune and its context for scoring.
	type hit struct {
		runeIdx  int
		prevRune rune // rune before this position (-1 if at start)
		currRune rune // rune at this position (original casing, for boundary detection)
	}
	hits := make([]hit, 0, len(needleRunes))

	ni := 0
	runePos := 0
	prevRune := rune(-1)
	for _, r := range value {
		if lowerRune(r, opts.CaseInsensitive) == needleRunes[ni] {
			hits = append(hits, hit{runePos, prevRune, r})
			ni++
			if ni >= len(needleRunes) {
				break
			}
		}
		prevRune = r
		runePos++
	}
	if ni < len(needleRunes) {
		return Result{}
	}
	if len(hits) == 0 {
		return Result{Matched: true}
	}

	first := hits[0].runeIdx
	span := hits[len(hits)-1].runeIdx - first + 1
	score := 1000 - first*5 - span*10 + len(hits)*10

	ranges := make([]Range, 0, len(hits))
	for i, h := range hits {
		score += boundaryBonus(h.prevRune, h.currRune, h.runeIdx == 0)
		if i > 0 && h.runeIdx == hits[i-1].runeIdx+1 {
			score += 15
		}
		ranges = append(ranges, Range{Start: h.runeIdx, End: h.runeIdx + 1})
	}
	return Result{Matched: true, Score: score, Ranges: mergeRanges(ranges)}
}

// lowerRune returns unicode.ToLower(r) when ci is true, otherwise r unchanged.
func lowerRune(r rune, ci bool) rune {
	if ci {
		return unicode.ToLower(r)
	}
	return r
}

// indexRunesInString returns the rune index of the first occurrence of needle in value
// (using opts for case comparison), the rune immediately before that position, and the
// rune at that position (both needed by boundaryBonus). Returns -1 if not found.
func indexRunesInString(value string, needle []rune, opts Options) (runeIdx int, prevR rune, firstR rune) {
	if len(needle) == 0 {
		return 0, -1, 0
	}
	prev := rune(-1)
	runePos := 0
	for bytePos, r := range value {
		if lowerRune(r, opts.CaseInsensitive) == needle[0] && matchesNeedle(value[bytePos:], needle, opts) {
			return runePos, prev, r
		}
		prev = r
		runePos++
	}
	return -1, -1, 0
}

// matchesNeedle reports whether value starts with needle (using opts for case comparison).
func matchesNeedle(value string, needle []rune, opts Options) bool {
	i := 0
	for _, r := range value {
		if i >= len(needle) {
			return true
		}
		if lowerRune(r, opts.CaseInsensitive) != needle[i] {
			return false
		}
		i++
	}
	return i >= len(needle)
}

// boundaryBonus returns a score bonus for a match character based on its context.
// prevR is the rune immediately before the match (-1 if the match is at the start).
// currR is the matched rune (original casing). atStart must be true when runeIdx == 0.
func boundaryBonus(prevR, currR rune, atStart bool) int {
	if atStart || prevR < 0 {
		return 30
	}
	if prevR == '-' || prevR == '_' || prevR == '.' || prevR == ' ' {
		return 30
	}
	if unicode.IsLower(prevR) && unicode.IsUpper(currR) {
		return 25
	}
	return 0
}

func mergeRanges(ranges []Range) []Range {
	if len(ranges) == 0 {
		return nil
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Start == ranges[j].Start {
			return ranges[i].End < ranges[j].End
		}
		return ranges[i].Start < ranges[j].Start
	})
	merged := []Range{ranges[0]}
	for _, current := range ranges[1:] {
		last := &merged[len(merged)-1]
		if current.Start <= last.End {
			if current.End > last.End {
				last.End = current.End
			}
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

// normalizeRunes converts needle to a rune slice, lowercased when opts.CaseInsensitive is true.
// This is called only on the query needle (short, typically 1–20 chars), so allocating once
// per term per Match call is acceptable.
func normalizeRunes(value string, opts Options) []rune {
	runes := []rune(value)
	if !opts.CaseInsensitive {
		return runes
	}
	for i, r := range runes {
		runes[i] = unicode.ToLower(r)
	}
	return runes
}
