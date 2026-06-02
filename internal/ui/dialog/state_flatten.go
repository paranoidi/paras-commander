package dialog

// FlattenDialogState holds the flatten confirmation dialog.
type FlattenDialogState struct {
	Open         bool
	Destination  FileDialogField
	DestSubFocus int // FlattenDestSubFocus* when FocusField==0
	Recursive    bool
	RemoveEmpty  bool
	FocusField   int
	DirRoots     []string // pruned directory roots at open (path strings)
	// DestPathInvalid is true after a debounced check when the destination looks like a path and os.Lstat fails.
	DestPathInvalid bool
	// DestPathCheckPending is true until debounced validation runs after Destination.Value changed.
	DestPathCheckPending bool
}

// Destination sub-focus for path input row (text vs trailing path-picker glyph).
const (
	FlattenDestSubFocusText = iota
	FlattenDestSubFocusPicker
)
