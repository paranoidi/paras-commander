// Package textutil holds small pure string/path helpers shared across internal/app and
// internal/apphandler/* packages (status banners, log lines, absolute-path normalization).
package textutil

import (
	"path/filepath"
	"strings"
)

// BannerMaxRunes is the default max length (in runes) for a status banner line before
// TruncateBannerRunes ellipsizes it.
const BannerMaxRunes = 72

// AbsPathClean returns the cleaned absolute form of p, falling back to a cleaned relative
// path if the absolute form cannot be resolved.
func AbsPathClean(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(abs)
}

// TruncateBannerRunes trims s and, if longer than maxRunes, truncates it with a trailing "…".
func TruncateBannerRunes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

// FirstLine returns the first non-empty line of s (after trim), for errors that join
// multiple messages with newlines.
func FirstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, part := range strings.Split(s, "\n") {
		line := strings.TrimSpace(part)
		if line != "" {
			return line
		}
	}
	return ""
}
