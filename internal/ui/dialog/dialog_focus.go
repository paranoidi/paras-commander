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

// Down moves focus down per AGENTS.md: within content only until last content, then to OK;
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

// Up moves focus up per AGENTS.md: from any trailing button to last content; within content without wrap from 0.
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

// DialogLinearForm describes focus indices for a dialog with N content rows
// (indices 0..numContent-1) followed by OK and Cancel.
type DialogLinearForm struct {
	NumContent    int
	SegmentStarts []int
}

// NewDialogLinearForm returns layout for NumContent fields and two trailing buttons.
func NewDialogLinearForm(numContent int) DialogLinearForm {
	return DialogLinearForm{NumContent: numContent}
}

// WithSegments returns a copy with segment-jump Tab/Backtab enabled (see DialogTrailingButtonsForm).
func (d DialogLinearForm) WithSegments(starts ...int) DialogLinearForm {
	d.SegmentStarts = starts
	return d
}

func (d DialogLinearForm) trailing() DialogTrailingButtonsForm {
	t := NewDialogTrailingButtonsForm(d.NumContent, 2)
	if len(d.SegmentStarts) >= 2 {
		t.SegmentStarts = d.SegmentStarts
	}
	return t
}

// TotalFocus is the number of focus positions (content + OK + Cancel).
func (d DialogLinearForm) TotalFocus() int {
	return d.trailing().TotalFocus()
}

// OKIndex is the focus index of the OK button.
func (d DialogLinearForm) OKIndex() int {
	return d.trailing().OKIndex()
}

// CancelIndex is the focus index of the Cancel button.
func (d DialogLinearForm) CancelIndex() int {
	return d.trailing().CancelIndex()
}

// Tab advances focus with wrap (Tab / Shift+Tab convenience), 0..TotalFocus-1.
func (d DialogLinearForm) Tab(focus int) int {
	return d.trailing().Tab(focus)
}

// Backtab moves focus backward with wrap.
func (d DialogLinearForm) Backtab(focus int) int {
	return d.trailing().Backtab(focus)
}

// Down moves focus down per AGENTS.md: within content only until last content, then to OK;
// on OK/Cancel, Down does nothing.
func (d DialogLinearForm) Down(focus int) int {
	return d.trailing().Down(focus)
}

// Up moves focus up per AGENTS.md: from OK/Cancel to last content; within content without wrap from 0.
func (d DialogLinearForm) Up(focus int) int {
	return d.trailing().Up(focus)
}

// Left moves focus between OK and Cancel only.
func (d DialogLinearForm) Left(focus int) int {
	return d.trailing().Left(focus)
}

// Right moves focus from OK to Cancel only.
func (d DialogLinearForm) Right(focus int) int {
	return d.trailing().Right(focus)
}

// MoveFocus applies the standard dialog focus-navigation key and reports
// whether the key was handled.
func (d DialogLinearForm) MoveFocus(focus int, key tcell.Key) (int, bool) {
	return d.trailing().MoveFocus(focus, key)
}

// UserMenuDialogForm is focus for N menu rows plus a single Cancel button (no OK).
type UserMenuDialogForm struct {
	NumEntries int
}

// CancelIndex is the focus index of the Cancel button.
func (d UserMenuDialogForm) CancelIndex() int {
	return d.NumEntries
}

// TotalFocus is entries plus Cancel.
func (d UserMenuDialogForm) TotalFocus() int {
	if d.NumEntries < 0 {
		return 1
	}
	return d.NumEntries + 1
}

func (d UserMenuDialogForm) Tab(focus int) int {
	total := d.TotalFocus()
	if total <= 0 {
		return 0
	}
	return (focus + 1) % total
}

func (d UserMenuDialogForm) Backtab(focus int) int {
	total := d.TotalFocus()
	if total <= 0 {
		return 0
	}
	if focus <= 0 {
		return total - 1
	}
	return focus - 1
}

func (d UserMenuDialogForm) Down(focus int) int {
	if d.NumEntries == 0 {
		return d.CancelIndex()
	}
	if focus < d.NumEntries-1 {
		return focus + 1
	}
	if focus == d.NumEntries-1 {
		return d.CancelIndex()
	}
	return focus
}

