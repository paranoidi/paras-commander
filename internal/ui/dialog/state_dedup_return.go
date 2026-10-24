package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// DedupReturnDialogState is the modal offered when Find duplicates is run from
// the kept results' root: show the kept results or rescan. With ScopeDirs set it
// instead asks, when directories are selected, whether to scan Dir or only them.
type DedupReturnDialogState struct {
	Open      bool
	Focus     dlg.ID // DedupReturnFirst (Show, or Selected in scope mode), DedupReturnSecond or dlg.Cancel
	Dir       string
	ScopeDirs []string // selected directories (absolute); non-empty = scope mode
}
