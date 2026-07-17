package dialog

import "github.com/gdamore/tcell/v2"

// ListDialogForm describes focus for list + OK + Cancel dialogs
// (0=list, 1=OK, 2=Cancel). When HideOK is true, focus never lands on OK
// (navigate/bookmark path picker shows Cancel only).
type ListDialogForm struct {
	HideOK bool
}

// ListIndex is the focus index of the filtered list / query field.
func (ListDialogForm) ListIndex() int { return 0 }

// OKIndex is the focus index of the OK button.
func (ListDialogForm) OKIndex() int { return 1 }

// CancelIndex is the focus index of the Cancel button.
func (ListDialogForm) CancelIndex() int { return 2 }

// MoveFocus applies Tab, Backtab, and arrow keys for list+OK+Cancel dialogs
// (Midnight Commander rules: Up from buttons returns to list; Down from OK
// moves to Cancel; Down on list is caller-specific). When handled is false,
// focus is unchanged and the caller should handle list motion.
func (f ListDialogForm) MoveFocus(focus int, key tcell.Key) (newFocus int, handled bool) {
	nf, ok := listOKCancelNavFocusKey(focus, key)
	if !ok {
		return focus, false
	}
	if f.HideOK && nf == f.OKIndex() {
		if key == tcell.KeyTab {
			nf = f.CancelIndex()
		} else {
			nf = f.ListIndex()
		}
	}
	return nf, true
}

// ListOKCancelNavFocusKey applies Tab, Backtab, arrow keys for a modal with
// focus indices 0=list, 1=OK, 2=Cancel.
func ListOKCancelNavFocusKey(focus int, key tcell.Key) (newFocus int, handled bool) {
	return ListDialogForm{}.MoveFocus(focus, key)
}

func listOKCancelNavFocusKey(focus int, key tcell.Key) (newFocus int, handled bool) {
	switch key {
	case tcell.KeyTab:
		return (focus + 1) % 3, true
	case tcell.KeyBacktab:
		return (focus + 2) % 3, true
	case tcell.KeyLeft:
		switch focus {
		case 1:
			return 0, true
		case 2:
			return 1, true
		default:
			return focus, false
		}
	case tcell.KeyRight:
		if focus == 1 {
			return 2, true
		}
		return focus, false
	case tcell.KeyUp:
		if focus != 0 {
			return 0, true
		}
		return focus, false
	case tcell.KeyDown:
		if focus == 1 {
			return 2, true
		}
		return focus, false
	default:
		return focus, false
	}
}

// FindDialogNavFocusKey applies Tab/arrow navigation for find dialog focus.
// When hasSelectionsCheckbox is false: 0=list+filter, 1=only-directories, 2=only-files, 3=stay-on-volume, 4=OK, 5=Cancel.
// When true: 0=list+filter, 1=only-directories, 2=only-files, 3=stay-on-volume, 4=selections, 5=OK, 6=Cancel.
func FindDialogNavFocusKey(focus int, hasSelectionsCheckbox bool, key tcell.Key) (newFocus int, handled bool) {
	numContent := 4
	if hasSelectionsCheckbox {
		numContent = 5
	}
	form := NewDialogLinearForm(numContent)
	okFocus := form.OKIndex()
	cancelFocus := form.CancelIndex()

	switch key {
	case tcell.KeyTab, tcell.KeyBacktab:
		return form.MoveFocus(focus, key)
	case tcell.KeyLeft:
		switch focus {
		case 2:
			return 1, true
		case 3:
			return 2, true
		case okFocus:
			return okFocus - 1, true
		case cancelFocus:
			return okFocus, true
		default:
			return focus, false
		}
	case tcell.KeyRight:
		switch focus {
		case 1:
			return 2, true
		case 2:
			return 3, true
		case okFocus:
			return cancelFocus, true
		default:
			return focus, false
		}
	case tcell.KeyUp:
		if focus == 0 {
			return focus, false
		}
		if focus == okFocus || focus == cancelFocus {
			return okFocus - 1, true
		}
		if focus == 4 && hasSelectionsCheckbox {
			return 3, true
		}
		if focus == 1 || focus == 2 || focus == 3 {
			return 0, true
		}
		return focus - 1, true
	case tcell.KeyDown:
		if focus == 0 || focus == cancelFocus {
			return focus, false
		}
		if focus == 1 || focus == 2 || focus == 3 {
			return 4, true // OK, or selections row when hasSelectionsCheckbox
		}
		return focus + 1, true
	default:
		return focus, false
	}
}

// ListClampedSelectionDelta returns selected+delta clamped to [0, rankedLen-1].
// If rankedLen <= 0 it returns 0.
func ListClampedSelectionDelta(selected, rankedLen, delta int) int {
	if rankedLen <= 0 {
		return 0
	}
	maxSel := rankedLen - 1
	v := selected + delta
	if v < 0 {
		return 0
	}
	if v > maxSel {
		return maxSel
	}
	return v
}

// ListNavKeySelection computes the new Selected index for the list-navigation
// keys shared by fuzzy-picker style dialogs (Up/Down by one row, PgUp/PgDn by
// pageSize rows, Ctrl+Home/Ctrl+End to the first/last row). pageSize is
// clamped to at least 1. ok is false when rankedLen <= 0 or key isn't one of
// these keys (or Home/End without Ctrl) — the caller should leave selected
// unchanged and fall through to its own handling.
func ListNavKeySelection(key tcell.Key, mods tcell.ModMask, selected, rankedLen, pageSize int) (newSelected int, ok bool) {
	if rankedLen <= 0 {
		return selected, false
	}
	switch key {
	case tcell.KeyUp:
		return ListClampedSelectionDelta(selected, rankedLen, -1), true
	case tcell.KeyDown:
		return ListClampedSelectionDelta(selected, rankedLen, 1), true
	case tcell.KeyPgUp:
		return ListClampedSelectionDelta(selected, rankedLen, -max(1, pageSize)), true
	case tcell.KeyPgDn:
		return ListClampedSelectionDelta(selected, rankedLen, max(1, pageSize)), true
	case tcell.KeyHome:
		if mods&tcell.ModCtrl != 0 {
			return 0, true
		}
	case tcell.KeyEnd:
		if mods&tcell.ModCtrl != 0 {
			return rankedLen - 1, true
		}
	}
	return selected, false
}