func (d UserMenuDialogForm) Up(focus int) int {
	if focus == d.CancelIndex() {
		if d.NumEntries > 0 {
			return d.NumEntries - 1
		}
		return focus
	}
	if focus > 0 {
		return focus - 1
	}
	return focus
}

// MoveFocus applies user-menu dialog navigation (entries + Cancel only).
func (d UserMenuDialogForm) MoveFocus(focus int, key tcell.Key) (int, bool) {
	switch key {
	case tcell.KeyTab:
		return d.Tab(focus), true
	case tcell.KeyBacktab:
		return d.Backtab(focus), true
	case tcell.KeyDown:
		return d.Down(focus), true
	case tcell.KeyUp:
		return d.Up(focus), true
	case tcell.KeyLeft, tcell.KeyRight:
		return focus, true
	default:
		return focus, false
	}
}

// TransferDialogLinearForm is focus navigation for the copy/move destination dialog:
// NumContent fields, then OK, Add paused, Cancel.
type TransferDialogLinearForm struct {
	NumContent    int
	SegmentStarts []int
}

// NewTransferDialogLinearForm returns layout for NumContent fields plus three trailing buttons.
func NewTransferDialogLinearForm(numContent int) TransferDialogLinearForm {
	return TransferDialogLinearForm{NumContent: numContent}
}

// WithSegments returns a copy with segment-jump Tab/Backtab enabled (see DialogTrailingButtonsForm).
func (d TransferDialogLinearForm) WithSegments(starts ...int) TransferDialogLinearForm {
	d.SegmentStarts = starts
	return d
}

func (d TransferDialogLinearForm) trailing() DialogTrailingButtonsForm {
	t := NewDialogTrailingButtonsForm(d.NumContent, 3)
	if len(d.SegmentStarts) >= 2 {
		t.SegmentStarts = d.SegmentStarts
	}
	return t
}

func (d TransferDialogLinearForm) TotalFocus() int {
	return d.trailing().TotalFocus()
}

func (d TransferDialogLinearForm) OKIndex() int {
	return d.trailing().OKIndex()
}

func (d TransferDialogLinearForm) AddPausedIndex() int {
	return d.trailing().MiddleButtonIndex()
}

func (d TransferDialogLinearForm) CancelIndex() int {
	return d.trailing().CancelIndex()
}

func (d TransferDialogLinearForm) Tab(focus int) int {
	return d.trailing().Tab(focus)
}

func (d TransferDialogLinearForm) Backtab(focus int) int {
	return d.trailing().Backtab(focus)
}

func (d TransferDialogLinearForm) Down(focus int) int {
	return d.trailing().Down(focus)
}

func (d TransferDialogLinearForm) Up(focus int) int {
	return d.trailing().Up(focus)
}

func (d TransferDialogLinearForm) Left(focus int) int {
	return d.trailing().Left(focus)
}

func (d TransferDialogLinearForm) Right(focus int) int {
	return d.trailing().Right(focus)
}

// MoveFocus applies the standard dialog focus-navigation key and reports
// whether the key was handled.
func (d TransferDialogLinearForm) MoveFocus(focus int, key tcell.Key) (int, bool) {
	return d.trailing().MoveFocus(focus, key)
}

// TransferDialogMoveFocus applies Tab/arrow navigation for the copy/move dialog (destination or self-copy phase).
func TransferDialogMoveFocus(st TransferDialogState, focus int, key tcell.Key) (int, bool) {
	n := TransferDialogEffectiveNumContent(st)
	okIdx := n // buttons start at numContent (3-button form, so OKIndex == NumContent)
	var segs []int
	if n == 3 {
		// Copy: destination(0) | preserve checkboxes(1,2) | buttons(okIdx)
		segs = []int{0, 1, okIdx}
	} else {
		// Move or self-copy: single content field | buttons
		segs = []int{0, okIdx}
	}
	return NewTransferDialogLinearForm(n).WithSegments(segs...).MoveFocus(focus, key)
}

// DialogPairLeftRight toggles between 0 and 1 (e.g. quit confirm) with Left/Right.
func DialogPairLeftRight(focus int, goRight bool) int {
	if goRight {
		if focus < 1 {
			return 1
		}
		return focus
	}
	if focus > 0 {
		return 0
	}
	return focus
}
