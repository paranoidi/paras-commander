package ui

import "unicode"

// IsWordRune reports readline-style word constituents (letters, digits, underscore).
// Slashes, dots, hyphens, spaces, and other runes act as delimiters between words.
func IsWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// BackwardWordIndex returns the cursor index after moving backward by one word from pos.
// pos is in rune indices; result is clamped to [0, len(runes)].
func BackwardWordIndex(runes []rune, pos int) int {
	pos = clampRuneCursor(pos, len(runes))
	if pos == 0 {
		return 0
	}
	i := pos
	for i > 0 && !IsWordRune(runes[i-1]) {
		i--
	}
	for i > 0 && IsWordRune(runes[i-1]) {
		i--
	}
	return i
}

// ForwardWordIndex returns the cursor index after moving forward by one word from pos.
func ForwardWordIndex(runes []rune, pos int) int {
	pos = clampRuneCursor(pos, len(runes))
	if pos >= len(runes) {
		return len(runes)
	}
	i := pos
	for i < len(runes) && !IsWordRune(runes[i]) {
		i++
	}
	for i < len(runes) && IsWordRune(runes[i]) {
		i++
	}
	return i
}

// KillWordBackward removes the runes from the backward-word boundary up to (but not including) pos.
// It returns the new rune slice and the new cursor (start of deleted region).
func KillWordBackward(runes []rune, pos int) ([]rune, int) {
	pos = clampRuneCursor(pos, len(runes))
	start := BackwardWordIndex(runes, pos)
	if start == pos {
		return runes, pos
	}
	out := make([]rune, 0, len(runes)-(pos-start))
	out = append(out, runes[:start]...)
	out = append(out, runes[pos:]...)
	return out, start
}
