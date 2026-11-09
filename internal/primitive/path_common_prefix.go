package primitive

import "strings"

// StripCommonPathPrefix returns left and right with the longest shared leading path
// segments removed. Inputs use slash-separated display paths (as from PathWithHomeTilde).
// When either side is empty, or prefixes differ (e.g. ~/ vs /), inputs are unchanged.
func StripCommonPathPrefix(left, right string) (string, string) {
	if left == "" || right == "" {
		return left, right
	}
	leftPrefix, leftSegs := splitDisplayPath(left)
	rightPrefix, rightSegs := splitDisplayPath(right)
	if leftPrefix != rightPrefix {
		return left, right
	}
	n := 0
	for n < len(leftSegs) && n < len(rightSegs) && leftSegs[n] == rightSegs[n] {
		n++
	}
	if n == 0 {
		return left, right
	}
	return joinDisplayPathSuffix(leftSegs[n:]), joinDisplayPathSuffix(rightSegs[n:])
}

func splitDisplayPath(s string) (prefix string, segs []string) {
	switch {
	case strings.HasPrefix(s, "~/"):
		rest := s[2:]
		if rest == "" {
			return "~/", nil
		}
		return "~/", strings.Split(rest, "/")
	case strings.HasPrefix(s, "/"):
		rest := strings.TrimPrefix(s, "/")
		if rest == "" {
			return "/", nil
		}
		return "/", strings.Split(rest, "/")
	case s == "":
		return "", nil
	default:
		return "", strings.Split(s, "/")
	}
}

func joinDisplayPathSuffix(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	return strings.Join(segs, "/")
}
