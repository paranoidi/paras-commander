package keymap

// DefaultUserMenuOverlayKeys holds chords that apply only while the F2 user menu strip is open.
func DefaultUserMenuOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionUserMenuBack: {"Left", "Backspace"},
	}
}

// AllowedInUserMenuOverlay reports whether actionID may appear under [dialog.user_menu].
func AllowedInUserMenuOverlay(actionID string) bool {
	return actionID == ActionUserMenuBack
}
