package lineedit

// InsertRune inserts r at pos and returns the new text and caret.
func InsertRune(runes []rune, pos int, r rune) ([]rune, int) {
	pos = ClampRuneCursor(pos, len(runes))
	out := make([]rune, 0, len(runes)+1)
	out = append(out, runes[:pos]...)
	out = append(out, r)
	out = append(out, runes[pos:]...)
	return out, pos + 1
}

// DeleteBefore removes the rune before pos (backspace). No-op when pos <= 0.
func DeleteBefore(runes []rune, pos int) ([]rune, int) {
	pos = ClampRuneCursor(pos, len(runes))
	if pos <= 0 {
		return runes, pos
	}
	out := make([]rune, 0, len(runes)-1)
	out = append(out, runes[:pos-1]...)
	out = append(out, runes[pos:]...)
	return out, pos - 1
}

// DeleteAt removes the rune at pos. No-op when pos >= len(runes).
func DeleteAt(runes []rune, pos int) ([]rune, int) {
	pos = ClampRuneCursor(pos, len(runes))
	if pos >= len(runes) {
		return runes, pos
	}
	out := make([]rune, 0, len(runes)-1)
	out = append(out, runes[:pos]...)
	out = append(out, runes[pos+1:]...)
	return out, pos
}
