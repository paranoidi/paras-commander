package keymap

// DefaultRenameDialogOverlayKeys holds chords that apply only while the main
// rename dialog (name field) is focused — not during sanitize/slugify sub-screens.
func DefaultRenameDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFileRenameOpenSanitize: {"F2"},
		ActionFileRenameOpenSlugify: {"F3"},
	}
}

// AllowedInRenameDialogOverlay reports whether actionID may appear under
// [rename_dialog_action_keys].
func AllowedInRenameDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFileRenameOpenSanitize, ActionFileRenameOpenSlugify:
		return true
	default:
		return false
	}
}
