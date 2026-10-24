package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Button IDs of the dedup "remove empty directories" confirmation. The third button is dlg.Cancel.
const (
	DedupEmptyDirsYes = dlg.ID("dedup-empty-yes")
	DedupEmptyDirsNo  = dlg.ID("dedup-empty-no")
)

// DedupEmptyDirsConfirmState holds the "remove directories left empty by this
// delete?" confirmation shown after confirming a dedup delete.
type DedupEmptyDirsConfirmState struct {
	Open  bool
	Focus dlg.ID   // DedupEmptyDirsYes (default, also ""), DedupEmptyDirsNo or dlg.Cancel
	Dirs  []string // directories that would be removed, relative to the dedup root
}
