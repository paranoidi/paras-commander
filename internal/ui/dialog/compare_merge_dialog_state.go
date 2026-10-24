package dialog

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// CompareMergeDialogState is the compare merge reconciliation modal.
type CompareMergeDialogState struct {
	Open bool
	// Focus is the focused widget (the CompareMerge* IDs, dlg.OK or dlg.Cancel); empty is the first.
	Focus         dlg.ID
	Direction     comparepkg.MergeDirection
	CopyMissing   bool
	CopyModified  bool
	MoveMode      bool
	PrimaryPath   string
	SecondaryPath string
	PreviewText   string
}
