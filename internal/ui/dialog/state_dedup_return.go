package dialog

// DedupReturnDialogState is the modal offered when Find duplicates is run from
// the kept results' root: show the kept results or rescan. With ScopeDirs set it
// instead asks, when directories are selected, whether to scan Dir or only them.
type DedupReturnDialogState struct {
	Open        bool
	ButtonFocus int // 0=Show, 1=Rescan, 2=Cancel; scope mode 0=Selected, 1=Directory, 2=Cancel
	Dir         string
	ScopeDirs   []string // selected directories (absolute); non-empty = scope mode
}
