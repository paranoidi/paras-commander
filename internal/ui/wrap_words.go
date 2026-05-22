package ui

import (
	"strings"
	"unicode/utf8"
)

// MessageLogWrapRunes is the fallback maximum line length when wrapping status/toast text
// for the Messages log on terminals too small for a layout (see MessageLogWrapColsForLayout).
const MessageLogWrapRunes = 80

// WrapWordsToWidth splits text into lines of at most maxCols runes, breaking at spaces between words.
// Words longer than maxCols are hard-broken into maxCols-sized segments. Newlines become spaces.
// Empty input yields nil.
func WrapWordsToWidth(text string, maxCols int) []string {
	if maxCols < 1 {
		return nil
	}
	text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", " "))
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	cur := ""

	runeLen := func(s string) int { return utf8.RuneCountInString(s) }

	flush := func() {
		if strings.TrimSpace(cur) != "" {
			lines = append(lines, strings.TrimSpace(cur))
		}
		cur = ""
	}

	for _, w := range words {
		wr := []rune(w)
		if len(wr) > maxCols {
			flush()
			for len(wr) > 0 {
				take := maxCols
				if take > len(wr) {
					take = len(wr)
				}
				lines = append(lines, string(wr[:take]))
				wr = wr[take:]
			}
			continue
		}
		if cur == "" {
			cur = w
			continue
		}
		if runeLen(cur)+1+len(wr) > maxCols {
			flush()
			cur = w
		} else {
			cur = cur + " " + w
		}
	}
	flush()
	return lines
}
