package keymap

import "strings"

// DefaultCompareOverlayKeys holds built-in chords for the Compare full-screen view ([compare]).
func DefaultCompareOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionCompareClose:       {"esc"},
		ActionCompareCycleFilter: {"F3"},
		ActionCompareResetFilter: {"S-F3"},
		ActionCompareRefresh:     {"M-r"},
		ActionCompareMerge:       {"F5"},
	}
}

// AllowedInCompareOverlay reports whether actionID may appear under [compare].
func AllowedInCompareOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "compare.")
}
