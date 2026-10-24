package dialog

import "github.com/gdamore/tcell/v2"

// ListOKCancelNavFocusKey applies Tab, Backtab and arrow keys for a modal with focus indices
// 0=list, 1=OK, 2=Cancel (Midnight Commander rules: Up from buttons returns to list; Down from
// OK moves to Cancel; Down on the list is caller-specific). When handled is false, focus is
// unchanged and the caller should handle list motion.
func ListOKCancelNavFocusKey(focus int, key tcell.Key) (newFocus int, handled bool) {
	return listOKCancelNavFocusKey(focus, key)
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
// When hasSelectionsCheckbox is false: 0=list+filter, 1=max depth, 2=only-directories, 3=only-files, 4=stay-on-volume, 5=include-hidden, 6=OK, 7=Cancel.
// When true: 0=list+filter, 1=max depth, 2=only-directories, 3=only-files, 4=stay-on-volume, 5=include-hidden, 6=selections, 7=OK, 8=Cancel.
// Row 1 (focus 2-5) uses Left/Right; Up/Down moves between list, depth, row 1, optional selections, and buttons.
func FindDialogNavFocusKey(focus int, hasSelectionsCheckbox bool, key tcell.Key) (newFocus int, handled bool) {
	numContent := 6
	if hasSelectionsCheckbox {
		numContent = 7
	}
	form := NewDialogLinearForm(numContent)
	okFocus := form.OKIndex()
	cancelFocus := form.CancelIndex()
	const (
		depthFocus  = 1
		onlyDirs    = 2
		volumeFocus = 5
	)
	selectionsFocus := 6
	inRow := focus >= onlyDirs && focus <= volumeFocus

	switch key {
	case tcell.KeyTab, tcell.KeyBacktab:
		return form.MoveFocus(focus, key)
	case tcell.KeyLeft:
		switch {
		case focus == depthFocus:
			return 0, true
		case focus > onlyDirs && focus <= volumeFocus:
			return focus - 1, true
		case focus == okFocus:
			return okFocus - 1, true
		case focus == cancelFocus:
			return okFocus, true
		default:
			return focus, false
		}
	case tcell.KeyRight:
		switch {
		case focus >= onlyDirs && focus < volumeFocus:
			return focus + 1, true
		case focus == okFocus:
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
		if hasSelectionsCheckbox && focus == selectionsFocus {
			return volumeFocus, true
		}
		if inRow {
			return depthFocus, true
		}
		return focus - 1, true
	case tcell.KeyDown:
		if focus == 0 || focus == cancelFocus {
			return focus, false
		}
		if inRow {
			if hasSelectionsCheckbox {
				return selectionsFocus, true
			}
			return okFocus, true
		}
		return focus + 1, true
	default:
		return focus, false
	}
}
