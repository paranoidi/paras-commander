package ui

import "github.com/gdamore/tcell/v2"

// DialogLinearForm describes focus indices for a dialog with N content rows
// (indices 0..numContent-1) followed by OK at firstButton and Cancel at firstButton+1.
type DialogLinearForm struct {
	NumContent  int
	FirstButton int // index of OK; must equal NumContent
}

// NewDialogLinearForm returns layout for NumContent fields and two trailing buttons.
func NewDialogLinearForm(numContent int) DialogLinearForm {
	return DialogLinearForm{NumContent: numContent, FirstButton: numContent}
}

// TotalFocus is the number of focus positions (content + OK + Cancel).
func (d DialogLinearForm) TotalFocus() int {
	return d.NumContent + 2
}

// OKIndex is the focus index of the OK button.
func (d DialogLinearForm) OKIndex() int {
	return d.FirstButton
}

// CancelIndex is the focus index of the Cancel button.
func (d DialogLinearForm) CancelIndex() int {
	return d.FirstButton + 1
}

// Tab advances focus with wrap (Tab / Shift+Tab convenience), 0..TotalFocus-1.
func (d DialogLinearForm) Tab(focus int) int {
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + 1) % n
}

// Backtab moves focus backward with wrap.
func (d DialogLinearForm) Backtab(focus int) int {
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + n - 1) % n
}

// Down moves focus down per AGENTS.md: within content only until last content, then to OK;
// on OK/Cancel, Down does nothing.
func (d DialogLinearForm) Down(focus int) int {
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

// Up moves focus up per AGENTS.md: from OK/Cancel to last content; within content without wrap from 0.
func (d DialogLinearForm) Up(focus int) int {
	if focus >= d.FirstButton {
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

// Left moves focus between OK and Cancel only.
func (d DialogLinearForm) Left(focus int) int {
	if focus == d.CancelIndex() {
		return d.OKIndex()
	}
	return focus
}

// Right moves focus from OK to Cancel only.
func (d DialogLinearForm) Right(focus int) int {
	if focus == d.OKIndex() {
		return d.CancelIndex()
	}
	return focus
}

// MoveFocus applies the standard dialog focus-navigation key and reports
// whether the key was handled.
func (d DialogLinearForm) MoveFocus(focus int, key tcell.Key) (int, bool) {
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

// TransferDialogLinearForm is focus navigation for the copy/move destination dialog:
// NumContent fields, then OK, Add paused, Cancel.
type TransferDialogLinearForm struct {
	NumContent int
}

// NewTransferDialogLinearForm returns layout for NumContent fields plus three trailing buttons.
func NewTransferDialogLinearForm(numContent int) TransferDialogLinearForm {
	return TransferDialogLinearForm{NumContent: numContent}
}

func (d TransferDialogLinearForm) TotalFocus() int {
	return d.NumContent + 3
}

func (d TransferDialogLinearForm) OKIndex() int {
	return d.NumContent
}

func (d TransferDialogLinearForm) AddPausedIndex() int {
	return d.NumContent + 1
}

func (d TransferDialogLinearForm) CancelIndex() int {
	return d.NumContent + 2
}

func (d TransferDialogLinearForm) Tab(focus int) int {
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + 1) % n
}

func (d TransferDialogLinearForm) Backtab(focus int) int {
	n := d.TotalFocus()
	if n <= 0 {
		return 0
	}
	return (focus + n - 1) % n
}

func (d TransferDialogLinearForm) Down(focus int) int {
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

func (d TransferDialogLinearForm) Up(focus int) int {
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

func (d TransferDialogLinearForm) Left(focus int) int {
	switch focus {
	case d.AddPausedIndex():
		return d.OKIndex()
	case d.CancelIndex():
		return d.AddPausedIndex()
	default:
		return focus
	}
}

func (d TransferDialogLinearForm) Right(focus int) int {
	switch focus {
	case d.OKIndex():
		return d.AddPausedIndex()
	case d.AddPausedIndex():
		return d.CancelIndex()
	default:
		return focus
	}
}

// MoveFocus applies the standard dialog focus-navigation key and reports
// whether the key was handled.
func (d TransferDialogLinearForm) MoveFocus(focus int, key tcell.Key) (int, bool) {
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
