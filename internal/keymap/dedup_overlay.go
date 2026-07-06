package keymap

import "strings"

// DefaultDedupOverlayKeys holds built-in chords for the find-duplicates view ([dedup]).
func DefaultDedupOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionDedupClose:          {"esc", "left"},
		ActionDedupToggleSort:     {"C-s"}, // match the file-list sort shortcut (panel.sort-dialog)
		ActionDedupToggleEmpty:    {"M-e"},
		ActionDedupMarkRedundant:  {"C-k"}, // "keep uniques"
		ActionDedupMarkDuplicates: {"C-d"}, // "delete duplicates from here"
		ActionDedupMarkGroup:      {"C-g"}, // "group" — toggle-mark every copy in the group
	}
}

// AllowedInDedupOverlay reports whether actionID may appear under [dedup].
func AllowedInDedupOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "dedup.")
}
