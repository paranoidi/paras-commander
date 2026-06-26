package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// CompareFilterDialogState is the compare category filter picker modal.
// Focus 0-5 are filter radio rows; 6 = OK; 7 = Cancel.
// OriginalFocus holds the focus index active when the dialog opened, used to
// restore the filter if the user cancels.
type CompareFilterDialogState struct {
	Open          bool
	Focus         int
	OriginalFocus int
}

const compareFilterDialogNumContent = 6

// CompareFilterDialogMoveFocus applies standard dialog navigation.
func CompareFilterDialogMoveFocus(focus int, key tcell.Key) (int, bool) {
	form := NewDialogTrailingButtonsForm(compareFilterDialogNumContent, 2)
	return form.MoveFocus(focus, key)
}

// CompareFilterDialogOKIndex returns the OK button focus index.
func CompareFilterDialogOKIndex() int {
	return compareFilterDialogNumContent
}

// CompareFilterDialogCancelIndex returns the Cancel button focus index.
func CompareFilterDialogCancelIndex() int {
	return compareFilterDialogNumContent + 1
}

// CompareFilterForFocus maps focus index 0-5 to a Filter value.
func CompareFilterForFocus(focus int) (comparepkg.Filter, bool) {
	switch focus {
	case 0:
		return comparepkg.FilterAll, true
	case 1:
		return comparepkg.FilterEqual, true
	case 2:
		return comparepkg.FilterRelocated, true
	case 3:
		return comparepkg.FilterPrimaryOnly, true
	case 4:
		return comparepkg.FilterSecondaryOnly, true
	case 5:
		return comparepkg.FilterContentDiff, true
	default:
		return comparepkg.FilterAll, false
	}
}

// FocusForCompareFilter maps a Filter value to its radio focus index.
func FocusForCompareFilter(f comparepkg.Filter) int {
	switch f {
	case comparepkg.FilterAll:
		return 0
	case comparepkg.FilterEqual:
		return 1
	case comparepkg.FilterRelocated:
		return 2
	case comparepkg.FilterPrimaryOnly:
		return 3
	case comparepkg.FilterSecondaryOnly:
		return 4
	case comparepkg.FilterContentDiff:
		return 5
	default:
		return 0
	}
}
