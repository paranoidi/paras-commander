package dialog

// FlattenDialogState holds the flatten confirmation dialog.
type FlattenDialogState struct {
	Open        bool
	Destination string
	Recursive   bool
	RemoveEmpty bool
	FocusField  int
	DirRoots    []string // pruned directory roots at open (path strings)
}
