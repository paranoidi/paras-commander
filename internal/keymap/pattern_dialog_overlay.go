package keymap

// DefaultPatternDialogOverlayKeys holds chords that apply only while the Filter or the Select /
// Unselect group dialog is open (saved patterns and the session history).
func DefaultPatternDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPatternLoad:    {"F2"},
		ActionPatternHistory: {"F3"},
		ActionPatternSave:    {"F5"},
		ActionPatternDelete:  {"F8"},
	}
}

// AllowedInPatternDialogOverlay reports whether actionID may appear under [dialog.pattern].
func AllowedInPatternDialogOverlay(actionID string) bool {
	_, ok := DefaultPatternDialogOverlayKeys()[actionID]
	return ok
}
