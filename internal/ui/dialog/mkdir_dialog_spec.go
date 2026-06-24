package dialog

import "unicode"

// MkdirActionRadioSpec describes one mkdir-with-selection post-action radio row.
type MkdirActionRadioSpec struct {
	Action   MkdirAction
	Label    string
	Shortcut rune
}

// MkdirActionRadioSpecs returns radio rows for mkdir when MkdirShowActions is true.
// Order matches MkdirAction iota and focus indices after len(Fields).
func MkdirActionRadioSpecs() []MkdirActionRadioSpec {
	return []MkdirActionRadioSpec{
		{MkdirActionCreate, "Create", 'r'},
		{MkdirActionCreateCopySelect, "and copy selected", 'y'},
		{MkdirActionCreateMoveSelect, "and move selected", 'm'},
	}
}

// MkdirActionForAltShortcut returns the action and radio focus offset when Alt+letter
// matches a mkdir radio mnemonic (case-insensitive). The second result is false when
// there is no match.
func MkdirActionForAltShortcut(r rune) (MkdirAction, int, bool) {
	for i, spec := range MkdirActionRadioSpecs() {
		if runeEqualFold(spec.Shortcut, r) {
			return spec.Action, i, true
		}
	}
	return 0, 0, false
}

func runeEqualFold(a, b rune) bool {
	return unicode.ToLower(a) == unicode.ToLower(b)
}
