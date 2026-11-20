package keymap

// DefaultSelectedStripOverlayKeys holds built-in chords for the Selected strip ([selected]),
// consulted before [main] while a strip (the file panel's or the find-duplicates one) has focus.
func DefaultSelectedStripOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPanelSelectedToggleTree: {"M-t"}, // shadows global panel.cycle-column-profile in-strip
		// Open the row in the active / inactive panel (the app-wide S-left / S-right convention).
		ActionDestinationActivePanel:   {"S-left"},
		ActionDestinationInactivePanel: {"S-right"},
	}
}

// AllowedInSelectedStripOverlay reports whether actionID may appear under [selected].
func AllowedInSelectedStripOverlay(actionID string) bool {
	_, ok := DefaultSelectedStripOverlayKeys()[actionID]
	return ok
}
