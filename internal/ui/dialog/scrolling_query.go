package dialog

import "github.com/paranoidi/paras-commander/internal/ui/lineedit"

// ScrollingQuery is a single-line dialog filter/pattern field with caret and
// horizontal scroll state (used by fuzzy list dialogs and the path picker).
type ScrollingQuery struct {
	Value  string
	Cursor int // rune offset within Value (0..len(runes))
	Scroll int // first visible rune offset for horizontal scrolling
}

// InsertRune inserts r at the caret.
func (q *ScrollingQuery) InsertRune(r rune) {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, r)
	newRunes = append(newRunes, runes[pos:]...)
	q.Value = string(newRunes)
	q.Cursor = pos + 1
}

// Backspace removes the rune before the caret.
func (q *ScrollingQuery) Backspace() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	if pos <= 0 || len(runes) == 0 {
		return
	}
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:pos-1]...)
	newRunes = append(newRunes, runes[pos:]...)
	q.Value = string(newRunes)
	q.Cursor = pos - 1
}

// Delete removes the rune at the caret.
func (q *ScrollingQuery) Delete() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	if pos >= len(runes) {
		return
	}
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, runes[pos+1:]...)
	q.Value = string(newRunes)
}

// Clear removes all text and resets caret/scroll.
func (q *ScrollingQuery) Clear() {
	if q == nil {
		return
	}
	q.Value = ""
	q.Cursor = 0
	q.Scroll = 0
}

// MoveCursor moves the caret by delta runes.
func (q *ScrollingQuery) MoveCursor(delta int) {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	q.Cursor = lineedit.ClampRuneCursor(q.Cursor+delta, len(runes))
}

// MoveCursorStart moves the caret to the beginning.
func (q *ScrollingQuery) MoveCursorStart() {
	if q == nil {
		return
	}
	q.Cursor = 0
}

// MoveCursorEnd moves the caret to the end.
func (q *ScrollingQuery) MoveCursorEnd() {
	if q == nil {
		return
	}
	q.Cursor = len([]rune(q.Value))
}

// MoveWordBackward moves the caret to the start of the previous word.
func (q *ScrollingQuery) MoveWordBackward() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	q.Cursor = lineedit.BackwardWordIndex(runes, pos)
}

// MoveWordForward moves the caret past the end of the next word.
func (q *ScrollingQuery) MoveWordForward() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	q.Cursor = lineedit.ForwardWordIndex(runes, pos)
}

// KillWordBackward deletes from the backward-word boundary up to the caret.
func (q *ScrollingQuery) KillWordBackward() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	newRunes, newPos := lineedit.KillWordBackward(runes, pos)
	q.Value = string(newRunes)
	q.Cursor = newPos
}

// EnsureVisible adjusts Scroll so Cursor stays within the visible width.
func (q *ScrollingQuery) EnsureVisible(width int) {
	if q == nil {
		return
	}
	length := len([]rune(q.Value))
	q.Cursor, q.Scroll = EnsureScrollInputVisible(length, q.Cursor, q.Scroll, width)
}
