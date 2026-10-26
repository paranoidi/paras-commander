package jobs

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// appendJobOperation records the Operations row for ev: a success for a progress sample's file,
// a failure (with its error) for EventItemFailed. ui.AppendJobOperation dedupes and replaces rows.
func (h *Handler) appendJobOperation(ev jobs.Event) {
	var op ui.JobOperation
	switch ev.Type {
	case jobs.EventProgress:
		if ev.CurrentPath == "" {
			return
		}
		op.Label = jobbridge.ActivityDetailLabel(h.state.ActiveJob(), ev)
	case jobs.EventItemFailed:
		if ev.FailedItem == nil {
			return
		}
		op = ui.JobOperation{
			Label: jobbridge.ActivityDetailLabel(h.state.ActiveJob(), jobs.Event{
				JobID:           ev.JobID,
				CurrentPath:     ev.FailedItem.Item.Src.String(),
				CurrentDestPath: ev.FailedItem.Item.Dst.String(),
			}),
			Error:  ev.FailedItem.Error,
			Failed: true,
		}
	default:
		return
	}
	if h.model.JobOperations == nil {
		h.model.JobOperations = make(map[string][]ui.JobOperation)
	}
	h.model.JobOperations[ev.JobID] = ui.AppendJobOperation(h.model.JobOperations[ev.JobID], op)
}

// visibleOperations is the selected job's Operations rows after the view's filters.
func (h *Handler) visibleOperations() []ui.JobOperationRow {
	ops := h.model.JobOperations[h.selectedJobEntry().ID]
	return ui.VisibleJobOperations(ops, h.model.JobsView, h.model.JobsFilterCaseInsensitive)
}

// operationsFocused reports whether the Operations pane has focus.
func (h *Handler) operationsFocused() bool {
	order, _ := h.jobsPaneLayout()
	fp := h.model.JobsView.FocusPane
	return fp >= 0 && fp < len(order) && order[fp] == ui.JobsPaneOperations
}

// plainRune reports an unmodified (or shifted) printable key, the keys that type into the
// Operations fuzzy filter.
func plainRune(event *tcell.EventKey) bool {
	return event.Key() == tcell.KeyRune && event.Modifiers()&^tcell.ModShift == 0
}

// handleOperationsFilterKey edits an open Operations fuzzy filter (query set or editing, pane
// focused): printable runes type, Backspace deletes, Ctrl+Backspace and Ctrl+L clear, Esc closes
// the filter. It reports whether the key was consumed; scrolling and actions fall through.
func (h *Handler) handleOperationsFilterKey(event *tcell.EventKey) bool {
	f := &h.model.JobsView.OpsFilter
	if f.Query == "" && !f.Editing {
		return false
	}
	switch event.Key() {
	case tcell.KeyEsc:
		h.setOperationsQuery(quickfilter.Filter{})
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if event.Modifiers()&tcell.ModCtrl != 0 {
			h.clearOperationsQuery()
			return true
		}
		if next, changed := f.Backspace(); changed { // an empty query closes the filter
			f.Query = next
			h.model.JobsView.OpsScroll = 0
		}
	case tcell.KeyCtrlL:
		h.clearOperationsQuery()
	case tcell.KeyRune:
		if !plainRune(event) {
			return false
		}
		h.typeOperationsRune(event.Rune())
	default:
		return false
	}
	return true
}

// typeOperationsRune types r into the Operations fuzzy filter, opening it if needed.
func (h *Handler) typeOperationsRune(r rune) {
	f := &h.model.JobsView.OpsFilter
	f.Query = f.InsertRune(r)
	h.model.JobsView.OpsScroll = 0
}

// clearOperationsQuery empties the fuzzy query while the filter stays open for typing.
func (h *Handler) clearOperationsQuery() {
	h.setOperationsQuery(quickfilter.Filter{Editing: true})
}

func (h *Handler) setOperationsQuery(f quickfilter.Filter) {
	h.model.JobsView.OpsFilter = f
	h.model.JobsView.OpsScroll = 0
}

// ResetOperationsFilter shows every result and drops the fuzzy query.
func (h *Handler) ResetOperationsFilter() {
	h.model.JobsView.OpsResultHidden = dialog.JobResultNone
	h.setOperationsQuery(quickfilter.Filter{})
}

// OpenResultFilterDialog opens the Operations result picker on the current result filter.
func (h *Handler) OpenResultFilterDialog() {
	h.model.JobsResultFilterDialog = dialog.JobsResultFilterDialogState{
		Open:   true,
		Filter: h.model.JobsView.OpsResultShown(),
	}
}

// HandleResultFilterDialogKey feeds a key to the open result picker.
func (h *Handler) HandleResultFilterDialogKey(event *tcell.EventKey) {
	d := &h.model.JobsResultFilterDialog
	if ev, ok := dlg.HandleKey(dialog.JobsResultFilterView(*d), &d.Focus, event); ok {
		h.ApplyResultFilterDialogEvent(ev)
	}
}

// ApplyResultFilterDialogEvent acts on a widget event of the result picker (key or click).
func (h *Handler) ApplyResultFilterDialogEvent(ev dlg.Event) {
	switch ev.ID {
	case dlg.OK:
		h.ResultFilterDialogOK()
	case dlg.Cancel:
		h.ResultFilterDialogCancel()
	default:
		h.model.JobsResultFilterDialog.Apply(ev)
	}
}

// ResultFilterDialogOK applies the picked results to the Operations pane.
func (h *Handler) ResultFilterDialogOK() {
	h.model.JobsView.OpsResultHidden = dialog.JobResultAll &^ h.model.JobsResultFilterDialog.Filter
	h.model.JobsView.OpsScroll = 0
	h.ResultFilterDialogCancel()
}

// ResultFilterDialogCancel closes the result picker without applying it.
func (h *Handler) ResultFilterDialogCancel() {
	h.model.JobsResultFilterDialog = dialog.JobsResultFilterDialogState{}
}
