package dialog

import "github.com/gdamore/tcell/v2"

const (
	configDialogFocusHorizontalSplit = 2
	configDialogFocusScrollFirst     = 3
	configDialogFocusScrollLast      = 8
	configDialogFocusListingFirst    = 9
	configDialogFocusListingLast     = 11
	configDialogFocusViewLast        = configDialogFocusHorizontalSplit
	configDialogFocusScrollCenter    = 7
	configDialogFocusSplitFirst      = 12
	configDialogFocusSplitLast       = 14
	configDialogFocusOK              = 15
	configDialogFocusCancel          = 16
)

// ConfigDialogSplitIndex maps a focus index to a carousel-split input (0..2).
func ConfigDialogSplitIndex(focus int) (int, bool) {
	if focus < configDialogFocusSplitFirst || focus > configDialogFocusSplitLast {
		return 0, false
	}
	return focus - configDialogFocusSplitFirst, true
}

// ConfigDialogScrollModeFocus returns the focus index for scroll-mode row (0..2).
func ConfigDialogScrollModeFocus(row int) int {
	return configDialogFocusScrollFirst + 2*row
}

// ConfigDialogScrollbarFocus returns the focus index for scrollbar-style row (0..2).
func ConfigDialogScrollbarFocus(row int) int {
	return configDialogFocusScrollFirst + 1 + 2*row
}

// ConfigDialogScrollModeIndex maps a scroll-section focus index to a scroll-mode radio row.
func ConfigDialogScrollModeIndex(focus int) (int, bool) {
	if focus < configDialogFocusScrollFirst || focus > configDialogFocusScrollLast || (focus-configDialogFocusScrollFirst)%2 != 0 {
		return 0, false
	}
	return (focus - configDialogFocusScrollFirst) / 2, true
}

// ConfigDialogScrollbarIndex maps a scroll-section focus index to a scrollbar-style radio row.
func ConfigDialogScrollbarIndex(focus int) (int, bool) {
	if focus < configDialogFocusScrollFirst+1 || focus > configDialogFocusScrollLast || (focus-configDialogFocusScrollFirst)%2 != 1 {
		return 0, false
	}
	return (focus - configDialogFocusScrollFirst - 1) / 2, true
}

// ConfigDialogInScrollSection reports whether focus is on an interleaved scroll-mode / scrollbar radio.
func ConfigDialogInScrollSection(focus int) bool {
	return focus >= configDialogFocusScrollFirst && focus <= configDialogFocusScrollLast
}

// ConfigDialogMoveScrollFocus applies column-aware Up/Down/Left/Right within the scroll radio block
// and listing-format Up from the first row back to scroll-mode Center.
func ConfigDialogMoveScrollFocus(focus int, key tcell.Key) (int, bool) {
	// The split inputs share one row, so Up from the buttons lands on its first input.
	if key == tcell.KeyUp && (focus == configDialogFocusOK || focus == configDialogFocusCancel) {
		return configDialogFocusSplitFirst, true
	}
	if _, ok := ConfigDialogSplitIndex(focus); ok {
		switch key {
		case tcell.KeyUp:
			return configDialogFocusListingLast, true
		case tcell.KeyDown:
			return configDialogFocusOK, true
		}
		return focus, false
	}
	if focus >= configDialogFocusListingFirst && focus <= configDialogFocusListingLast {
		if key == tcell.KeyUp && focus == configDialogFocusListingFirst {
			return configDialogFocusScrollCenter, true
		}
		if key == tcell.KeyDown && focus == configDialogFocusListingLast {
			return configDialogFocusSplitFirst, true
		}
		return focus, false
	}
	if !ConfigDialogInScrollSection(focus) {
		return focus, false
	}
	// Scroll block: mode radios sit on odd offsets from First, scrollbar radios on the next cell.
	const first, last = configDialogFocusScrollFirst, configDialogFocusScrollLast
	onModeCol := (focus-first)%2 == 0
	switch key {
	case tcell.KeyRight:
		if onModeCol {
			return focus + 1, true
		}
		return focus, true
	case tcell.KeyLeft:
		if !onModeCol {
			return focus - 1, true
		}
		return focus, true
	case tcell.KeyDown:
		if focus+2 <= last {
			return focus + 2, true
		}
		return configDialogFocusListingFirst, true
	case tcell.KeyUp:
		if focus-2 >= first {
			return focus - 2, true
		}
		return configDialogFocusViewLast, true
	default:
		return focus, false
	}
}
