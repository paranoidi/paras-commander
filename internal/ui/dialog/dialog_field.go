package dialog

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

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
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
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
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
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
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
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
	f.Cursor = lineedit.ClampRuneCursor(f.Cursor+delta, len(runes))
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
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	f.Cursor = lineedit.BackwardWordIndex(runes, pos)
}

// MoveWordForward moves the cursor past the end of the next word (readline-style).
func (f *FileDialogField) MoveWordForward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	f.Cursor = lineedit.ForwardWordIndex(runes, pos)
}

// KillWordBackward deletes from the backward-word boundary up to the cursor.
func (f *FileDialogField) KillWordBackward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	newRunes, newPos := lineedit.KillWordBackward(runes, pos)
	f.Value = string(newRunes)
	f.Cursor = newPos
}

// ClearCompletion drops any active filesystem completion ghost text.
func (f *FileDialogField) ClearCompletion() {
	if f == nil {
		return
	}
	f.CompletionSuffix = ""
	f.CompletionIsDir = false
}

// AcceptCompletion inserts CompletionSuffix at the caret and appends "/" when CompletionIsDir.
// Returns false when there is nothing to accept.
func (f *FileDialogField) AcceptCompletion() bool {
	if f == nil || f.CompletionSuffix == "" {
		return false
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	suffix := []rune(f.CompletionSuffix)
	newRunes := make([]rune, 0, len(runes)+len(suffix)+1)
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, suffix...)
	newRunes = append(newRunes, runes[pos:]...)
	f.Value = string(newRunes)
	if f.CompletionIsDir {
		f.Value += "/"
	}
	f.Cursor = len([]rune(f.Value))
	f.ClearCompletion()
	return true
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

// isDialogInputRune mirrors scrollquery.IsDialogInputRune (a plain printable rune with no
// modifier or Shift only). Duplicated here rather than imported: scrollquery already imports
// this package, so importing scrollquery back would cycle.
func isDialogInputRune(ev *tcell.EventKey) bool {
	if ev.Key() != tcell.KeyRune || !unicode.IsPrint(ev.Rune()) {
		return false
	}
	mod := ev.Modifiers()
	return mod == tcell.ModNone || mod == tcell.ModShift
}

// TryDialogInputFieldActions handles [dialog.input] chords (restore default, word motion,
// backward kill word) for a focused dialog text field. keysDialogInput may be nil (no overlay
// configured), f may be nil. Returns true when the chord matched a dialog-input action (even
// when the edit was a no-op), so the caller should not fall through to generic key handling.
func TryDialogInputFieldActions(ev *tcell.EventKey, f *FileDialogField, keysDialogInput *keymap.Map) bool {
	if keysDialogInput == nil || f == nil {
		return false
	}
	id, ok := keysDialogInput.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionDialogInputRestoreDefault:
		return f.RestorePrefill()
	case keymap.ActionDialogInputKillWordBackward:
		f.KillWordBackward()
		return true
	case keymap.ActionDialogInputBackwardWord:
		f.MoveWordBackward()
		return true
	case keymap.ActionDialogInputForwardWord:
		f.MoveWordForward()
		return true
	default:
		return false
	}
}

// TryDialogInputRestore handles just the ui.input.restore-default chord for a focused field
// (narrower than TryDialogInputFieldActions: word-motion/kill-word chords are left unhandled,
// for contexts where the field has no text cursor, e.g. the path-picker glyph focused instead
// of the text). Returns true when the chord matched and the field state changed.
func TryDialogInputRestore(ev *tcell.EventKey, f *FileDialogField, keysDialogInput *keymap.Map) bool {
	if keysDialogInput == nil || f == nil {
		return false
	}
	id, ok := keysDialogInput.Lookup(ev)
	if !ok || id != keymap.ActionDialogInputRestoreDefault {
		return false
	}
	return f.RestorePrefill()
}

// HandleFileDialogFieldKey applies standard text-editing keys to f: [dialog.input] chords via
// keysDialogInput, then cursor motion, backspace/delete/clear, and printable-rune insertion.
// afterEdit runs after any mutation (e.g. mass-rename preview recompute, path completion sync).
// Returns true when the event was consumed.
func HandleFileDialogFieldKey(ev *tcell.EventKey, f *FileDialogField, keysDialogInput *keymap.Map, afterEdit func()) bool {
	if f == nil {
		return false
	}
	if TryDialogInputFieldActions(ev, f, keysDialogInput) {
		if afterEdit != nil {
			afterEdit()
		}
		return true
	}
	edited := false
	switch ev.Key() {
	case tcell.KeyLeft:
		f.MoveCursor(-1)
		edited = true
	case tcell.KeyRight:
		f.MoveCursor(1)
		edited = true
	case tcell.KeyHome:
		f.MoveCursorStart()
		edited = true
	case tcell.KeyEnd:
		f.MoveCursorEnd()
		edited = true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		f.Backspace()
		edited = true
	case tcell.KeyDelete:
		f.Delete()
		edited = true
	case tcell.KeyCtrlL:
		f.Clear()
		edited = true
	case tcell.KeyRune:
		if isDialogInputRune(ev) {
			f.InsertRune(ev.Rune())
			edited = true
		}
	}
	if edited {
		if afterEdit != nil {
			afterEdit()
		}
		return true
	}
	return false
}
