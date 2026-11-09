package keymap

import "strings"

// DefaultUniqueOverlayKeys holds built-in chords for the Find unique files view ([unique]).
func DefaultUniqueOverlayKeys() map[string][]string {
	return withOpenInPanelKeys(map[string][]string{
		ActionUniqueClose:          {"esc"},
		ActionUniqueToggleNode:     {"right", "C-right"},
		ActionUniqueCollapse:       {"left", "C-left"}, // collapse node, or jump to parent
		ActionUniqueCollapseAll:    {"M-S-left"},
		ActionUniqueExpandAll:      {"M-S-right"},
		ActionUniquePrevDir:        {"M-up"},
		ActionUniqueNextDir:        {"M-down"},
		ActionUniquePrevPane:       {"S-tab"}, // shadows global panel.toggle-hide-inactive in-view
		ActionUniqueHashUnverified: {"M-H"},
	})
}

// AllowedInUniqueOverlay reports whether actionID may appear under [unique].
func AllowedInUniqueOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "unique.") || isOpenInPanelAction(actionID)
}
