package keymap

// DefaultBookmarkDialogOverlayKeys holds chords that apply only while the
// bookmarks path picker (navigate purpose) is open.
func DefaultBookmarkDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionBookmarkDelete:    {"F8"},
		ActionBookmarkOpenOther: {"M-Enter", "S-Enter"},
	}
}

// AllowedInBookmarkDialogOverlay reports whether actionID may appear under
// [dialog.bookmark].
func AllowedInBookmarkDialogOverlay(actionID string) bool {
	_, ok := DefaultBookmarkDialogOverlayKeys()[actionID]
	return ok
}
