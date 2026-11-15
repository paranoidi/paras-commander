package keymap

// DefaultFilterDialogOverlayKeys holds chords that apply only while the Filter dialog is open
// (saved filter patterns and the session history).
func DefaultFilterDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFilterLoadPattern:   {"F2"},
		ActionFilterHistory:       {"F3"},
		ActionFilterSavePattern:   {"F5"},
		ActionFilterDeletePattern: {"F8"},
	}
}

// AllowedInFilterDialogOverlay reports whether actionID may appear under [dialog.filter].
func AllowedInFilterDialogOverlay(actionID string) bool {
	_, ok := DefaultFilterDialogOverlayKeys()[actionID]
	return ok
}
