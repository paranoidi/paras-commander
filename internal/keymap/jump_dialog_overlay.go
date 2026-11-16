package keymap

// DefaultJumpDialogOverlayKeys holds chords that apply only while Jump to directory is open.
func DefaultJumpDialogOverlayKeys() map[string][]string {
	return withOpenInPanelKeys(map[string][]string{})
}

// AllowedInJumpDialogOverlay reports whether actionID may appear under [dialog.jump].
func AllowedInJumpDialogOverlay(actionID string) bool {
	_, ok := DefaultJumpDialogOverlayKeys()[actionID]
	return ok
}
