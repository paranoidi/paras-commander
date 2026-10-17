package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// CompareFilterDialogMode picks what OK does with the chosen kind.
type CompareFilterDialogMode int

const (
	CompareFilterModeCategory CompareFilterDialogMode = iota // set the view filter
	CompareFilterModeSelect                                  // select matching rows
	CompareFilterModeUnselect                                // unselect matching rows
)

// CompareFilterDialogState is the compare category picker modal. Mode switches it
// from the view filter to select/unselect by kind (OK (un)selects matching rows instead).
// Focus indexes the radio rows, then OK, then Cancel.
// Filter is the pending radio selection (updated by Space/Enter/Alt mnemonic;
// applied only on OK).
type CompareFilterDialogState struct {
	Open   bool
	Mode   CompareFilterDialogMode
	Focus  int
	Filter comparepkg.Filter
}

// Radios returns the radio list for the dialog's mode.
func (s CompareFilterDialogState) Radios() []comparepkg.FilterDialogRadio {
	if s.Mode != CompareFilterModeCategory {
		return comparepkg.SelectDialogRadios()
	}
	return comparepkg.FilterDialogRadios()
}

// Title returns the dialog frame title.
func (s CompareFilterDialogState) Title() string {
	switch s.Mode {
	case CompareFilterModeSelect:
		return "Select"
	case CompareFilterModeUnselect:
		return "Unselect"
	}
	return "Category"
}

// MoveFocus applies standard dialog navigation.
// Segments: all radios | buttons.
func (s CompareFilterDialogState) MoveFocus(key tcell.Key) (int, bool) {
	n := len(s.Radios())
	form := NewDialogTrailingButtonsForm(n, 2).WithSegments(0, n)
	return form.MoveFocus(s.Focus, key)
}

// OKIndex returns the OK button focus index.
func (s CompareFilterDialogState) OKIndex() int {
	return len(s.Radios())
}

// CancelIndex returns the Cancel button focus index.
func (s CompareFilterDialogState) CancelIndex() int {
	return len(s.Radios()) + 1
}

// FilterForFocus maps radio focus index to a Filter value.
func (s CompareFilterDialogState) FilterForFocus(focus int) (comparepkg.Filter, bool) {
	return comparepkg.FilterForFocus(s.Radios(), focus)
}

// FocusForFilter maps a Filter value to its radio focus index.
func (s CompareFilterDialogState) FocusForFilter(f comparepkg.Filter) int {
	return comparepkg.FocusForFilter(s.Radios(), f)
}
