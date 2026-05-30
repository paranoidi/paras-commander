package keymap

// DefaultBookmarkDialogOverlayKeys holds chords that apply only while the
// bookmarks path picker (navigate purpose) is open.
func DefaultBookmarkDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionBookmarkDelete: {"F8"},
	}
}

// AllowedInBookmarkDialogOverlay reports whether actionID may appear under
// [bookmark_dialog_action_keys].
func AllowedInBookmarkDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionBookmarkDelete:
		return true
	default:
		return false
	}
}
