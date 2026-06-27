package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// scrollingQueryEdit binds a ScrollingQuery view to mutable string/cursor/scroll fields.
type scrollingQueryEdit struct {
	q             *dialog.ScrollingQuery
	width         int
	onChange      func()
	ensureVisible func() // when nil, uses q.EnsureVisible(width)
	// applyVisibleAfterErase runs completion + scroll-reveal after deletions (path picker).
	applyVisibleAfterErase func()
	// maxScrollAfterErase caps scroll raised by completion sync in onChange after reveal.
	maxScrollAfterErase *int
}

func (a *App) pathPickerScrollingQuery() scrollingQueryEdit {
	st := &a.model.PathPicker
	q := &dialog.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	width := a.pathPickerQueryWidth()
	edit := scrollingQueryEdit{
		q:     q,
		width: width,
		ensureVisible: func() {
			valueLen := len([]rune(q.Value))
			suffixLen := len([]rune(st.QueryCompletionSuffix))
			q.Cursor, q.Scroll = dialog.EnsurePathInputScroll(valueLen, q.Cursor, q.Scroll, width, suffixLen)
		},
	}
	edit.applyVisibleAfterErase = func() {
		valueLen := len([]rune(q.Value))
		suffixLen := len([]rune(st.QueryCompletionSuffix))
		if dialog.ScrollContentLen(valueLen, q.Cursor) <= width {
			q.Scroll = 0
		} else if q.Scroll > 0 {
			q.Cursor, q.Scroll = dialog.AdjustScrollRevealOnErase(q.Value, q.Cursor, q.Scroll, width, suffixLen)
		}
		q.Cursor, q.Scroll = dialog.EnsurePathInputScroll(valueLen, q.Cursor, q.Scroll, width, suffixLen)
		max := q.Scroll
		edit.maxScrollAfterErase = &max
	}
	edit.onChange = func() {
		st.Query = q.Value
		st.QueryCursor = q.Cursor
		st.QueryScroll = q.Scroll
		a.syncPathPickerRanks()
		a.syncPathPickerCompletion()
		if edit.maxScrollAfterErase != nil && st.QueryScroll > *edit.maxScrollAfterErase {
			st.QueryScroll = *edit.maxScrollAfterErase
			q.Scroll = *edit.maxScrollAfterErase
		}
		edit.maxScrollAfterErase = nil
		a.armPathPickerValidateTimer()
		st.Selected = 0
		dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
	}
	return edit
}

func newScrollingQueryEdit(value *string, cursor, scroll *int, width int, onChange func()) scrollingQueryEdit {
	q := &dialog.ScrollingQuery{Value: *value, Cursor: *cursor, Scroll: *scroll}
	edit := scrollingQueryEdit{
		q:     q,
		width: width,
		onChange: func() {
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

func findDialogScrollingQuery(st *dialog.FindDialogState, width int, onChange func()) scrollingQueryEdit {
	return newScrollingQueryEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, width, onChange)
}

func helpViewScrollingQuery(st *dialog.HelpViewState, width int, onChange func()) scrollingQueryEdit {
	return newScrollingQueryEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, width, onChange)
}

func historyDialogScrollingQuery(st *dialog.HistoryDialogState, width int, onChange func()) scrollingQueryEdit {
	return newScrollingQueryEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, width, onChange)
}

func sftpConnectDialogScrollingQuery(st *dialog.SFTPConnectDialogState, width int, onChange func()) scrollingQueryEdit {
	return newScrollingQueryEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, width, onChange)
}

func groupSelectScrollingQuery(gs *dialog.GroupSelectState, width int) scrollingQueryEdit {
	return newScrollingQueryEdit(&gs.Text, &gs.TextCursor, &gs.TextScroll, width, nil)
}

func (e *scrollingQueryEdit) applyVisible() {
	if e == nil || e.q == nil {
		return
	}
	if e.ensureVisible != nil {
		e.ensureVisible()
		return
	}
	e.q.EnsureVisible(e.width)
}

func (e *scrollingQueryEdit) apply() {
	if e == nil || e.q == nil || e.onChange == nil {
		return
	}
	e.applyVisible()
	e.onChange()
}

func (e *scrollingQueryEdit) applyVisibleOnly() {
	if e == nil || e.q == nil {
		return
	}
	e.applyVisible()
	if e.onChange != nil {
		e.onChange()
	}
}

