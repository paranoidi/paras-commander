package keymap

// DefaultHistoryDialogOverlayKeys holds chords that apply only while the history dialog is open.
func DefaultHistoryDialogOverlayKeys() map[string][]string {
	return withOpenInPanelKeys(map[string][]string{
		ActionPanelHistoryBothPanels: {"F5"},
	})
}

// AllowedInHistoryDialogOverlay reports whether actionID may appear under
// [dialog.history].
func AllowedInHistoryDialogOverlay(actionID string) bool {
	_, ok := DefaultHistoryDialogOverlayKeys()[actionID]
	return ok
}
