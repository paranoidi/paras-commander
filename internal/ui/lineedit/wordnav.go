package lineedit

import (
	"sync"
	"unicode"
)

var (
	killMu     sync.Mutex
	killBuffer []rune
)

// SetKillBuffer stores the text last removed by a kill (shared by all fields, like a shell kill ring).
func SetKillBuffer(r []rune) {
	killMu.Lock()
	killBuffer = append([]rune(nil), r...)
	killMu.Unlock()
}

// KillBuffer returns a copy of the kill buffer.
func KillBuffer() []rune {
	killMu.Lock()
	defer killMu.Unlock()
	return append([]rune(nil), killBuffer...)
}

// Yank inserts the kill buffer at pos and returns the new runes and cursor (end of inserted text).
func Yank(runes []rune, pos int) ([]rune, int) {
	buf := KillBuffer()
	if len(buf) == 0 {
		return runes, ClampRuneCursor(pos, len(runes))
	}
	pos = ClampRuneCursor(pos, len(runes))
	out := make([]rune, 0, len(runes)+len(buf))
	out = append(out, runes[:pos]...)
	out = append(out, buf...)
	out = append(out, runes[pos:]...)
	return out, pos + len(buf)
}

// ClampRuneCursor clamps pos to [0, length] for rune-indexed cursors.
func ClampRuneCursor(pos, length int) int {
	if pos < 0 {
		return 0
	}
	if pos > length {
		return length
	}
	return pos
}

// IsWordRune reports readline-style word constituents (letters, digits, underscore).
// Slashes, dots, hyphens, spaces, and other runes act as delimiters between words.
func IsWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// BackwardWordIndex returns the cursor index after moving backward by one word from pos.
// pos is in rune indices; result is clamped to [0, len(runes)].
func BackwardWordIndex(runes []rune, pos int) int {
	pos = ClampRuneCursor(pos, len(runes))
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
	pos = ClampRuneCursor(pos, len(runes))
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

// KillWordForward removes the runes from pos up to the forward-word boundary (readline M-d).
// It returns the new rune slice and the cursor (unchanged pos).
func KillWordForward(runes []rune, pos int) ([]rune, int) {
	pos = ClampRuneCursor(pos, len(runes))
	end := ForwardWordIndex(runes, pos)
	if end == pos {
		return runes, pos
	}
	out := make([]rune, 0, len(runes)-(end-pos))
	out = append(out, runes[:pos]...)
	out = append(out, runes[end:]...)
	return out, pos
}

// KillWordBackward removes the runes from the backward-word boundary up to (but not including) pos.
// It returns the new rune slice and the new cursor (start of deleted region).
func KillWordBackward(runes []rune, pos int) ([]rune, int) {
	pos = ClampRuneCursor(pos, len(runes))
	start := BackwardWordIndex(runes, pos)
	if start == pos {
		return runes, pos
	}
	out := make([]rune, 0, len(runes)-(pos-start))
	out = append(out, runes[:start]...)
	out = append(out, runes[pos:]...)
	return out, start
}
