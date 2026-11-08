package keymap

// DefaultColumnsDialogOverlayKeys holds chords that apply only while the Columns dialog is open.
func DefaultColumnsDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionColumnsSaveProfile:    {"F5"},
		ActionColumnsLoadProfile:    {"F2"},
		ActionColumnsDeleteProfile:  {"F8"},
		ActionColumnsDefaultProfile: {"F6"},
	}
}

// AllowedInColumnsDialogOverlay reports whether actionID may appear under
// [dialog.columns].
func AllowedInColumnsDialogOverlay(actionID string) bool {
	_, ok := DefaultColumnsDialogOverlayKeys()[actionID]
	return ok
}
