package dialog

import "github.com/gdamore/tcell/v2"

const (
	configDialogFocusHorizontalSplit = 3
	configDialogFocusScrollFirst     = 4
	configDialogFocusScrollLast      = 9
	configDialogFocusListingFirst    = 10
	configDialogFocusListingLast     = 12
	configDialogFocusViewLast        = configDialogFocusHorizontalSplit
	configDialogFocusScrollCenter    = 8
	configDialogFocusOK              = 13
	configDialogFocusCancel          = 14
)

// ConfigDialogScrollModeFocus returns the focus index for scroll-mode row (0..2).
func ConfigDialogScrollModeFocus(row int) int {
	return configDialogFocusScrollFirst + 2*row
}

// ConfigDialogScrollbarFocus returns the focus index for scrollbar-style row (0..2).
func ConfigDialogScrollbarFocus(row int) int {
	return 5 + 2*row
}

// ConfigDialogScrollModeIndex maps a scroll-section focus index to a scroll-mode radio row.
func ConfigDialogScrollModeIndex(focus int) (int, bool) {
	if focus < configDialogFocusScrollFirst || focus > configDialogFocusScrollLast || focus%2 != 0 {
		return 0, false
	}
	return (focus - configDialogFocusScrollFirst) / 2, true
}

// ConfigDialogScrollbarIndex maps a scroll-section focus index to a scrollbar-style radio row.
func ConfigDialogScrollbarIndex(focus int) (int, bool) {
	switch focus {
	case 5, 7, 9:
		return (focus - 5) / 2, true
	default:
		return 0, false
	}
}

// ConfigDialogInScrollSection reports whether focus is on an interleaved scroll-mode / scrollbar radio.
func ConfigDialogInScrollSection(focus int) bool {
	return focus >= configDialogFocusScrollFirst && focus <= configDialogFocusScrollLast
}

// ConfigDialogMoveScrollFocus applies column-aware Up/Down/Left/Right within the scroll radio block
// and listing-format Up from the first row back to scroll-mode Center.
func ConfigDialogMoveScrollFocus(focus int, key tcell.Key) (int, bool) {
	if focus >= configDialogFocusListingFirst && focus <= configDialogFocusListingLast {
		if key == tcell.KeyUp && focus == configDialogFocusListingFirst {
			return configDialogFocusScrollCenter, true
		}
		return focus, false
	}
	if !ConfigDialogInScrollSection(focus) {
		return focus, false
	}
	switch key {
	case tcell.KeyRight:
		switch focus {
		case 4:
			return 5, true
		case 6:
			return 7, true
		case 8:
			return 9, true
		default:
			return focus, true
		}
	case tcell.KeyLeft:
		switch focus {
		case 5:
			return 4, true
		case 7:
			return 6, true
		case 9:
			return 8, true
		default:
			return focus, true
		}
	case tcell.KeyDown:
		switch focus {
		case 4:
			return 6, true
		case 6:
			return 8, true
		case 8:
			return configDialogFocusListingFirst, true
		case 5:
			return 7, true
		case 7:
			return 9, true
		case 9:
			return configDialogFocusListingFirst, true
		default:
			return focus, false
		}
	case tcell.KeyUp:
		switch focus {
		case 4, 5:
			return configDialogFocusViewLast, true
		case 6:
			return 4, true
		case 7:
			return 5, true
		case 8:
			return 6, true
		case 9:
			return 7, true
		default:
			return focus, false
		}
	default:
		return focus, false
	}
}
