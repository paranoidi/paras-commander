package keymap

import "strings"

// DefaultCopiesOverlayKeys holds built-in chords for the Other copies view ([copies]).
func DefaultCopiesOverlayKeys() map[string][]string {
	return withOpenInPanelKeys(map[string][]string{
		ActionCopiesClose:    {"esc"},
		ActionCopiesPrevPane: {"S-tab"}, // shadows global panel.toggle-hide-inactive in-view
	})
}

// AllowedInCopiesOverlay reports whether actionID may appear under [copies].
func AllowedInCopiesOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "copies.") || isOpenInPanelAction(actionID)
}
