package keymap

// DefaultMkdirDialogOverlayKeys holds chords that apply only while the mkdir
// dialog (directory name field) is focused.
func DefaultMkdirDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFileMkdirExtractCommonName: {"F7"},
	}
}

// AllowedInMkdirDialogOverlay reports whether actionID may appear under
// [dialog.mkdir].
func AllowedInMkdirDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFileMkdirExtractCommonName:
		return true
	default:
		return false
	}
}
