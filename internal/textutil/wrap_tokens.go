package textutil

import (
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/paranoidi/paras-commander/internal/primitive"
)

// WrapTokens joins tokens with single spaces into lines at most width cells wide, breaking only
// between tokens; a token wider than width is hard-broken into width-sized pieces (its last piece may share a
// line with the next token). width below 1 returns the tokens joined on one line.
func WrapTokens(tokens []string, width int) []string {
	if width < 1 {
		return []string{strings.Join(tokens, " ")}
	}
	var lines []string
	cur, curW := "", 0
	for _, tok := range tokens {
		w := runewidth.StringWidth(tok)
		if cur != "" && curW+1+w <= width {
			cur, curW = cur+" "+tok, curW+1+w
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		for w > width {
			head := runewidth.Truncate(tok, width, "")
			if head == "" { // a cell wider than the row: take it whole rather than loop
				head = string([]rune(tok)[:1])
			}
			lines = append(lines, head)
			tok = tok[len(head):]
			w = runewidth.StringWidth(tok)
		}
		cur, curW = tok, w
	}
	if cur != "" || len(lines) == 0 {
		lines = append(lines, cur)
	}
	return lines
}

// ClampLines returns lines cut to at most n rows (n below 1 counts as 1). When rows were dropped
// the last kept row ends with an ellipsis, shortened to fit width.
func ClampLines(lines []string, n, width int) []string {
	n = max(n, 1)
	if len(lines) <= n {
		return lines
	}
	out := append([]string(nil), lines[:n]...)
	out[n-1] = runewidth.Truncate(out[n-1]+string(primitive.Ellipsis), width, string(primitive.Ellipsis))
	return out
}
