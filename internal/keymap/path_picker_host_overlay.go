package keymap

// DefaultPathPickerHostOverlayKeys holds chords that apply only while a dialog
// that hosts a path-picker row is focused (copy/move destination or symlink/hardlink fields).
// They are merged into a separate map from [action_keys] so the same chord as
// global shortcuts (e.g. F9 for app.open-menu) can be reused here.
func DefaultPathPickerHostOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionUIOpenPathPicker: {"F9"},
	}
}

// AllowedInPathPickerHostOverlay reports whether actionID may appear under
// [path_picker_host_action_keys].
func AllowedInPathPickerHostOverlay(actionID string) bool {
	return actionID == ActionUIOpenPathPicker
}
