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

// CompareMergeDialogLinearForm is focus navigation for the merge dialog.
type CompareMergeDialogLinearForm struct {
	form DialogTrailingButtonsForm
}

// NewCompareMergeDialogLinearForm returns merge dialog focus layout.
func NewCompareMergeDialogLinearForm() CompareMergeDialogLinearForm {
	return CompareMergeDialogLinearForm{
		form: NewDialogTrailingButtonsForm(compareMergeDialogNumContent, 2),
	}
}

// CompareMergeDialogMoveFocus applies standard dialog navigation.
func CompareMergeDialogMoveFocus(focus int, key tcell.Key) (int, bool) {
	return NewCompareMergeDialogLinearForm().form.MoveFocus(focus, key)
}

// OKIndex returns the OK button focus index.
func (d CompareMergeDialogLinearForm) OKIndex() int { return d.form.OKIndex() }

// CancelIndex returns the Cancel button focus index.
func (d CompareMergeDialogLinearForm) CancelIndex() int { return d.form.CancelIndex() }
