package keymap

// DefaultOpenInPanelKeys is the single definition of the open-in-primary/secondary chords shared
// by the find, pin and dedup overlays.
func DefaultOpenInPanelKeys() map[string][]string {
	return map[string][]string{
		ActionOpenInPrimary:   {"S-left"},
		ActionOpenInSecondary: {"S-right"},
	}
}

func isOpenInPanelAction(actionID string) bool {
	_, ok := DefaultOpenInPanelKeys()[actionID]
	return ok
}

// withOpenInPanelKeys merges DefaultOpenInPanelKeys into an overlay's defaults.
func withOpenInPanelKeys(m map[string][]string) map[string][]string {
	for id, keys := range DefaultOpenInPanelKeys() {
		m[id] = keys
	}
	return m
}
