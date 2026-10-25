package dialog

import "github.com/gdamore/tcell/v2"

// DialogTrailingButtonsForm describes focus indices for a dialog with N content rows
// (indices 0..NumContent-1) followed by NumTrailingButtons in order: OK, optional middle, Cancel.
// NumTrailingButtons must be 2 or 3.
// When SegmentStarts has 2+ entries, Tab/Backtab jump to the first element of the
// next/previous segment (wrapping around) instead of moving one step at a time.
// SegmentStarts must be sorted ascending; the last entry should equal OKIndex().
type DialogTrailingButtonsForm struct {
	NumContent         int
	NumTrailingButtons int
	SegmentStarts      []int
}

// NewDialogTrailingButtonsForm returns a trailing-button layout. numTrailingButtons must be 2 or 3.
func NewDialogTrailingButtonsForm(numContent, numTrailingButtons int) DialogTrailingButtonsForm {
	if numTrailingButtons != 2 && numTrailingButtons != 3 {
		numTrailingButtons = 2
	}
	return DialogTrailingButtonsForm{NumContent: numContent, NumTrailingButtons: numTrailingButtons}
}

// WithSegments returns a copy of the form with segment-jump Tab/Backtab enabled.
// starts must be sorted ascending; the last value must equal OKIndex().
func (d DialogTrailingButtonsForm) WithSegments(starts ...int) DialogTrailingButtonsForm {
	d.SegmentStarts = starts
	return d
}

// segmentOf returns which segment focus currently belongs to (0-indexed).
func (d DialogTrailingButtonsForm) segmentOf(focus int) int {
	seg := 0
	for i, s := range d.SegmentStarts {
		if focus >= s {
			seg = i
		}
	}
	return seg
}

// TotalFocus is the number of focus positions (content rows + trailing buttons).
func (d DialogTrailingButtonsForm) TotalFocus() int {
	return d.NumContent + d.NumTrailingButtons
}

// OKIndex is the focus index of the OK button.
func (d DialogTrailingButtonsForm) OKIndex() int {
	return d.NumContent
}

// MiddleButtonIndex is the focus index between OK and Cancel when NumTrailingButtons == 3; otherwise -1.
func (d DialogTrailingButtonsForm) MiddleButtonIndex() int {
	if d.NumTrailingButtons < 3 {
		return -1
	}
	return d.NumContent + 1
}

// CancelIndex is the focus index of the Cancel button.
func (d DialogTrailingButtonsForm) CancelIndex() int {
	return d.NumContent + d.NumTrailingButtons - 1
}

// Tab advances focus with wrap (Tab / Shift+Tab convenience), 0..TotalFocus-1.
// When SegmentStarts is set, jumps to the first element of the next segment (with wrap).
func (d DialogTrailingButtonsForm) Tab(focus int) int {
	if len(d.SegmentStarts) >= 2 {
		n := len(d.SegmentStarts)
		return d.SegmentStarts[(d.segmentOf(focus)+1)%n]
	}
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + 1) % n
}

// Backtab moves focus backward with wrap.
// When SegmentStarts is set, jumps to the first element of the previous segment (with wrap).
func (d DialogTrailingButtonsForm) Backtab(focus int) int {
	if len(d.SegmentStarts) >= 2 {
		n := len(d.SegmentStarts)
		return d.SegmentStarts[(d.segmentOf(focus)-1+n)%n]
	}
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + n - 1) % n
}

// Down moves focus down per docs/agents/dialogs.md: within content only until last content, then to OK;
// on trailing buttons, Down does nothing.
func (d DialogTrailingButtonsForm) Down(focus int) int {
	if d.NumContent <= 0 {
		return focus
	}
	lastContent := d.NumContent - 1
	if focus < lastContent {
		return focus + 1
	}
	if focus == lastContent {
		return d.OKIndex()
	}
	return focus
}

// Up moves focus up per docs/agents/dialogs.md: from any trailing button to last content; within content without wrap from 0.
func (d DialogTrailingButtonsForm) Up(focus int) int {
	if focus >= d.OKIndex() {
		if d.NumContent <= 0 {
			return focus
		}
		return d.NumContent - 1
	}
	if focus > 0 {
		return focus - 1
	}
	return 0
}

// Left moves focus along the trailing button strip only (OK <-> Cancel or OK <-> middle <-> Cancel).
func (d DialogTrailingButtonsForm) Left(focus int) int {
	if d.NumTrailingButtons == 2 {
		if focus == d.CancelIndex() {
			return d.OKIndex()
		}
		return focus
	}
	switch focus {
	case d.MiddleButtonIndex():
		return d.OKIndex()
	case d.CancelIndex():
		return d.MiddleButtonIndex()
	default:
		return focus
	}
}

// Right moves focus along the trailing button strip only.
func (d DialogTrailingButtonsForm) Right(focus int) int {
	if d.NumTrailingButtons == 2 {
		if focus == d.OKIndex() {
			return d.CancelIndex()
		}
		return focus
	}
	switch focus {
	case d.OKIndex():
		return d.MiddleButtonIndex()
	case d.MiddleButtonIndex():
		return d.CancelIndex()
	default:
		return focus
	}
}

// MoveFocus applies the standard dialog focus-navigation key and reports whether the key was handled.
func (d DialogTrailingButtonsForm) MoveFocus(focus int, key tcell.Key) (int, bool) {
	switch key {
	case tcell.KeyTab:
		return d.Tab(focus), true
	case tcell.KeyBacktab:
		return d.Backtab(focus), true
	case tcell.KeyDown:
		return d.Down(focus), true
	case tcell.KeyUp:
		return d.Up(focus), true
	case tcell.KeyLeft:
		return d.Left(focus), true
	case tcell.KeyRight:
		return d.Right(focus), true
	default:
		return focus, false
	}
}

// DialogLinearForm is the standard OK+Cancel trailing-button layout.
type DialogLinearForm = DialogTrailingButtonsForm

// NewDialogLinearForm returns layout for NumContent fields and two trailing buttons.
func NewDialogLinearForm(numContent int) DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(numContent, 2)
}
