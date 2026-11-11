package keymap

// DefaultThemeDialogOverlayKeys holds chords that apply only while the theme picker is open.
func DefaultThemeDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionThemeToggleAutoReload: {"F5"},
		ActionThemeNextPreview:      {"F6"},
		ActionThemePrevPreview:      {"S-F6"},
	}
}

// AllowedInThemeDialogOverlay reports whether actionID may appear under [dialog.theme].
func AllowedInThemeDialogOverlay(actionID string) bool {
	_, ok := DefaultThemeDialogOverlayKeys()[actionID]
	return ok
}
