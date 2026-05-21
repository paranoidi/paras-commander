package gitstatus

import (
	"path/filepath"
	"strings"
)

// entry is one path's porcelain-derived status before aggregation.
type entry struct {
	path     string // absolute, cleaned
	staged   Status
	unstaged Status
}

func parsePorcelain(stdout string, workRoot string) []entry {
	workRoot = filepath.Clean(workRoot)
	var out []entry
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if len(line) < 3 {
			continue
		}
		x, y := rune(line[0]), rune(line[1])
		if line[2] != ' ' {
			continue
		}
		rawPath := strings.TrimSpace(line[3:])
		rawPath = unquotePorcelainPath(rawPath)
		if rawPath == "" {
			continue
		}
		abs := filepath.Clean(filepath.Join(workRoot, filepath.FromSlash(rawPath)))
		var staged, unstaged Status
		if x == '?' && y == '?' {
			staged = NotModified
			unstaged = New
		} else {
			staged = mapIndexRune(x)
			unstaged = mapWorkTreeRune(y)
		}
		out = append(out, entry{path: abs, staged: staged, unstaged: unstaged})
	}
	return out
}

func unquotePorcelainPath(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		// Git quotes unusual paths; minimal unescape for common cases.
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, "\\n", "\n")
		inner = strings.ReplaceAll(inner, "\\t", "\t")
		inner = strings.ReplaceAll(inner, "\\\\", "\\")
		inner = strings.ReplaceAll(inner, "\\\"", "\"")
		return inner
	}
	return s
}
