package ui

import "github.com/gdamore/tcell/v2"

// DialogTrailingButtonsForm describes focus indices for a dialog with N content rows
// (indices 0..NumContent-1) followed by NumTrailingButtons in order: OK, optional middle, Cancel.
// NumTrailingButtons must be 2 or 3.
type DialogTrailingButtonsForm struct {
	NumContent         int
	NumTrailingButtons int
}

// NewDialogTrailingButtonsForm returns a trailing-button layout. numTrailingButtons must be 2 or 3.
func NewDialogTrailingButtonsForm(numContent, numTrailingButtons int) DialogTrailingButtonsForm {
	if numTrailingButtons != 2 && numTrailingButtons != 3 {
		numTrailingButtons = 2
	}
	return DialogTrailingButtonsForm{NumContent: numContent, NumTrailingButtons: numTrailingButtons}
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
func (d DialogTrailingButtonsForm) Tab(focus int) int {
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + 1) % n
}

// Backtab moves focus backward with wrap.
func (d DialogTrailingButtonsForm) Backtab(focus int) int {
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
	NumContent int
}

// NewDialogLinearForm returns layout for NumContent fields and two trailing buttons.
func NewDialogLinearForm(numContent int) DialogLinearForm {
	return DialogLinearForm{NumContent: numContent}
}

func (d DialogLinearForm) trailing() DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(d.NumContent, 2)
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

// TransferDialogLinearForm is focus navigation for the copy/move destination dialog:
// NumContent fields, then OK, Add paused, Cancel.
type TransferDialogLinearForm struct {
	NumContent int
}

// NewTransferDialogLinearForm returns layout for NumContent fields plus three trailing buttons.
func NewTransferDialogLinearForm(numContent int) TransferDialogLinearForm {
	return TransferDialogLinearForm{NumContent: numContent}
}

func (d TransferDialogLinearForm) trailing() DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(d.NumContent, 3)
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
	return NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(st)).MoveFocus(focus, key)
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
