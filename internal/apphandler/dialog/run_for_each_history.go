package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
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
	dialog.EnsureRunForEachHistoryPickerListScroll(st, h.pickerListRows())
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

// runForEachHistoryPickerBinding binds the open command-history picker to handlePickerKey and
// applyPickerEvent.
func (h *Handler) runForEachHistoryPickerBinding() pickerBinding {
	st := &h.model.FileDialog.RunForEachHistoryPicker
	return pickerBinding{
		focus: &st.Focus, query: &st.Query, cursor: &st.QueryCursor, scroll: &st.QueryScroll,
		selected: &st.Selected, offset: &st.ListScroll, count: func() int { return len(st.Ranked) },
		resync: h.syncRunForEachHistoryPickerRanks, activate: h.activateRunForEachHistorySelection, closePicker: h.closeRunForEachHistoryPicker,
	}
}
