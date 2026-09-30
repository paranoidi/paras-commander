package dialog

// DedupReturnDialogState is the modal offered when Find duplicates is run from
// outside the kept results' root: show the kept results or rescan.
type DedupReturnDialogState struct {
	Open        bool
	ButtonFocus int // 0=Show, 1=Rescan, 2=Cancel
}
