package ui

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

func (f *FileDialogField) commitPrefill() {
	if f.Prefill != "" && f.PrefillPending {
		f.PrefillPending = false
	}
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
