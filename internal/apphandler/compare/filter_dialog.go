package compare

import (
	"unicode"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenFilterDialog opens the compare category-filter picker.
func (h *Handler) OpenFilterDialog() {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	focus := dialog.FocusForCompareFilter(h.model.CompareView.Filter)
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{
		Open:          true,
		Focus:         focus,
		OriginalFocus: focus,
	}
}

func (h *Handler) closeFilterDialog() {
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{}
}

func (h *Handler) cancelFilterDialog() {
	d := &h.model.CompareFilterDialog
	if f, ok := dialog.CompareFilterForFocus(d.OriginalFocus); ok {
		h.SetFilter(f)
	}
	h.closeFilterDialog()
}

func (h *Handler) confirmFilterDialog() {
	h.closeFilterDialog()
}

// applyFilterDialogFocus applies the filter for the given focus index live
// (while the dialog is still open) so the compare list updates in real time.
func (h *Handler) applyFilterDialogFocus(focus int) {
	if f, ok := dialog.CompareFilterForFocus(focus); ok {
		h.SetFilter(f)
	}
}

// HandleFilterDialogKey routes keys for the open category-filter dialog. No-op when the
// dialog is not open.
func (h *Handler) HandleFilterDialogKey(event *tcell.EventKey) {
	d := &h.model.CompareFilterDialog
	if !d.Open {
		return
	}
	if dialog.AltDialogOK(event) {
		h.confirmFilterDialog()
		return
	}
	if dialog.AltDialogCancel(event) {
		h.cancelFilterDialog()
		return
	}
	if event.Key() == tcell.KeyEsc {
		h.cancelFilterDialog()
		return
	}
	if event.Key() == tcell.KeyEnter {
		if d.Focus == dialog.CompareFilterDialogCancelIndex() {
			h.cancelFilterDialog()
		} else {
			h.confirmFilterDialog()
		}
		return
	}
	// Alt+letter shortcuts navigate to a filter option and apply live.
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		for _, row := range comparepkg.FilterDialogRadios() {
			if filterShortcutMatches(event.Rune(), row.Shortcut) {
				newFocus := dialog.FocusForCompareFilter(row.Filter)
				d.Focus = newFocus
				h.applyFilterDialogFocus(newFocus)
				return
			}
		}
	}
	if nf, ok := dialog.CompareFilterDialogMoveFocus(d.Focus, event.Key()); ok {
		d.Focus = nf
		// Apply live when navigating to a radio option (not a button).
		if nf < dialog.CompareFilterDialogOKIndex() {
			h.applyFilterDialogFocus(nf)
		}
	}
}

func filterShortcutMatches(got, want rune) bool {
	return got == want || unicode.ToLower(got) == unicode.ToLower(want)
}
