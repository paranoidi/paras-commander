package keymap

// DefaultActionKeys returns built-in default chords per action (single-stroke only).
// Derived from DefaultActionSpecs() so they stay in sync.
func DefaultActionKeys() map[string][]string {
	out := make(map[string][]string, len(DefaultActionSpecs()))
	for _, spec := range DefaultActionSpecs() {
		if len(spec.DefaultKeys) > 0 {
			keys := make([]string, len(spec.DefaultKeys))
			copy(keys, spec.DefaultKeys)
			out[spec.ID] = keys
		}
	}
	return out
}
