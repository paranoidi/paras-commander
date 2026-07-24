package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) pathPickerScrollingQuery() scrollquery.Edit {
	st := &a.model.PathPicker
	q := &dialog.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	width := a.pathPickerQueryWidth()
	edit := scrollquery.Edit{
		Q:     q,
		Width: width,
		EnsureVisible: func() {
			valueLen := len([]rune(q.Value))
			suffixLen := len([]rune(st.QueryCompletionSuffix))
			q.Cursor, q.Scroll = dialog.EnsurePathInputScroll(valueLen, q.Cursor, q.Scroll, width, suffixLen)
		},
	}
	edit.ApplyVisibleAfterErase = func() {
		valueLen := len([]rune(q.Value))
		suffixLen := len([]rune(st.QueryCompletionSuffix))
		if dialog.ScrollContentLen(valueLen, q.Cursor) <= width {
			q.Scroll = 0
		} else if q.Scroll > 0 {
			q.Cursor, q.Scroll = dialog.AdjustScrollRevealOnErase(q.Value, q.Cursor, q.Scroll, width, suffixLen)
		}
		q.Cursor, q.Scroll = dialog.EnsurePathInputScroll(valueLen, q.Cursor, q.Scroll, width, suffixLen)
		max := q.Scroll
		edit.MaxScrollAfterErase = &max
	}
	edit.OnChange = func() {
		st.Query = q.Value
		st.QueryCursor = q.Cursor
		st.QueryScroll = q.Scroll
		a.syncPathPickerRanks()
		a.syncPathPickerCompletion()
		if edit.MaxScrollAfterErase != nil && st.QueryScroll > *edit.MaxScrollAfterErase {
			st.QueryScroll = *edit.MaxScrollAfterErase
			q.Scroll = *edit.MaxScrollAfterErase
		}
		edit.MaxScrollAfterErase = nil
		a.armPathPickerValidateTimer()
		st.Selected = 0
		dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
	}
	return edit
}

// handleScrollingQueryKey handles edit keys for a focused scrolling query row,
// using the app's [dialog.input] keymap overlay. Returns true when consumed.
func (a *App) handleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, e scrollquery.Edit) bool {
	return scrollquery.HandleKey(a.keys.DialogInput, ev, inputFocused, e)
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
	return scrollquery.DialogInputWidthFromFrame(width)
}

func (a *App) helpDialogQueryWidth() int {
	termW, termH := a.screen.Size()
	layout := a.layoutForTerminalSize(termW, termH)
	metrics, ok := dialog.ComputeHelpDialogListMetrics(layout)
	if !ok {
		return scrollquery.DialogInputWidthFromFrame(78)
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
	return scrollquery.DialogInputWidthFromFrame(width)
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
	return scrollquery.DialogInputWidthFromFrame(width)
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
	return scrollquery.DialogInputWidthFromFrame(width)
}
