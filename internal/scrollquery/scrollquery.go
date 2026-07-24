// Package scrollquery implements the App-independent core of scrolling-query
// text fields: cursor/scroll editing and key handling shared by every dialog
// that embeds a dialog.ScrollingQuery (find, help, history, SFTP connect,
// group-select, file-preview theme picker, path picker).
package scrollquery

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Edit binds a ScrollingQuery view to mutable string/cursor/scroll fields.
type Edit struct {
	Q             *dialog.ScrollingQuery
	Width         int
	OnChange      func()
	EnsureVisible func() // when nil, uses Q.EnsureVisible(Width)
	// ApplyVisibleAfterErase runs completion + scroll-reveal after deletions (path picker).
	ApplyVisibleAfterErase func()
	// MaxScrollAfterErase caps scroll raised by completion sync in OnChange after reveal.
	MaxScrollAfterErase *int
}

// NewEdit builds an Edit over plain *string/*int bindings (find/help/history/SFTP
// connect/group-select/file-preview theme picker dialogs all follow this shape).
func NewEdit(value *string, cursor, scroll *int, width int, onChange func()) Edit {
	q := &dialog.ScrollingQuery{Value: *value, Cursor: *cursor, Scroll: *scroll}
	edit := Edit{
		Q:     q,
		Width: width,
		OnChange: func() {
			*value = q.Value
			*cursor = q.Cursor
			*scroll = q.Scroll
			if onChange != nil {
				onChange()
			}
		},
	}
	return edit
}

func (e *Edit) applyVisible() {
	if e == nil || e.Q == nil {
		return
	}
	if e.EnsureVisible != nil {
		e.EnsureVisible()
		return
	}
	e.Q.EnsureVisible(e.Width)
}

// Apply commits a pending edit: reveals the cursor then runs OnChange. Used both
// internally by HandleKey and externally by callers that must commit an in-flight
// edit outside of key handling (e.g. group-select "confirm from input").
func (e *Edit) Apply() {
	if e == nil || e.Q == nil || e.OnChange == nil {
		return
	}
	e.applyVisible()
	e.OnChange()
}

func (e *Edit) applyVisibleOnly() {
	if e == nil || e.Q == nil {
		return
	}
	e.applyVisible()
	if e.OnChange != nil {
		e.OnChange()
	}
}

func (e *Edit) applyAfterErase() {
	if e == nil || e.Q == nil {
		return
	}
	if e.ApplyVisibleAfterErase != nil {
		e.ApplyVisibleAfterErase()
	} else {
		e.applyVisible()
	}
	if e.OnChange != nil {
		e.OnChange()
	}
}

// IsDialogInputRune reports a printable rune suitable for dialog text fields.
// Shifted punctuation (e.g. Shift+4 → '$') often arrives with ModShift; Alt/Ctrl are rejected.
func IsDialogInputRune(event *tcell.EventKey) bool {
	if event.Key() != tcell.KeyRune || !unicode.IsPrint(event.Rune()) {
		return false
	}
	mod := event.Modifiers()
	return mod == tcell.ModNone || mod == tcell.ModShift
}

// TryDialogInputActions applies [dialog.input] word/kill actions to a scrolling
// query field. dialogInputKeys is the resolved keymap.Bundle.DialogInput overlay.
// Restore-default is a no-op (no prefill).
func TryDialogInputActions(dialogInputKeys *keymap.Map, ev *tcell.EventKey, e Edit) bool {
	if dialogInputKeys == nil || e.Q == nil {
		return false
	}
	id, ok := dialogInputKeys.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionDialogInputKillWordBackward:
		e.Q.KillWordBackward()
		e.applyAfterErase()
		return true
	case keymap.ActionDialogInputBackwardWord:
		e.Q.MoveWordBackward()
		e.applyVisibleOnly()
		return true
	case keymap.ActionDialogInputForwardWord:
		e.Q.MoveWordForward()
		e.applyVisibleOnly()
		return true
	case keymap.ActionDialogInputRestoreDefault:
		return false
	default:
		return false
	}
}

// HandleKey handles edit keys for a focused scrolling query row. dialogInputKeys is
// the resolved keymap.Bundle.DialogInput overlay. Returns true when the event was consumed.
func HandleKey(dialogInputKeys *keymap.Map, ev *tcell.EventKey, inputFocused bool, e Edit) bool {
	if !inputFocused || e.Q == nil {
		return false
	}
	if TryDialogInputActions(dialogInputKeys, ev, e) {
		return true
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		e.Q.MoveCursor(-1)
		e.applyVisibleOnly()
		return true
	case tcell.KeyRight:
		e.Q.MoveCursor(1)
		e.applyVisibleOnly()
		return true
	case tcell.KeyHome:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return false
		}
		e.Q.MoveCursorStart()
		e.applyVisibleOnly()
		return true
	case tcell.KeyEnd:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return false
		}
		e.Q.MoveCursorEnd()
		e.applyVisibleOnly()
		return true
	case tcell.KeyCtrlA:
		e.Q.MoveCursorStart()
		e.applyVisibleOnly()
		return true
	case tcell.KeyCtrlE:
		e.Q.MoveCursorEnd()
		e.applyVisibleOnly()
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.Q.Backspace()
		e.applyAfterErase()
		return true
	case tcell.KeyDelete:
		e.Q.Delete()
		e.applyAfterErase()
		return true
	case tcell.KeyCtrlL, tcell.KeyCtrlU:
		if e.Q.Value == "" {
			return true
		}
		e.Q.Clear()
		e.Apply()
		return true
	case tcell.KeyRune:
		if IsDialogInputRune(ev) {
			e.Q.InsertRune(ev.Rune())
			e.Apply()
			return true
		}
		return false
	default:
		return false
	}
}

// DialogInputWidthFromFrame returns rect.Width-4 for a centered dialog frame width.
func DialogInputWidthFromFrame(frameWidth int) int {
	if frameWidth < 4 {
		return 0
	}
	return frameWidth - 4
}
