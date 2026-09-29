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
	if newPos < pos {
		lineedit.SetKillBuffer(runes[newPos:pos])
	}
	f.Value = string(newRunes)
	f.Cursor = newPos
}

// KillWordForward deletes from the cursor up to the forward-word boundary into the kill
// buffer (readline M-d).
func (f *FileDialogField) KillWordForward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	newRunes, newPos := lineedit.KillWordForward(runes, pos)
	if len(newRunes) != len(runes) {
		lineedit.SetKillBuffer(runes[pos : pos+len(runes)-len(newRunes)])
	}
	f.Value = string(newRunes)
	f.Cursor = newPos
}

// CaseWordForward upper- or lowercases up to the next word end and moves the cursor past
// it (readline M-u / M-l).
func (f *FileDialogField) CaseWordForward(upper bool) {
	if f == nil {
		return
	}
	f.commitPrefill()
	newRunes, newPos := lineedit.CaseWordForward([]rune(f.Value), f.Cursor, upper)
	f.Value = string(newRunes)
	f.Cursor = newPos
}

// CapitalizeWordForward capitalizes the next word and moves the cursor past it.
func (f *FileDialogField) CapitalizeWordForward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	newRunes, newPos := lineedit.CapitalizeWordForward([]rune(f.Value), f.Cursor)
	f.Value = string(newRunes)
	f.Cursor = newPos
}

// KillLine stores the whole value in the kill buffer and clears the field. An empty
// field leaves the buffer untouched.
func (f *FileDialogField) KillLine() {
	if f == nil {
		return
	}
	if f.Value != "" {
		lineedit.SetKillBuffer([]rune(f.Value))
	}
	f.Clear()
}

// KillLineBackward stores the text before the cursor in the kill buffer and deletes it
// (readline C-u). No-op at the start of the field.
func (f *FileDialogField) KillLineBackward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	if pos == 0 {
		return
	}
	lineedit.SetKillBuffer(runes[:pos])
	f.Value = string(runes[pos:])
	f.Cursor = 0
}

// KillLineForward stores the text from the cursor to the end in the kill buffer and
// deletes it (readline C-k). No-op at the end of the field.
func (f *FileDialogField) KillLineForward() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	pos := lineedit.ClampRuneCursor(f.Cursor, len(runes))
	if pos == len(runes) {
		return
	}
	lineedit.SetKillBuffer(runes[pos:])
	f.Value = string(runes[:pos])
	f.Cursor = pos
}

// Yank inserts the kill buffer (last C-w deletion) at the cursor.
func (f *FileDialogField) Yank() {
	if f == nil {
		return
	}
	f.commitPrefill()
	runes := []rune(f.Value)
	newRunes, newPos := lineedit.Yank(runes, lineedit.ClampRuneCursor(f.Cursor, len(runes)))
	f.Value = string(newRunes)
	f.Cursor = newPos
}

func (f *FileDialogField) commitPrefill() {
	if f.Prefill != "" && f.PrefillPending {
		f.PrefillPending = false
	}
}

// CommitPrefill clears PrefillPending while keeping Value (placeholder becomes committed text).
// Used when Right should accept the suggestion before a second Right moves to the path-picker icon.
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
	case keymap.ActionDialogInputKillWordForward:
		f.KillWordForward()
		return true
	case keymap.ActionDialogInputKillLine:
		f.KillLine()
		return true
	case keymap.ActionDialogInputKillLineBackward:
		f.KillLineBackward()
		return true
	case keymap.ActionDialogInputKillLineForward:
		f.KillLineForward()
		return true
	case keymap.ActionDialogInputYank:
		f.Yank()
		return true
	case keymap.ActionDialogInputBackwardWord:
		f.MoveWordBackward()
		return true
	case keymap.ActionDialogInputForwardWord:
		f.MoveWordForward()
		return true
	case keymap.ActionDialogInputUpcaseWord:
		f.CaseWordForward(true)
		return true
	case keymap.ActionDialogInputDowncaseWord:
		f.CaseWordForward(false)
		return true
	case keymap.ActionDialogInputCapitalizeWord:
		f.CapitalizeWordForward()
		return true
	case keymap.ActionDialogInputLineStart:
		f.MoveCursorStart()
		return true
	case keymap.ActionDialogInputLineEnd:
		f.MoveCursorEnd()
		return true
	default:
		return false
	}
}

// TryDialogInputRestore handles just the ui.input.restore-default chord for a focused field
// (narrower than TryDialogInputFieldActions: word-motion/kill-word chords are left unhandled,
// for contexts where the field has no text cursor, e.g. the path-picker icon focused instead
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
