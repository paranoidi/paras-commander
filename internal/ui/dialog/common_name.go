package dialog

import (
	"strings"
	"unicode"
)

const minExtractedCommonNameRunes = 2

// ExtractLongestCommonName returns the longest continuous substring shared by all
// names, trimmed to a usable directory name. Requires at least two names.
func ExtractLongestCommonName(names []string) string {
	if len(names) < 2 {
		return ""
	}
	base := []rune(names[0])
	maxLen := 0
	var candidates []string
	for i := 0; i < len(base); i++ {
		for j := i + 1; j <= len(base); j++ {
			sub := string(base[i:j])
			if len([]rune(sub)) < maxLen {
				continue
			}
			if !allNamesContain(names[1:], sub) {
				continue
			}
			runeLen := len([]rune(sub))
			if runeLen > maxLen {
				maxLen = runeLen
				candidates = []string{sub}
				continue
			}
			if runeLen == maxLen {
				candidates = append(candidates, sub)
			}
		}
	}
	return bestNormalizedCommonName(names, candidates)
}

func bestNormalizedCommonName(names, candidates []string) string {
	best := ""
	bestLen := 0
	for _, raw := range candidates {
		normalized := normalizeExtractedCommonName(raw)
		if normalized == "" || isWeakCommonNameMatch(names, raw, normalized) {
			continue
		}
		n := len([]rune(normalized))
		if n > bestLen {
			bestLen = n
			best = normalized
		}
	}
	return best
}

func allNamesContain(names []string, sub string) bool {
	for _, name := range names {
		if !strings.Contains(name, sub) {
			return false
		}
	}
	return true
}

func isWeakCommonNameMatch(names []string, raw, normalized string) bool {
	if normalized == "" {
		return true
	}
	suffixOnly := true
	for _, name := range names {
		if !strings.HasSuffix(name, raw) {
			suffixOnly = false
			break
		}
	}
	if !suffixOnly {
		return false
	}
	// Reject extension-like suffix fragments (e.g. ".txt" shared by unrelated basenames).
	return strings.Contains(normalized, ".") || len([]rune(normalized)) <= 4
}

func normalizeExtractedCommonName(s string) string {
	s = strings.TrimSpace(s)
	for {
		prev := s
		s = strings.TrimFunc(s, isCommonNameSeparator)
		s = strings.TrimFunc(s, unicode.IsDigit)
		if s == prev {
			break
		}
	}
	if len([]rune(s)) < minExtractedCommonNameRunes {
		return ""
	}
	return s
}

func isCommonNameSeparator(r rune) bool {
	switch r {
	case '-', '_', '.', ' ', '\t', '[', ']', '(', ')':
		return true
	default:
		return unicode.IsSpace(r)
	}
}
