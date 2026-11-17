package keymap

// DefaultConfigDialogOverlayKeys holds chords that apply only while the Configuration dialog is open.
func DefaultConfigDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionEditConfig: {"F4"},
	}
}

// AllowedInConfigDialogOverlay reports whether actionID may appear under [dialog.config].
func AllowedInConfigDialogOverlay(actionID string) bool {
	_, ok := DefaultConfigDialogOverlayKeys()[actionID]
	return ok
}
