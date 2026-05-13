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
