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

// WrapTextLines splits on newlines and wraps each logical line to at most maxCols runes.
// Explicit newlines are preserved; long lines break at spaces when practical, otherwise hard-break.
func WrapTextLines(text string, maxCols int) []string {
	if maxCols < 1 {
		maxCols = 1
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		lines = append(lines, wrapTextLine(line, maxCols)...)
	}
	return lines
}

func wrapTextLine(line string, maxCols int) []string {
	if utf8.RuneCountInString(line) <= maxCols {
		return []string{line}
	}
	var out []string
	runes := []rune(line)
	for len(runes) > 0 {
		if len(runes) <= maxCols {
			out = append(out, string(runes))
			break
		}
		breakAt := maxCols
		minBreak := max(maxCols/3, 1)
		for i := maxCols - 1; i >= minBreak; i-- {
			if runes[i] == ' ' {
				breakAt = i
				break
			}
		}
		out = append(out, string(runes[:breakAt]))
		runes = runes[breakAt:]
		for len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return out
}
