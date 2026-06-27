package app

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) openCompareFilterDialog() {
	if a.model.ViewMode != ui.ViewCompare {
		return
	}
	focus := dialog.FocusForCompareFilter(a.model.CompareView.Filter)
	a.model.CompareFilterDialog = dialog.CompareFilterDialogState{
		Open:          true,
		Focus:         focus,
		OriginalFocus: focus,
	}
}

func (a *App) closeCompareFilterDialog() {
	a.model.CompareFilterDialog = dialog.CompareFilterDialogState{}
}

func (a *App) cancelCompareFilterDialog() {
	d := &a.model.CompareFilterDialog
	if f, ok := dialog.CompareFilterForFocus(d.OriginalFocus); ok {
		a.compareCtrl.SetFilter(f)
	}
	a.closeCompareFilterDialog()
}

func (a *App) confirmCompareFilter() {
	a.closeCompareFilterDialog()
}

// applyCompareFilterFocus applies the filter for the given focus index live
// (while the dialog is still open) so the compare list updates in real time.
func (a *App) applyCompareFilterFocus(focus int) {
	if f, ok := dialog.CompareFilterForFocus(focus); ok {
		a.compareCtrl.SetFilter(f)
	}
}

func (a *App) handleCompareFilterDialogKey(event *tcell.EventKey) {
	d := &a.model.CompareFilterDialog
	if !d.Open {
		return
	}
	if a.tryStandardDialogActions(event, a.confirmCompareFilter, a.cancelCompareFilterDialog, nil) {
		return
	}
	if event.Key() == tcell.KeyEsc {
		a.cancelCompareFilterDialog()
		return
	}
	if event.Key() == tcell.KeyEnter {
		if d.Focus == dialog.CompareFilterDialogCancelIndex() {
			a.cancelCompareFilterDialog()
		} else {
			a.confirmCompareFilter()
		}
		return
	}
	// Alt+letter shortcuts navigate to a filter option and apply live.
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		var target comparepkg.Filter
		found := true
		switch {
		case runeMatchesCaseFold(event.Rune(), 'a'):
			target = comparepkg.FilterAll
		case runeMatchesCaseFold(event.Rune(), 'e'):
			target = comparepkg.FilterEqual
		case runeMatchesCaseFold(event.Rune(), 'r'):
			target = comparepkg.FilterRelocated
		case runeMatchesCaseFold(event.Rune(), 'p'):
			target = comparepkg.FilterPrimaryOnly
		case runeMatchesCaseFold(event.Rune(), 's'):
			target = comparepkg.FilterSecondaryOnly
		case runeMatchesCaseFold(event.Rune(), 'd'):
			target = comparepkg.FilterContentDiff
		default:
			found = false
		}
		if found {
			newFocus := dialog.FocusForCompareFilter(target)
			d.Focus = newFocus
			a.applyCompareFilterFocus(newFocus)
			return
		}
	}
	if nf, ok := dialog.CompareFilterDialogMoveFocus(d.Focus, event.Key()); ok {
		d.Focus = nf
		// Apply live when navigating to a radio option (not a button).
		if nf < dialog.CompareFilterDialogOKIndex() {
			a.applyCompareFilterFocus(nf)
		}
	}
}
