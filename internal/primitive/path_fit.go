package primitive

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FitPathForWidth formats path for display within maxRunes terminal cells.
// The basename (final path segment) is kept intact when possible; earlier
// directories are shortened progressively (full name → first rune + ellipsis →
// single rune). Ellipsis uses Unicode …. When the path cannot be shortened
// further, the full string is clipped with truncateMiddleEllipsis.
func FitPathForWidth(path string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(path))
	// Clean strips a trailing slash from "~/", which must stay "~/" for display.
	if path == "~" {
		path = "~/"
	}
	if path == "" || path == "." {
		return truncateMiddleEllipsis(path, maxRunes)
	}

	prefix, segs := parseDisplayPath(path)
	switch {
	case prefix == "/" && len(segs) == 0:
		return truncateMiddleEllipsis("/", maxRunes)
	case prefix == "~/" && len(segs) == 0:
		t := "~/"
		if utf8.RuneCountInString(t) <= maxRunes {
			return t
		}
		return truncateMiddleEllipsis(t, maxRunes)
	}

	if len(segs) == 0 {
		return truncateMiddleEllipsis(path, maxRunes)
	}

	base := segs[len(segs)-1]
	dirs := segs[:len(segs)-1]

	full := joinPathParts(prefix, dirs, base)
	if utf8.RuneCountInString(full) <= maxRunes {
		return full
	}

	if utf8.RuneCountInString(base) > maxRunes {
		return truncateMiddleEllipsis(full, maxRunes)
	}

	repr := append([]string(nil), dirs...)
	for {
		candidate := joinPathParts(prefix, repr, base)
		if utf8.RuneCountInString(candidate) <= maxRunes {
			return candidate
		}
		bestI := -1
		bestLen := -1
		for i, s := range repr {
			if _, ok := nextShorterSegmentForm(s); !ok {
				continue
			}
			l := utf8.RuneCountInString(s)
			if l > bestLen || (l == bestLen && (bestI < 0 || i < bestI)) {
				bestLen = l
				bestI = i
			}
		}
		if bestI < 0 {
			break
		}
		next, _ := nextShorterSegmentForm(repr[bestI])
		repr[bestI] = next
	}

	return truncateMiddleEllipsis(joinPathParts(prefix, repr, base), maxRunes)
}

func parseDisplayPath(path string) (prefix string, segs []string) {
	switch {
	case strings.HasPrefix(path, "~/"):
		rest := strings.TrimPrefix(path, "~/")
		if rest == "" {
			return "~/", nil
		}
		return "~/", strings.Split(rest, "/")
	case strings.HasPrefix(path, "/"):
		if path == "/" {
			return "/", nil
		}
		return "/", strings.Split(strings.TrimPrefix(path, "/"), "/")
	default:
		return "", strings.Split(path, "/")
	}
}

func joinPathParts(prefix string, dirs []string, base string) string {
	parts := append(append([]string(nil), dirs...), base)
	body := strings.Join(parts, "/")
	switch prefix {
	case "":
		return body
	case "~/":
		if body == "" {
			return "~/"
		}
		return "~/" + body
	case "/":
		if body == "" {
			return "/"
		}
		return "/" + body
	default:
		return body
	}
}

func nextShorterSegmentForm(seg string) (string, bool) {
	rr := []rune(seg)
	n := len(rr)
	if n <= 1 {
		return "", false
	}
	if n == 2 && rr[1] == Ellipsis {
		return string(rr[0]), true
	}
	if n == 2 {
		return string(rr[0]), true
	}
	return string(rr[0]) + string(Ellipsis), true
}

func truncateMiddleEllipsis(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 3 {
		return string(runes[:width])
	}
	prefixLen := (width - 1) / 2
	suffixLen := width - prefixLen - 1
	return string(runes[:prefixLen]) + string(Ellipsis) + string(runes[len(runes)-suffixLen:])
}
