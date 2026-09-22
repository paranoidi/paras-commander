package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// MoveFocus handles the sort dialog's two-column radio grid: built-in radios on the left, meta
// radios on the right sharing their rows. Right/Left move focus between the columns on the same
// row (focus only; Space/Enter picks); Down from the bottom of either column goes to the first
// checkbox, and Up from that checkbox returns to the last built-in radio. Other keys return false
// so the linear form's standard navigation applies.
func (s SortDialogState) MoveFocus(focus int, key tcell.Key) (int, bool) {
	rows := len(panel.SortDialogRadios())
	n := s.MetaCount()
	cb := s.CheckboxFocus()
	isMeta := focus >= rows && focus < rows+n
	switch key {
	case tcell.KeyRight:
		if focus >= 0 && focus < rows {
			if focus < n {
				return rows + focus, true
			}
			return focus, true
		}
	case tcell.KeyLeft:
		if isMeta {
			return focus - rows, true
		}
	case tcell.KeyDown:
		if focus == rows-1 || (isMeta && focus == rows+n-1) {
			return cb, true
		}
	case tcell.KeyUp:
		if focus == cb {
			return rows - 1, true
		}
		if focus == rows {
			return focus, true
		}
	}
	return 0, false
}
