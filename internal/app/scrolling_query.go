package app

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// scrollingQueryEdit binds a ScrollingQuery view to mutable string/cursor/scroll fields.
type scrollingQueryEdit struct {
	q        *ui.ScrollingQuery
	width    int
	onChange func()
}

func (a *App) pathPickerScrollingQuery() scrollingQueryEdit {
	st := &a.model.PathPicker
	q := &ui.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	return scrollingQueryEdit{
		q:     q,
		width: a.pathPickerQueryWidth(),
		onChange: func() {
			st.Query = q.Value
			st.QueryCursor = q.Cursor
			st.QueryScroll = q.Scroll
			a.syncPathPickerRanks()
			a.armPathPickerValidateTimer()
			st.Selected = 0
			ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		},
	}
}

func findDialogScrollingQuery(st *ui.FindDialogState, width int, onChange func()) scrollingQueryEdit {
	q := &ui.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	return scrollingQueryEdit{
		q:     q,
		width: width,
		onChange: func() {
			st.Query = q.Value
			st.QueryCursor = q.Cursor
			st.QueryScroll = q.Scroll
			if onChange != nil {
				onChange()
			}
		},
	}
}

func helpViewScrollingQuery(st *ui.HelpViewState, width int, onChange func()) scrollingQueryEdit {
	q := &ui.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	return scrollingQueryEdit{
		q:     q,
		width: width,
		onChange: func() {
			st.Query = q.Value
			st.QueryCursor = q.Cursor
			st.QueryScroll = q.Scroll
			if onChange != nil {
				onChange()
			}
		},
	}
}

func historyDialogScrollingQuery(st *ui.HistoryDialogState, width int, onChange func()) scrollingQueryEdit {
	q := &ui.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	return scrollingQueryEdit{
		q:     q,
		width: width,
		onChange: func() {
			st.Query = q.Value
			st.QueryCursor = q.Cursor
			st.QueryScroll = q.Scroll
			if onChange != nil {
				onChange()
			}
		},
	}
}

func groupSelectScrollingQuery(gs *ui.GroupSelectState, width int) scrollingQueryEdit {
	q := &ui.ScrollingQuery{Value: gs.Text, Cursor: gs.TextCursor, Scroll: gs.TextScroll}
	return scrollingQueryEdit{
		q:     q,
		width: width,
		onChange: func() {
			gs.Text = q.Value
			gs.TextCursor = q.Cursor
			gs.TextScroll = q.Scroll
		},
	}
}

func (e *scrollingQueryEdit) apply() {
	if e == nil || e.q == nil || e.onChange == nil {
		return
	}
	e.q.EnsureVisible(e.width)
	e.onChange()
}

func (e *scrollingQueryEdit) applyVisibleOnly() {
	if e == nil || e.q == nil {
		return
	}
	e.q.EnsureVisible(e.width)
	if e.onChange != nil {
		e.onChange()
	}
}

// tryScrollingQueryDialogInputActions applies [dialog_input_action_keys] word/kill
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
		e.apply()
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
		e.apply()
		return true
	case tcell.KeyDelete:
		e.q.Delete()
		e.apply()
		return true
	case tcell.KeyCtrlL, tcell.KeyCtrlU:
		if e.q.Value == "" {
			return true
		}
		e.q.Clear()
		e.apply()
		return true
	case tcell.KeyRune:
		if ev.Modifiers() != tcell.ModNone {
			return false
		}
		if unicode.IsPrint(ev.Rune()) {
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
	metrics, ok := ui.ComputeHelpDialogListMetrics(layout)
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