func (e *scrollingQueryEdit) applyAfterErase() {
	if e == nil || e.q == nil {
		return
	}
	if e.applyVisibleAfterErase != nil {
		e.applyVisibleAfterErase()
	} else {
		e.applyVisible()
	}
	if e.onChange != nil {
		e.onChange()
	}
}

// tryScrollingQueryDialogInputActions applies [dialog.input] word/kill
// actions to a scrolling query field. Restore-default is a no-op (no prefill).
func (a *App) tryScrollingQueryDialogInputActions(ev *tcell.EventKey, e scrollingQueryEdit) bool {
	if a.keysDialogInput == nil || e.q == nil {
		return false
	}
	id, ok := a.keysDialogInput.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionDialogInputKillWordBackward:
		e.q.KillWordBackward()
		e.applyAfterErase()
		return true
	case keymap.ActionDialogInputBackwardWord:
		e.q.MoveWordBackward()
		e.applyVisibleOnly()
		return true
	case keymap.ActionDialogInputForwardWord:
		e.q.MoveWordForward()
		e.applyVisibleOnly()
		return true
	case keymap.ActionDialogInputRestoreDefault:
		return false
	default:
		return false
	}
}

// handleScrollingQueryKey handles edit keys for a focused scrolling query row.
// Returns true when the event was consumed.
func (a *App) handleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, e scrollingQueryEdit) bool {
	if !inputFocused || e.q == nil {
		return false
	}
	if a.tryScrollingQueryDialogInputActions(ev, e) {
		return true
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		e.q.MoveCursor(-1)
		e.applyVisibleOnly()
		return true
	case tcell.KeyRight:
		e.q.MoveCursor(1)
		e.applyVisibleOnly()
		return true
	case tcell.KeyHome:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return false
		}
		e.q.MoveCursorStart()
		e.applyVisibleOnly()
		return true
	case tcell.KeyEnd:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			return false
		}
		e.q.MoveCursorEnd()
		e.applyVisibleOnly()
		return true
	case tcell.KeyCtrlA:
		e.q.MoveCursorStart()
		e.applyVisibleOnly()
		return true
	case tcell.KeyCtrlE:
		e.q.MoveCursorEnd()
		e.applyVisibleOnly()
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.q.Backspace()
		e.applyAfterErase()
		return true
	case tcell.KeyDelete:
		e.q.Delete()
		e.applyAfterErase()
		return true
	case tcell.KeyCtrlL, tcell.KeyCtrlU:
		if e.q.Value == "" {
			return true
		}
		e.q.Clear()
		e.apply()
		return true
	case tcell.KeyRune:
		if isDialogInputRune(ev) {
			e.q.InsertRune(ev.Rune())
			e.apply()
			return true
		}
		return false
	default:
		return false
	}
}

// dialogInputWidthFromFrame returns rect.Width-4 for a centered dialog frame width.
func dialogInputWidthFromFrame(frameWidth int) int {
	if frameWidth < 4 {
		return 0
	}
	return frameWidth - 4
}

func (a *App) findDialogQueryWidth() int {
	termW, _ := a.screen.Size()
	width := 117
	if width > termW-4 {
		width = termW - 4
	}
	if width < 54 {
		width = 54
	}
	return dialogInputWidthFromFrame(width)
}

func (a *App) helpDialogQueryWidth() int {
	termW, termH := a.screen.Size()
	layout := a.layoutForTerminalSize(termW, termH)
	metrics, ok := dialog.ComputeHelpDialogListMetrics(layout)
	if !ok {
		return dialogInputWidthFromFrame(78)
	}
	return metrics.InputWidth
}

func (a *App) historyDialogQueryWidth() int {
	termW, _ := a.screen.Size()
	width := 78
	if width > termW-4 {
		width = termW - 4
	}
	if width < 36 {
		width = 36
	}
	return dialogInputWidthFromFrame(width)
}

func (a *App) sftpConnectDialogQueryWidth() int {
	termW, _ := a.screen.Size()
	width := 78
	if width > termW-4 {
		width = termW - 4
	}
	if width < 40 {
		width = 40
	}
	return dialogInputWidthFromFrame(width)
}

func (a *App) groupSelectQueryWidth() int {
	termW, _ := a.screen.Size()
	width := 54
	if width > termW-4 {
		width = termW - 4
	}
	if width < 30 {
		width = 30
	}
	return dialogInputWidthFromFrame(width)
}
