package keymap

// DefaultFlattenDialogOverlayKeys holds chords that apply only while the flatten
// dialog destination row is focused.
func DefaultFlattenDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFlattenDestinationActive:   {"F5"},
		ActionFlattenDestinationInactive: {"F6"},
	}
}

// AllowedInFlattenDialogOverlay reports whether actionID may appear under
// [flatten_dialog_action_keys].
func AllowedInFlattenDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFlattenDestinationActive, ActionFlattenDestinationInactive:
		return true
	default:
		return false
	}
}
