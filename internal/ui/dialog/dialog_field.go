package dialog

// InsertRune inserts r at the field cursor. If the field is still showing a
// suggested prefill, the first printable input replaces the suggestion.
func (f *FileDialogField) InsertRune(r rune) {
	if f == nil {
		return
	}
	if f.Prefill != "" && f.PrefillPending {
		f.Value = ""
		f.Cursor = 0
		f.PrefillPending = false
	}
	runes := []rune(f.Value)
	pos := clampRuneCursor(f.Cursor, len(runes))
	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, r)
	newRunes = append(newRunes, runes[pos:]...)
	f.Value = string(newRunes)
	f.Cursor = pos + 1
}

// Backspace removes the rune before the field cursor.
func (f *FileDialogField) Backspace() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := clampRuneCursor(f.Cursor, len(runes))
	if pos <= 0 || len(runes) == 0 {
		return
	}
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:pos-1]...)
	newRunes = append(newRunes, runes[pos:]...)
	f.Value = string(newRunes)
	f.Cursor = pos - 1
}

// Delete removes the rune at the field cursor.
func (f *FileDialogField) Delete() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := clampRuneCursor(f.Cursor, len(runes))
	if pos >= len(runes) {
		return
	}
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, runes[pos+1:]...)
	f.Value = string(newRunes)
	f.Cursor = pos
}

// Clear removes all text from the field.
func (f *FileDialogField) Clear() {
	if f == nil {
		return
	}
	f.Value = ""
	f.Cursor = 0
	f.PrefillPending = false
}

// RestorePrefill resets the field to its suggested default state: Value becomes
// Prefill, the cursor moves to the end, and PrefillPending is re-armed so the
// next printable rune replaces from scratch (matching the on-open behaviour).
// Returns false (no-op) when Prefill is empty.
func (f *FileDialogField) RestorePrefill() bool {
	if f == nil || f.Prefill == "" {
		return false
	}
	f.Value = f.Prefill
	f.Cursor = len([]rune(f.Prefill))
	f.PrefillPending = true
	return true
}

// MoveCursor moves the cursor by delta runes and commits pending prefill text.
func (f *FileDialogField) MoveCursor(delta int) {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	f.Cursor = clampRuneCursor(f.Cursor+delta, len(runes))
}

// MoveCursorStart moves the cursor to the beginning and commits pending prefill text.
func (f *FileDialogField) MoveCursorStart() {
	if f == nil {
		return
	}
	f.commitPrefill()
	f.Cursor = 0
}

// MoveCursorEnd moves the cursor to the end and commits pending prefill text.
func (f *FileDialogField) MoveCursorEnd() {
	if f == nil {
		return
	}
	f.commitPrefill()
	f.Cursor = len([]rune(f.Value))
}

// MoveWordBackward moves the cursor to the start of the previous word (readline-style).
func (f *FileDialogField) MoveWordBackward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := clampRuneCursor(f.Cursor, len(runes))
	f.Cursor = BackwardWordIndex(runes, pos)
}

// MoveWordForward moves the cursor past the end of the next word (readline-style).
func (f *FileDialogField) MoveWordForward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := clampRuneCursor(f.Cursor, len(runes))
	f.Cursor = ForwardWordIndex(runes, pos)
}

// KillWordBackward deletes from the backward-word boundary up to the cursor.
func (f *FileDialogField) KillWordBackward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := clampRuneCursor(f.Cursor, len(runes))
	newRunes, newPos := KillWordBackward(runes, pos)
	f.Value = string(newRunes)
	f.Cursor = newPos
}

func (f *FileDialogField) commitPrefill() {
	if f.Prefill != "" && f.PrefillPending {
		f.PrefillPending = false
	}
}

// CommitPrefill clears PrefillPending while keeping Value (placeholder becomes committed text).
// Used when Right should accept the suggestion before a second Right moves to the path-picker glyph.
func (f *FileDialogField) CommitPrefill() {
	if f == nil {
		return
	}
	f.commitPrefill()
}

func clampRuneCursor(pos, length int) int {
	if pos < 0 {
		return 0
	}
	if pos > length {
		return length
	}
	return pos
}
