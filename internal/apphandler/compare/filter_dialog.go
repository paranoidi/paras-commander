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
	cur := h.model.CompareView.Filter
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{
		Open:   true,
		Focus:  dialog.FocusForCompareFilter(cur),
		Filter: cur,
	}
}

func (h *Handler) closeFilterDialog() {
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{}
}

func (h *Handler) confirmFilterDialog() {
	d := &h.model.CompareFilterDialog
	h.SetFilter(d.Filter)
	h.closeFilterDialog()
}

// selectFilterDialogRadio sets the pending radio selection for focus without
// applying it to the view (Space/Enter/Alt mnemonic).
func (h *Handler) selectFilterDialogRadio(focus int) {
	d := &h.model.CompareFilterDialog
	if f, ok := dialog.CompareFilterForFocus(focus); ok {
		d.Filter = f
		d.Focus = focus
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
		h.closeFilterDialog()
		return
	}
	if event.Key() == tcell.KeyEsc {
		h.closeFilterDialog()
		return
	}
	if event.Key() == tcell.KeyEnter {
		if d.Focus == dialog.CompareFilterDialogCancelIndex() {
			h.closeFilterDialog()
		} else if d.Focus < dialog.CompareFilterDialogOKIndex() {
			h.selectFilterDialogRadio(d.Focus)
		} else {
			h.confirmFilterDialog()
		}
		return
	}
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone && event.Rune() == ' ' &&
		d.Focus < dialog.CompareFilterDialogOKIndex() {
		h.selectFilterDialogRadio(d.Focus)
		return
	}
	// Alt+letter selects a filter option (pending until OK), like sort-dialog mnemonics.
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		for _, row := range comparepkg.FilterDialogRadios() {
			if filterShortcutMatches(event.Rune(), row.Shortcut) {
				h.selectFilterDialogRadio(dialog.FocusForCompareFilter(row.Filter))
				return
			}
		}
	}
	if nf, ok := dialog.CompareFilterDialogMoveFocus(d.Focus, event.Key()); ok {
		d.Focus = nf
	}
}

func filterShortcutMatches(got, want rune) bool {
	return got == want || unicode.ToLower(got) == unicode.ToLower(want)
}
