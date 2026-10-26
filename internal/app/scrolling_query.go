package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (a *App) pathPickerScrollingQuery() scrollquery.Edit {
	st := &a.model.PathPicker
	q := &dialog.ScrollingQuery{Value: st.Query, Cursor: st.QueryCursor, Scroll: st.QueryScroll}
	width := a.dialogCtrl.PathPickerQueryWidth()
	edit := scrollquery.Edit{
		Q:     q,
		Width: width,
		EnsureVisible: func() {
			valueLen := len([]rune(q.Value))
			suffixLen := len([]rune(st.Completion.GhostSuffix(q.Value)))
			q.Cursor, q.Scroll = dialog.EnsurePathInputScroll(valueLen, q.Cursor, q.Scroll, width, suffixLen)
		},
	}
	edit.ApplyVisibleAfterErase = func() {
		valueLen := len([]rune(q.Value))
		suffixLen := len([]rune(st.Completion.GhostSuffix(q.Value)))
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
		a.dialogCtrl.SyncPathPickerRanks()
		a.dialogCtrl.SyncPathPickerCompletion()
		if edit.MaxScrollAfterErase != nil && st.QueryScroll > *edit.MaxScrollAfterErase {
			st.QueryScroll = *edit.MaxScrollAfterErase
			q.Scroll = *edit.MaxScrollAfterErase
		}
		edit.MaxScrollAfterErase = nil
		a.dialogCtrl.ArmPathPickerValidateTimer()
		st.Selected = 0
		dialog.EnsurePathPickerListScroll(st, a.dialogCtrl.PathPickerListRows())
	}
	return edit
}

// handleScrollingQueryKey handles edit keys for a focused scrolling query row,
// using the app's [dialog.input] keymap overlay. Returns true when consumed.
func (a *App) handleScrollingQueryKey(ev *tcell.EventKey, inputFocused bool, e scrollquery.Edit) bool {
	return scrollquery.HandleKey(a.keys.DialogInput, ev, inputFocused, e)
}

// dialogInputWidth is the text width of input id in spec on the current screen
// (dlg.InputTextWidth), at least 1.
func (a *App) dialogInputWidth(spec dlg.Spec, id dlg.ID) int {
	w, h := a.screen.Size()
	return max(dlg.InputTextWidth(w, h, spec, id), 1)
}

func (a *App) helpDialogQueryWidth() int { return a.dialogInputWidth(a.helpView(), dialog.HelpFilter) }

func (a *App) historyDialogQueryWidth() int {
	return a.dialogInputWidth(a.historyView(), dialog.HistoryFilter)
}

func (a *App) sftpConnectDialogQueryWidth() int {
	return a.dialogInputWidth(a.sftpConnectView(), dialog.SFTPFilter)
}

func (a *App) groupSelectQueryWidth() int {
	return a.dialogInputWidth(a.groupSelectView(), dialog.PatternInput)
}
