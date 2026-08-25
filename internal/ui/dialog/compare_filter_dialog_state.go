package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// CompareFilterDialogState is the compare category filter picker modal.
// Focus 0-5 are filter radio rows; 6 = OK; 7 = Cancel.
// Filter is the pending radio selection (updated by Space/Enter/Alt mnemonic;
// applied to the view only on OK).
type CompareFilterDialogState struct {
	Open   bool
	Focus  int
	Filter comparepkg.Filter
}

func compareFilterDialogNumContent() int {
	return len(comparepkg.FilterDialogRadios())
}

// CompareFilterDialogMoveFocus applies standard dialog navigation.
// Segments: all filter radios | buttons.
func CompareFilterDialogMoveFocus(focus int, key tcell.Key) (int, bool) {
	n := compareFilterDialogNumContent()
	form := NewDialogTrailingButtonsForm(n, 2).WithSegments(0, n)
	return form.MoveFocus(focus, key)
}

// CompareFilterDialogOKIndex returns the OK button focus index.
func CompareFilterDialogOKIndex() int {
	return compareFilterDialogNumContent()
}

// CompareFilterDialogCancelIndex returns the Cancel button focus index.
func CompareFilterDialogCancelIndex() int {
	return compareFilterDialogNumContent() + 1
}

// CompareFilterForFocus maps radio focus index to a Filter value.
func CompareFilterForFocus(focus int) (comparepkg.Filter, bool) {
	return comparepkg.FilterForFocus(focus)
}

// FocusForCompareFilter maps a Filter value to its radio focus index.
func FocusForCompareFilter(f comparepkg.Filter) int {
	return comparepkg.FocusForFilter(f)
}
