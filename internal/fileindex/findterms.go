package fileindex

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/paranoidi/fileindex/db"
)

// likeTerm is one name test of a Find query: a LIKE pattern on the lowered name (ESCAPE '\'),
// negated or not, and the literal runs of the pattern (lowered, split at wildcards), which the
// trigram index can look up.
type likeTerm struct {
	pattern string
	runs    []string
	not     bool
}

// termGroups is a Find query's name tests: every group must hold (AND), a group holds when any
// of its terms does (OR).
type termGroups [][]likeTerm

// seg is one character of a query token; lit marks one that is quoted or escaped, so it is never
// an operator or a wildcard.
type seg struct {
	r   rune
	lit bool
}

// parseTerms reads the Terms grammar: whitespace-separated terms, "a | b" ORs the terms on either
// side (binding tighter than the space), a leading "!" negates, "^" and "$" anchor the start and
// end of the name, a term with "*" or "?" must match the whole name, double quotes (an unclosed
// one runs to the end) and "\ " make text literal, and a leading "'" is dropped. Nothing is a
// syntax error: stray operators are ignored.
func parseTerms(text string) termGroups {
	var groups termGroups
	orNext := false
	for _, tok := range tokenizeTerms(text) {
		if len(tok) == 1 && !tok[0].lit && tok[0].r == '|' {
			orNext = true
			continue
		}
		t, ok := termOf(tok)
		if !ok {
			continue
		}
		if orNext && len(groups) > 0 {
			groups[len(groups)-1] = append(groups[len(groups)-1], t)
		} else {
			groups = append(groups, []likeTerm{t})
		}
		orNext = false
	}
	return groups
}

// patternTerms is Pattern mode's single term: spaces and operators are plain text, a text with no
// "*" or "?" matches as a substring.
func patternTerms(text string) termGroups {
	segs := make([]seg, 0, len(text))
	for _, r := range text {
		segs = append(segs, seg{r: r})
	}
	t, ok := compileTerm(segs, false, false, false)
	if !ok {
		return nil
	}
	return termGroups{{t}}
}

func tokenizeTerms(s string) [][]seg {
	var toks [][]seg
	var cur []seg
	flush := func() {
		if len(cur) > 0 {
			toks = append(toks, cur)
			cur = nil
		}
	}
	quoted := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; {
		case quoted && r == '"':
			quoted = false
		case quoted:
			cur = append(cur, seg{r, true})
		case r == '"':
			quoted = true
		case r == '\\' && i+1 < len(rs) && rs[i+1] == ' ':
			cur = append(cur, seg{' ', true})
			i++
		case unicode.IsSpace(r):
			flush()
		default:
			cur = append(cur, seg{r: r})
		}
	}
	flush()
	return toks
}

// termOf strips a token's operators and compiles what is left.
func termOf(tok []seg) (likeTerm, bool) {
	is := func(i int, r rune) bool { return i >= 0 && i < len(tok) && !tok[i].lit && tok[i].r == r }
	not := is(0, '!')
	if not {
		tok = tok[1:]
	}
	if is(0, '\'') {
		tok = tok[1:]
	}
	start := is(0, '^')
	if start {
		tok = tok[1:]
	}
	end := is(len(tok)-1, '$')
	if end {
		tok = tok[:len(tok)-1]
	}
	return compileTerm(tok, start, end, not)
}

// compileTerm builds the LIKE pattern: text is lowered like the stored name_lower columns, % _ \
// are escaped, "*" and "?" become wildcards. Without wildcards the name only has to contain the
// text, start with it (start) or end with it (end); with wildcards the pattern is the whole name.
func compileTerm(segs []seg, start, end, not bool) (likeTerm, bool) {
	if len(segs) == 0 {
		return likeTerm{}, false
	}
	var pat, run strings.Builder
	var runs []string
	endRun := func() {
		if run.Len() > 0 {
			runs = append(runs, run.String())
			run.Reset()
		}
	}
	wild := false
	for _, s := range segs {
		if !s.lit && (s.r == '*' || s.r == '?') {
			wild = true
			if s.r == '*' {
				pat.WriteByte('%')
			} else {
				pat.WriteByte('_')
			}
			endRun()
			continue
		}
		if strings.ContainsRune(`\%_`, s.r) {
			pat.WriteByte('\\')
		}
		l := db.Lower(string(s.r))
		pat.WriteString(l)
		run.WriteString(l)
	}
	endRun()
	p := pat.String()
	if !wild {
		if !start {
			p = "%" + p
		}
		if !end {
			p += "%"
		}
	}
	return likeTerm{pattern: p, runs: runs, not: not}, true
}

// where is the SQL condition on column col (a lowered name) for all groups; arg binds a value and
// returns its placeholder. "" when there are no groups.
func (g termGroups) where(col string, arg func(any) string) string {
	ands := make([]string, 0, len(g))
	for _, alts := range g {
		ors := make([]string, 0, len(alts))
		for _, t := range alts {
			op := "LIKE"
			if t.not {
				op = "NOT LIKE"
			}
			ors = append(ors, col+" "+op+" "+arg(t.pattern)+` ESCAPE '\'`)
		}
		ands = append(ands, "("+strings.Join(ors, " OR ")+")")
	}
	return strings.Join(ands, " AND ")
}

// trigramMinRun is the shortest literal run the trigram index can look up.
const trigramMinRun = 3

// trigram is an instance_trigram MATCH expression that every name satisfying the groups also
// satisfies (ok false when no group can give one). The index has detail=none, so it holds no
// phrases: the expression is quoted 3-character tokens, ANDed within an alternative, the
// alternatives ORed. It is a pre-filter; the LIKE conditions do the exact check. The group used
// is the one whose alternatives all have the longest literal run.
func (g termGroups) trigram() (match string, ok bool) {
	best, bestScore := -1, trigramMinRun-1
	for gi, alts := range g {
		score := int(^uint(0) >> 1)
		for _, t := range alts {
			if t.not {
				score = 0
				break
			}
			score = min(score, longestRun(t.runs))
		}
		if score > bestScore {
			best, bestScore = gi, score
		}
	}
	if best < 0 {
		return "", false
	}
	var ors []string
	for _, t := range g[best] {
		seen := map[string]bool{}
		var ands []string
		for _, run := range t.runs {
			rs := []rune(run)
			for i := 0; i+trigramMinRun <= len(rs); i++ {
				tri := string(rs[i : i+trigramMinRun])
				if !seen[tri] {
					seen[tri] = true
					ands = append(ands, `"`+strings.ReplaceAll(tri, `"`, `""`)+`"`)
				}
			}
		}
		ors = append(ors, "("+strings.Join(ands, " AND ")+")")
	}
	return strings.Join(ors, " OR "), true
}

func longestRun(runs []string) int {
	n := 0
	for _, r := range runs {
		n = max(n, utf8.RuneCountInString(r))
	}
	return n
}
