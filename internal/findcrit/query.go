package findcrit

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
)

// Mode is how the find query text is matched against an entry's path relative to the root.
type Mode int

const (
	// ModeFuzzy is the fzf subset of internal/search, ranked by score.
	ModeFuzzy Mode = iota
	// ModeGlob is a shell pattern (Glob), in walk order.
	ModeGlob
	// ModeRegex is a Go regular expression on the relative path, in walk order.
	ModeRegex
	modeCount
)

// ModeLabels are the match mode radio labels, indexed by Mode.
var ModeLabels = [modeCount]string{"Fuzzy", "Glob", "Regex"}

// Next is the mode after m (wrapping), for the cycle action.
func (m Mode) Next() Mode { return (m + 1) % modeCount }

// Tag is the short name shown in the query row and the criteria summary ("" for fuzzy).
func (m Mode) Tag() string {
	switch m {
	case ModeGlob:
		return "glob"
	case ModeRegex:
		return "regex"
	}
	return ""
}

// QueryEmpty reports whether query matches everything in mode m.
func QueryEmpty(query string, m Mode) bool {
	if m == ModeFuzzy {
		return search.Parse(query).Empty()
	}
	return strings.TrimSpace(query) == ""
}

// Matcher matches a compiled query against relative paths ('/'-separated). It is safe for
// concurrent use.
type Matcher struct {
	mode         Mode
	empty        bool
	fuzzy        search.Query
	fuzzyOpts    search.Options
	glob         panel.GroupMatcher
	globFullPath bool
	rx           *regexp.Regexp
}

// Compile builds the matcher for query in mode m. caseInsensitive applies to every mode. An
// empty query matches everything; an invalid glob or regexp returns an error.
func Compile(query string, m Mode, caseInsensitive bool) (*Matcher, error) {
	mt := &Matcher{mode: m, empty: QueryEmpty(query, m)}
	if mt.empty {
		return mt, nil
	}
	switch m {
	case ModeFuzzy:
		mt.fuzzy = search.Parse(query)
		mt.fuzzyOpts = search.Options{CaseInsensitive: caseInsensitive}
	case ModeGlob:
		g := strings.TrimSpace(query)
		if !strings.ContainsAny(g, "*?[") {
			g = "*" + g + "*" // a plain word finds names containing it (as Find in index's Pattern)
		}
		// With a '/' the pattern matches the whole relative path, where '*' crosses directories.
		mt.globFullPath = strings.Contains(g, "/")
		var err error
		if mt.globFullPath {
			mt.glob, err = panel.NewGroupMatcherFullPath(g, panel.GroupPatternShell, !caseInsensitive)
		} else {
			mt.glob, err = panel.NewGroupMatcher(g, panel.GroupPatternShell, !caseInsensitive)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid glob: %w", err)
		}
	case ModeRegex:
		expr := strings.TrimSpace(query)
		if caseInsensitive {
			expr = "(?i)" + expr
		}
		rx, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp: %w", err)
		}
		mt.rx = rx
	default:
		return nil, fmt.Errorf("unknown match mode %d", m)
	}
	return mt, nil
}

// Ranked reports whether results are ordered by score (fuzzy) rather than walk order.
func (mt *Matcher) Ranked() bool { return mt.mode == ModeFuzzy && !mt.empty }

// Empty reports whether the matcher matches everything.
func (mt *Matcher) Empty() bool { return mt.empty }

// Match matches rel, a path relative to the find root. Ranges are rune offsets into rel.
func (mt *Matcher) Match(rel string) search.Result {
	if mt.empty {
		return search.Result{Matched: true}
	}
	switch mt.mode {
	case ModeFuzzy:
		return mt.fuzzy.Match(rel, mt.fuzzyOpts)
	case ModeGlob:
		if mt.globFullPath {
			// The whole row matched: nothing to single out.
			return search.Result{Matched: mt.glob.Match(rel)}
		}
		start := strings.LastIndexByte(rel, '/') + 1
		if !mt.glob.Match(rel[start:]) {
			return search.Result{}
		}
		from := utf8.RuneCountInString(rel[:start])
		return search.Result{Matched: true, Ranges: []search.Range{{Start: from, End: from + utf8.RuneCountInString(rel[start:])}}}
	case ModeRegex:
		locs := mt.rx.FindAllStringIndex(rel, -1)
		if locs == nil {
			return search.Result{}
		}
		ranges := make([]search.Range, 0, len(locs))
		for _, l := range locs {
			if l[0] == l[1] {
				continue // empty matches (^, a*) highlight nothing
			}
			from := utf8.RuneCountInString(rel[:l[0]])
			ranges = append(ranges, search.Range{Start: from, End: from + utf8.RuneCountInString(rel[l[0]:l[1]])})
		}
		return search.Result{Matched: true, Ranges: ranges}
	}
	return search.Result{}
}
