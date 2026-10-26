package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Flatten dialog widget IDs. The buttons are dlg.OK and dlg.Cancel.
const (
	FlattenDest        = dlg.ID("dest")
	FlattenRecursive   = dlg.ID("recursive")
	FlattenRemoveEmpty = dlg.ID("remove-empty")
)

// FlattenView is the flatten dialog's widget spec: the destination input, then the Recursive and
// Remove empty directories checkboxes, above OK / Cancel. The destination row's rect (see
// dlg.Placement.Item) anchors the path completion dropdown, which DrawFlattenDialog paints after
// the spec.
func FlattenView(s FlattenDialogState) dlg.Spec {
	return dlg.Spec{
		Title: "Flatten", Width: PreferredFormDialogWidth, Unclamped: true,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{{
				{Kind: dlg.KindLabel, Label: "Destination:"},
				fileFieldInput(FlattenDest, s.Destination, s.DestPathInvalid && !s.DestPathCheckPending),
				{Kind: dlg.KindSeparator},
			}}},
			{Columns: [][]dlg.Item{{
				{ID: FlattenRecursive, Kind: dlg.KindCheckbox, Label: "Recursive flatten", Mnemonic: 'R', On: s.Recursive},
				{ID: FlattenRemoveEmpty, Kind: dlg.KindCheckbox, Label: "Remove empty directories", Mnemonic: 'E', On: s.RemoveEmpty},
			}}},
		},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// Apply folds an activation from dlg.HandleKey into the state: the checkboxes toggle.
// The destination, OK and Cancel are the caller's to handle.
func (s *FlattenDialogState) Apply(ev dlg.Event) {
	switch ev.ID {
	case FlattenRecursive:
		s.Recursive = !s.Recursive
	case FlattenRemoveEmpty:
		s.RemoveEmpty = !s.RemoveEmpty
	}
}
