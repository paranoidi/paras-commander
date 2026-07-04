package keymap

import "strings"

// DefaultDedupOverlayKeys holds built-in chords for the find-duplicates view ([dedup]).
func DefaultDedupOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionDedupClose:   {"esc", "left"},
		ActionDedupRefresh: {"M-r"},
	}
}

// AllowedInDedupOverlay reports whether actionID may appear under [dedup].
func AllowedInDedupOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "dedup.")
}
