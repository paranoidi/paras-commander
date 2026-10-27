package dialog

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Compare selection button IDs. The Cancel button is dlg.Cancel.
const (
	CompareSelectionDrop  = dlg.ID("drop")
	CompareSelectionStash = dlg.ID("stash")
)

// CompareSelectionDialogState asks what to do with the panels' live selection before Compare
// opens: Compare's selection, delete and merge act on the panel selections, so paths selected
// before Compare would ride along unseen.
type CompareSelectionDialogState struct {
	Open     bool
	Focus    dlg.ID // a CompareSelection* button ID or dlg.Cancel; opened on CompareSelectionStash
	Selected int    // live selected paths across both main panels
}

// CompareSelectionView is the compare selection dialog's widget spec: three message lines above
// Drop / Stash / Cancel.
func CompareSelectionView(s CompareSelectionDialogState) dlg.Spec {
	noun, them := "paths are", "them"
	if s.Selected == 1 {
		noun, them = "path is", "it"
	}
	return dlg.Spec{
		Title: "Compare", Width: 48, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: fmt.Sprintf("%d %s selected in the panels.", s.Selected, noun)},
			{Kind: dlg.KindLabel, Label: "Compare would act on " + them + " too."},
			{Kind: dlg.KindLabel, Label: "What to do with selection?"},
		}}}},
		Buttons: []dlg.Button{
			{ID: CompareSelectionDrop, Label: "Drop", Mnemonic: 'D'},
			{ID: CompareSelectionStash, Label: "Stash", Mnemonic: 'S'},
			dlg.CancelButton,
		},
	}
}
