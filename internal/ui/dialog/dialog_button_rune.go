package dialog

// ButtonRuneAction classifies a plain (unmodified) rune key press against the
// conventional dialog bindings: o/O activates OK, c/C activates Cancel, and
// space toggles/activates whatever is focused. Callers still gate on focus
// position, dialog-specific extra runes, and modifiers themselves — this only
// classifies the rune.
type ButtonRuneAction int

const (
	ButtonRuneNone ButtonRuneAction = iota
	ButtonRuneOK
	ButtonRuneCancel
	ButtonRuneToggle
)

// DialogButtonRune maps r to the button action it conventionally represents,
// or ButtonRuneNone when r isn't one of o/O, c/C, or space.
func DialogButtonRune(r rune) ButtonRuneAction {
	switch r {
	case 'o', 'O':
		return ButtonRuneOK
	case 'c', 'C':
		return ButtonRuneCancel
	case ' ':
		return ButtonRuneToggle
	default:
		return ButtonRuneNone
	}
}
