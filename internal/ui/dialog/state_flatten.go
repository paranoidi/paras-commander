package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// FlattenDialogState holds the flatten confirmation dialog.
type FlattenDialogState struct {
	Open        bool
	Destination FileDialogField
	Recursive   bool
	RemoveEmpty bool
	// Focus is the focused widget (FlattenDest, FlattenRecursive, FlattenRemoveEmpty, dlg.OK or
	// dlg.Cancel); empty is the destination field.
	Focus    dlg.ID
	DirRoots []string // pruned directory roots at open (path strings)
	// DestPathInvalid is true after a debounced check when the destination looks like a path and os.Lstat fails.
	DestPathInvalid bool
	// DestPathCheckPending is true until debounced validation runs after Destination.Value changed.
	DestPathCheckPending bool
}

// FocusOnDest reports whether the destination field has focus.
func (s FlattenDialogState) FocusOnDest() bool { return s.Focus == FlattenDest || s.Focus == "" }
