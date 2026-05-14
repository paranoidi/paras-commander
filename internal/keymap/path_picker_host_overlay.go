package keymap

// DefaultPathPickerHostOverlayKeys returns the built-in [path_picker_host_action_keys]
// defaults. The fuzzy path picker on copy/move destination rows and symlink/hardlink
// path fields is opened with the same chords as bookmark.open (see App.tryPathPickerHostShortcut);
// this overlay map stays empty so there is no second source of truth for that shortcut.
func DefaultPathPickerHostOverlayKeys() map[string][]string {
	return map[string][]string{}
}

// AllowedInPathPickerHostOverlay reports whether actionID may appear under
// [path_picker_host_action_keys]. Entries are not supported; the table must be absent
// or empty (see validatePathPickerHostOverlayKeys).
func AllowedInPathPickerHostOverlay(actionID string) bool {
	_ = actionID
	return false
}
