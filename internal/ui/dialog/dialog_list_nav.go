package dialog

import "github.com/gdamore/tcell/v2"

// ListOKCancelNavFocusKey applies Tab, Backtab, arrow keys for a modal with
// focus indices 0=list, 1=OK, 2=Cancel (Midnight Commander rules: Up from buttons
// returns to list; Down from OK moves to Cancel; Down on list is caller-specific).
// When handled is false, focus is unchanged and the caller should handle list
// motion (e.g. wrap vs clamp) or leave the key unhandled.
func ListOKCancelNavFocusKey(focus int, key tcell.Key) (newFocus int, handled bool) {
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
// When hasSelectionsCheckbox is false: 0=list+filter, 1=volume, 2=OK, 3=Cancel.
// When true: 0=list+filter, 1=volume, 2=selections, 3=OK, 4=Cancel.
func FindDialogNavFocusKey(focus int, hasSelectionsCheckbox bool, key tcell.Key) (newFocus int, handled bool) {
	maxFocus := 3
	if hasSelectionsCheckbox {
		maxFocus = 4
	}
	mod := maxFocus + 1
	switch key {
	case tcell.KeyTab:
		return (focus + 1) % mod, true
	case tcell.KeyBacktab:
		return (focus + mod - 1) % mod, true
	case tcell.KeyLeft:
		okFocus := 2
		cancelFocus := 3
		if hasSelectionsCheckbox {
			okFocus = 3
			cancelFocus = 4
		}
		switch focus {
		case okFocus:
			return okFocus - 1, true
		case cancelFocus:
			return okFocus, true
		default:
			return focus, false
		}
	case tcell.KeyRight:
		okFocus := 2
		cancelFocus := 3
		if hasSelectionsCheckbox {
			okFocus = 3
			cancelFocus = 4
		}
		switch focus {
		case okFocus - 1:
			return okFocus, true
		case okFocus:
			return cancelFocus, true
		default:
			return focus, false
		}
	case tcell.KeyUp:
		if focus == 0 {
			return focus, false
		}
		return focus - 1, true
	case tcell.KeyDown:
		if focus == 0 {
			return focus, false
		}
		if focus == maxFocus {
			return focus, false
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
