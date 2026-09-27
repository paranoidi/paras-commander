package dialog

// FlattenDialogState holds the flatten confirmation dialog.
type FlattenDialogState struct {
	Open        bool
	Destination FileDialogField
	Recursive   bool
	RemoveEmpty bool
	FocusField  int
	DirRoots    []string // pruned directory roots at open (path strings)
	// DestPathInvalid is true after a debounced check when the destination looks like a path and os.Lstat fails.
	DestPathInvalid bool
	// DestPathCheckPending is true until debounced validation runs after Destination.Value changed.
	DestPathCheckPending bool
}
