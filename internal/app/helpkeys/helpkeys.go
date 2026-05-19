// Package helpkeys formats keymap bindings for the help dialog.
package helpkeys

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// JoinDisplay joins every binding into one readable string (comma-separated).
// When preferredKey is non-empty and appears in keys, it is listed first.
func JoinDisplay(keys []string, preferredKey string) string {
	if len(keys) == 0 {
		return ""
	}
	ordered := ForDisplay(keys, preferredKey)
	out := ""
	for i, k := range ordered {
		if i > 0 {
			out += ", "
		}
		out += HumanKey(k)
	}
	return out
}

// ForDisplay orders keys with preferredKey first when present.
func ForDisplay(keys []string, preferredKey string) []string {
	if preferredKey == "" {
		return append([]string(nil), keys...)
	}
	prefPresent := false
	for _, k := range keys {
		if k == preferredKey {
			prefPresent = true
			break
		}
	}
	if !prefPresent {
		return append([]string(nil), keys...)
	}
	out := make([]string, 0, len(keys))
	out = append(out, preferredKey)
	for _, k := range keys {
		if k == preferredKey {
			continue
		}
		out = append(out, k)
	}
	return out
}

// HumanKey converts a TOML key string to a human-readable form.
func HumanKey(s string) string {
	ch, err := keymap.ParseKey(s)
	if err != nil {
		return s
	}
	return keymap.FormatChord(ch)
}

// ConcatKeywords joins keyword slices for fuzzy help search.
func ConcatKeywords(kw []string) string {
	out := ""
	for _, k := range kw {
		out += " " + k
	}
	return out
}

// CanonicalRankText is the terminal-agnostic corpus for help filtering and rank scores.
func CanonicalRankText(ent ui.HelpEntry) string {
	s := strings.Join([]string{ent.Keys, ent.Section, ent.Title}, " ")
	if ent.FuzzyExtra != "" {
		s += " " + ent.FuzzyExtra
	}
	return s
}

// ActionRunnableInBrowser is false for jobs-only shortcuts that no-op outside the jobs view.
func ActionRunnableInBrowser(actionID string) bool {
	switch actionID {
	case keymap.ActionJobsCancel, keymap.ActionJobsPause, keymap.ActionJobsResume,
		keymap.ActionJobsQueueUp, keymap.ActionJobsQueueDown, keymap.ActionJobsClearFinished:
		return false
	default:
		return true
	}
}
