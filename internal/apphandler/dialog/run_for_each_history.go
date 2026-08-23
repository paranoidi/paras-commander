package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// RunForEachHistoryFooterEligible reports whether the footer should show the F3 "History" hint:
// the run-for-each dialog is open on its main screen (not the history picker itself).
func (h *Handler) RunForEachHistoryFooterEligible() bool {
	d := &h.model.FileDialog
	return d.Open && d.DialogType == dialog.FileDialogRunForEach && !d.RunForEachHistoryOpen
}

// tryRunForEachDialogShortcut handles [dialog.run_for_each] while the run-for-each dialog's
// main screen is active. Returns true when the event was consumed.
func (h *Handler) tryRunForEachDialogShortcut(ev *tcell.EventKey) bool {
	if h.keysRunForEachDialog == nil {
		return false
	}
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogRunForEach || d.RunForEachHistoryOpen {
		return false
	}
	id, ok := h.keysRunForEachDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileRunForEachHistory:
		h.openRunForEachHistoryPicker()
		return true
	default:
		return false
	}
}

// openRunForEachHistoryPicker opens the fuzzy command-history picker over the in-memory,
// session-only recently-run command list (see commandsctrl.Handler.RunForEachHistory).
func (h *Handler) openRunForEachHistoryPicker() {
	d := &h.model.FileDialog
	d.RunForEachHistoryPicker = dialog.RunForEachHistoryPickerState{Items: h.commands.RunForEachHistory()}
	d.RunForEachHistoryOpen = true
	h.syncRunForEachHistoryPickerRanks()
}

// closeRunForEachHistoryPicker returns to the main run-for-each screen without applying a
// selection (Esc / Cancel). d.Fields are left untouched.
func (h *Handler) closeRunForEachHistoryPicker() {
	h.model.FileDialog.RunForEachHistoryOpen = false
}

// syncRunForEachHistoryPickerRanks re-filters and re-ranks the open picker's command list
// against the current query, clamping selection and list scroll.
func (h *Handler) syncRunForEachHistoryPickerRanks() {
	st := &h.model.FileDialog.RunForEachHistoryPicker
	cfg := h.host.Config()
	st.Ranked, st.MatchRanges = h.host.SyncFilteredListRanks(st.Items, st.Query, len(st.Items), cfg.Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsureRunForEachHistoryPickerListScroll(st, h.RunForEachHistoryPickerListRows())
}

// RunForEachHistoryPickerListRows returns how many rows the open picker's fuzzy list currently
// shows, derived from the dialog's actual on-screen rect (see runForEachHistoryPickerDialogHeight
// / drawRunForEachHistoryPickerContent).
func (h *Handler) RunForEachHistoryPickerListRows() int {
	rows := h.FileDialogRect().Height - 6
	if rows < 1 {
		rows = 1
	}
	return rows
}

// RunForEachHistoryPickerQueryWidth returns the visible width of the open picker's query input
// row.
func (h *Handler) RunForEachHistoryPickerQueryWidth() int {
	w := h.FileDialogRect().Width - 4
	if w < 10 {
		w = 10
	}
	return w
}

// activateRunForEachHistorySelection applies the selected command line back into the main
// run-for-each dialog's Command field and recomputes validation (Enter / OK).
func (h *Handler) activateRunForEachHistorySelection() {
	d := &h.model.FileDialog
	st := &d.RunForEachHistoryPicker
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return
	}
	value := st.Items[entIdx]
	if len(d.Fields) > 0 {
		d.Fields[0].Value = value
		d.Fields[0].Cursor = len([]rune(value))
		d.Fields[0].Prefill = ""
		d.Fields[0].PrefillPending = false
	}
	d.RunForEachHistoryOpen = false
	h.commands.RecomputeRunForEachValidation()
}

// handleRunForEachHistoryPickerKey routes a key event for the open command-history picker.
// Mirrors handleMassRenamePickerKey's shape (mass_rename_pattern.go) minus the F8-delete
// chord — run-for-each history has no on-disk counterpart to justify one.
func (h *Handler) handleRunForEachHistoryPickerKey(event *tcell.EventKey) bool {
	st := &h.model.FileDialog.RunForEachHistoryPicker

	if dialog.TryStandardDialogActions(event, h.activateRunForEachHistorySelection, h.closeRunForEachHistoryPicker, nil) {
		return false
	}

	if st.Focus == 0 {
		onChange := func() {
			h.syncRunForEachHistoryPickerRanks()
			st.Selected = 0
			dialog.EnsureRunForEachHistoryPickerListScroll(st, h.RunForEachHistoryPickerListRows())
		}
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, h.RunForEachHistoryPickerQueryWidth(), onChange)
		if scrollquery.HandleKey(h.keysDialogInput, event, true, edit) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.closeRunForEachHistoryPicker()
	case tcell.KeyEnter:
		switch st.Focus {
		case 2:
			h.closeRunForEachHistoryPicker()
		default:
			h.activateRunForEachHistorySelection()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := dialog.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				dialog.EnsureRunForEachHistoryPickerListScroll(st, h.RunForEachHistoryPickerListRows())
			}
			break
		}
		if h.host.HandleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), h.RunForEachHistoryPickerListRows, func() {
			dialog.EnsureRunForEachHistoryPickerListScroll(st, h.RunForEachHistoryPickerListRows())
		}) {
			break
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if h.host.HandleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), h.RunForEachHistoryPickerListRows, func() {
			dialog.EnsureRunForEachHistoryPickerListScroll(st, h.RunForEachHistoryPickerListRows())
		}) {
			break
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if st.Focus == 0 {
			break
		}
		switch dialog.DialogButtonRune(event.Rune()) {
		case dialog.ButtonRuneOK:
			h.activateRunForEachHistorySelection()
		case dialog.ButtonRuneCancel:
			h.closeRunForEachHistoryPicker()
		case dialog.ButtonRuneToggle:
			switch st.Focus {
			case 1:
				h.activateRunForEachHistorySelection()
			case 2:
				h.closeRunForEachHistoryPicker()
			}
		}
	}
	return false
}
