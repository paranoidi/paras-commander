package search

import (
	"sort"
	"strings"
	"unicode"
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
	if len(q.terms) == 0 {
		results := make([]RankedResult, len(values))
		for i := range values {
			results[i] = RankedResult{Index: i, Result: Result{Matched: true}}
		}
		return results
	}

	results := make([]RankedResult, 0, len(values))
	for i, value := range values {
		result := q.Match(value, opts)
		if result.Matched {
			results = append(results, RankedResult{Index: i, Result: result})
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Result.Score > results[j].Result.Score
	})
	return results
}

func parseTerm(field string) (term, bool) {
	inverse := false
	if strings.HasPrefix(field, "!") {
		inverse = true
		field = strings.TrimPrefix(field, "!")
	}
	if field == "" {
		return term{}, false
	}

	parsed := term{Kind: termFuzzy, Text: field, Inverse: inverse}
	if strings.HasPrefix(field, "'") {
		parsed.Kind = termExact
		parsed.Text = strings.TrimPrefix(field, "'")
	} else if strings.HasPrefix(field, "^") {
		parsed.Kind = termPrefix
		parsed.Text = strings.TrimPrefix(field, "^")
	} else if strings.HasSuffix(field, "$") {
		parsed.Kind = termSuffix
		parsed.Text = strings.TrimSuffix(field, "$")
	}
	if parsed.Text == "" {
		return term{}, false
	}
	return parsed, true
}

func matchTerm(term term, value string, opts Options) Result {
	switch term.Kind {
	case termPrefix:
		return matchPrefix(term.Text, value, opts)
	case termSuffix:
		return matchSuffix(term.Text, value, opts)
	case termExact:
		return matchExact(term.Text, value, opts)
	default:
		return matchFuzzy(term.Text, value, opts)
	}
}

func matchPrefix(needle, value string, opts Options) Result {
	p := prepared(value, opts)
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) > len(p.match) || !sameRunes(p.match[:len(needleRunes)], needleRunes) {
		return Result{}
	}
	return Result{
		Matched: true,
		Score:   900 + boundaryBonus(p.original, 0) + len(needleRunes)*4,
		Ranges:  []Range{{Start: 0, End: len(needleRunes)}},
	}
}

func matchSuffix(needle, value string, opts Options) Result {
	p := prepared(value, opts)
	needleRunes := normalizeRunes(needle, opts)
	if len(needleRunes) > len(p.match) {
		return Result{}
	}
	start := len(p.match) - len(needleRunes)
	if !sameRunes(p.match[start:], needleRunes) {
		return Result{}
	}
	return Result{
		Matched: true,
		Score:   800 + len(needleRunes)*4 - start,
		Ranges:  []Range{{Start: start, End: len(p.match)}},
	}
}

func matchExact(needle, value string, opts Options) Result {
	p := prepared(value, opts)
	needleRunes := normalizeRunes(needle, opts)
	start := indexRunes(p.match, needleRunes)
	if start < 0 {
		return Result{}
	}
	return Result{
		Matched: true,
		Score:   700 + boundaryBonus(p.original, start) + len(needleRunes)*5 - start*2,
		Ranges:  []Range{{Start: start, End: start + len(needleRunes)}},
	}
}

func matchFuzzy(needle, value string, opts Options) Result {
	p := prepared(value, opts)
	needleRunes := normalizeRunes(needle, opts)
	positions := make([]int, 0, len(needleRunes))
	next := 0
	for _, needleRune := range needleRunes {
		found := -1
		for i := next; i < len(p.match); i++ {
			if p.match[i] == needleRune {
				found = i
				break
			}
		}
		if found < 0 {
			return Result{}
		}
		positions = append(positions, found)
		next = found + 1
	}
	if len(positions) == 0 {
		return Result{Matched: true}
	}

	first := positions[0]
	span := positions[len(positions)-1] - first + 1
	score := 1000 - first*10 - span*5 + len(positions)*10
	for i, position := range positions {
		score += boundaryBonus(p.original, position)
		if i > 0 && position == positions[i-1]+1 {
			score += 15
		}
	}

	ranges := make([]Range, 0, len(positions))
	for _, position := range positions {
		ranges = append(ranges, Range{Start: position, End: position + 1})
	}
	return Result{Matched: true, Score: score, Ranges: mergeRanges(ranges)}
}

type preparedValue struct {
	original []rune
	match    []rune
}

func prepared(value string, opts Options) preparedValue {
	original := []rune(value)
	if !opts.CaseInsensitive {
		return preparedValue{original: original, match: original}
	}
	match := make([]rune, len(original))
	for i, r := range original {
		match[i] = unicode.ToLower(r)
	}
	return preparedValue{original: original, match: match}
}

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

func sameRunes(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func indexRunes(value, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(value) {
		return -1
	}
	for i := 0; i <= len(value)-len(needle); i++ {
		if sameRunes(value[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func boundaryBonus(value []rune, position int) int {
	if position <= 0 || position >= len(value) {
		return 30
	}
	previous := value[position-1]
	current := value[position]
	if previous == '-' || previous == '_' || previous == '.' || previous == ' ' {
		return 30
	}
	if unicode.IsLower(previous) && unicode.IsUpper(current) {
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
