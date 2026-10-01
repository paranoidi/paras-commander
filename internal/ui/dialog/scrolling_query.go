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
	newRunes, cur := lineedit.InsertRune(runes, pos, r)
	q.Value = string(newRunes)
	q.Cursor = cur
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
	newRunes, cur := lineedit.DeleteBefore(runes, pos)
	q.Value = string(newRunes)
	q.Cursor = cur
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
	newRunes, _ := lineedit.DeleteAt(runes, pos)
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
	if newPos < pos {
		lineedit.SetKillBuffer(runes[newPos:pos])
	}
	q.Value = string(newRunes)
	q.Cursor = newPos
}

// KillWordForward deletes from the caret up to the forward-word boundary into the kill
// buffer (readline M-d).
func (q *ScrollingQuery) KillWordForward() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	newRunes, newPos := lineedit.KillWordForward(runes, pos)
	if len(newRunes) != len(runes) {
		lineedit.SetKillBuffer(runes[pos : pos+len(runes)-len(newRunes)])
	}
	q.Value = string(newRunes)
	q.Cursor = newPos
}

// CaseWordForward upper- or lowercases up to the next word end and moves the caret past
// it (readline M-u / M-l).
func (q *ScrollingQuery) CaseWordForward(upper bool) {
	if q == nil {
		return
	}
	newRunes, newPos := lineedit.CaseWordForward([]rune(q.Value), q.Cursor, upper)
	q.Value = string(newRunes)
	q.Cursor = newPos
}

// CapitalizeWordForward capitalizes the next word and moves the caret past it.
func (q *ScrollingQuery) CapitalizeWordForward() {
	if q == nil {
		return
	}
	newRunes, newPos := lineedit.CapitalizeWordForward([]rune(q.Value), q.Cursor)
	q.Value = string(newRunes)
	q.Cursor = newPos
}

// KillLine stores the whole value in the kill buffer and clears the query. An empty
// query leaves the buffer untouched.
func (q *ScrollingQuery) KillLine() {
	if q == nil {
		return
	}
	if q.Value != "" {
		lineedit.SetKillBuffer([]rune(q.Value))
	}
	q.Clear()
}

// KillLineBackward stores the text before the caret in the kill buffer and deletes it
// (readline C-u). No-op at the start of the query.
func (q *ScrollingQuery) KillLineBackward() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	if pos == 0 {
		return
	}
	lineedit.SetKillBuffer(runes[:pos])
	q.Value = string(runes[pos:])
	q.Cursor = 0
}

// KillLineForward stores the text from the caret to the end in the kill buffer and
// deletes it (readline C-k). No-op at the end of the query.
func (q *ScrollingQuery) KillLineForward() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	pos := lineedit.ClampRuneCursor(q.Cursor, len(runes))
	if pos == len(runes) {
		return
	}
	lineedit.SetKillBuffer(runes[pos:])
	q.Value = string(runes[:pos])
	q.Cursor = pos
}

// Yank inserts the kill buffer (last killed text) at the caret.
func (q *ScrollingQuery) Yank() {
	if q == nil {
		return
	}
	runes := []rune(q.Value)
	newRunes, newPos := lineedit.Yank(runes, lineedit.ClampRuneCursor(q.Cursor, len(runes)))
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
