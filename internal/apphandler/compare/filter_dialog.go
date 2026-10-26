package compare

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// OpenFilterDialog opens the compare category-filter picker.
func (h *Handler) OpenFilterDialog() {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	cur := h.model.CompareView.Filter
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{
		Open:   true,
		Focus:  (dialog.CompareFilterDialogState{}).FocusForFilter(cur),
		Filter: cur,
	}
}

// OpenSelectDialog opens the select-by-kind picker: OK adds (selected) or removes (!selected)
// the focused column's path of every visible matching row in that side's panel selection.
func (h *Handler) OpenSelectDialog(selected bool) {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	mode := dialog.CompareFilterModeSelect
	if !selected {
		mode = dialog.CompareFilterModeUnselect
	}
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{
		Open: true, Mode: mode, Filter: comparepkg.FilterNone,
		FocusSecondary: h.model.CompareView.FocusColumn == ui.CompareColumnSecondary,
	}
}

func (h *Handler) closeFilterDialog() {
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{}
}

func (h *Handler) confirmFilterDialog() {
	d := &h.model.CompareFilterDialog
	if d.Mode != dialog.CompareFilterModeCategory {
		f, selected := d.Filter, d.Mode == dialog.CompareFilterModeSelect
		h.closeFilterDialog()
		h.selectMatching(f, selected)
		return
	}
	h.SetFilter(d.Filter)
	h.closeFilterDialog()
}

// sideName names a panel's compare column in messages.
func (h *Handler) sideName(panelID int) string {
	if panelID == h.model.CompareView.PanelID(true) {
		return "right"
	}
	return "left"
}

// selectMatching adds (selected) or removes the focused side's path of every visible row of
// kind f in that panel's selection and reports how many changed per side. Rows without a
// focused-side entry (only on the other side) use the other side's path and panel.
func (h *Handler) selectMatching(f comparepkg.Filter, selected bool) {
	var n [2]int
	focusSecondary := h.model.CompareView.FocusColumn == ui.CompareColumnSecondary
	for _, row := range h.FilteredRows() {
		if !comparepkg.FilterMatches(f, row.Kind) {
			continue
		}
		abs, id, ok, err := h.sideOf(row, focusSecondary)
		if err == nil && !ok {
			abs, id, ok, err = h.sideOf(row, !focusSecondary)
		}
		if err != nil || !ok {
			continue
		}
		// ponytail: AddSelection lstats unlisted paths on the UI thread, fine for local
		// roots; add a batched file-only add if huge results lag.
		if h.host.SetPanelSelection(id, abs.String(), selected) {
			n[id]++
		}
	}
	verb := "select"
	if !selected {
		verb = "unselect"
	}
	var parts []string
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if n[id] == 0 {
			continue
		}
		if len(parts) == 0 {
			parts = append(parts, fmt.Sprintf("%d files on %s side", n[id], h.sideName(id)))
		} else {
			parts = append(parts, fmt.Sprintf("%d on %s side", n[id], h.sideName(id)))
		}
	}
	if len(parts) == 0 {
		h.host.SetTransientMessage("Nothing to "+verb, ui.MessageUrgencyInfo)
		return
	}
	h.host.SetTransientMessage(strings.ToUpper(verb[:1])+verb[1:]+"ed "+strings.Join(parts, ", "), ui.MessageUrgencyInfo)
}

// InvertColumnSelection toggles the focused column's path of every visible row in that
// panel's selection.
func (h *Handler) InvertColumnSelection() {
	n := 0
	panelID := h.model.CompareView.FocusedPanelID()
	for _, row := range h.FilteredRows() {
		abs, id, ok, err := h.focusedSide(row)
		if err != nil || !ok {
			continue
		}
		h.host.TogglePanelSelection(id, abs.String())
		n++
	}
	h.host.SetTransientMessage(fmt.Sprintf("Inverted %d files on %s side", n, h.sideName(panelID)), ui.MessageUrgencyInfo)
}

// ClearColumnSelection clears the focused column's panel selection.
func (h *Handler) ClearColumnSelection() {
	panelID := h.model.CompareView.FocusedPanelID()
	h.host.ClearPanelSelection(panelID)
	h.host.SetTransientMessage("Cleared selection on "+h.sideName(panelID)+" side", ui.MessageUrgencyInfo)
}

// HandleFilterDialogKey routes keys for the open category-filter dialog. No-op when the
// dialog is not open. Toggling a row only changes the pending filter; OK applies it.
func (h *Handler) HandleFilterDialogKey(event *tcell.EventKey) {
	d := &h.model.CompareFilterDialog
	if !d.Open {
		return
	}
	if ev, ok := dlg.HandleKey(dialog.CompareFilterView(*d), &d.Focus, event); ok {
		h.ApplyFilterDialogEvent(ev)
	}
}

// ApplyFilterDialogEvent acts on a widget event of the open category-filter dialog (key or click).
func (h *Handler) ApplyFilterDialogEvent(ev dlg.Event) {
	d := &h.model.CompareFilterDialog
	switch ev.ID {
	case dlg.OK:
		h.confirmFilterDialog()
	case dlg.Cancel:
		h.closeFilterDialog()
	default:
		d.Apply(ev)
	}
}

// FilterDialogOK applies the category-filter dialog.
func (h *Handler) FilterDialogOK() { h.confirmFilterDialog() }

// FilterDialogCancel closes the category-filter dialog.
func (h *Handler) FilterDialogCancel() { h.closeFilterDialog() }
