package dialog

import (
	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
)

// CompareMergeDialogState is the compare merge reconciliation modal.
type CompareMergeDialogState struct {
	Open bool
	// Focus indexes: 0-1 direction radios, 2-3 transfer checkboxes, 4-5 operation radios, 6 OK, 7 Cancel.
	Focus         int
	Direction     comparepkg.MergeDirection
	CopyMissing   bool
	CopyModified  bool
	MoveMode      bool
	PrimaryPath   string
	SecondaryPath string
	PreviewText   string
}

const compareMergeDialogNumContent = 6

// CompareMergeDialogLinearForm is the merge dialog focus layout.
type CompareMergeDialogLinearForm = DialogTrailingButtonsForm

// NewCompareMergeDialogLinearForm returns merge dialog focus layout.
// Segments: direction radios(0,1) | transfer checkboxes(2,3) | operation radios(4,5) | buttons(6).
func NewCompareMergeDialogLinearForm() DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(compareMergeDialogNumContent, 2).WithSegments(0, 2, 4, compareMergeDialogNumContent)
}

// CompareMergeDialogMoveFocus applies standard dialog navigation.
func CompareMergeDialogMoveFocus(focus int, key tcell.Key) (int, bool) {
	return NewCompareMergeDialogLinearForm().MoveFocus(focus, key)
}
