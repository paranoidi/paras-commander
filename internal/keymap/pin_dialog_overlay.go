package keymap

// DefaultPinDialogOverlayKeys holds chords that apply only while the Pin dialog is open.
func DefaultPinDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPinView:            {"F3"},
		ActionPinOpenInPrimary:   {"S-left"},
		ActionPinOpenInSecondary: {"S-right"},
		ActionPinRemove:          {"F8"},
		ActionPinRemoveAll:       {"S-F8"},
	}
}

// AllowedInPinDialogOverlay reports whether actionID may appear under [dialog.pin].
func AllowedInPinDialogOverlay(actionID string) bool {
	_, ok := DefaultPinDialogOverlayKeys()[actionID]
	return ok
}
