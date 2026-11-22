package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Button IDs of the dedup "Keep Directory" confirmation. The third button is dlg.Cancel.
const (
	DedupKeepThis     = dlg.ID("dedup-keep-this")
	DedupKeepPrevious = dlg.ID("dedup-keep-previous")
)

// DedupKeepConfirmState holds the confirmation shown when keeping a directory would drop keeps
// the same duplicate groups already have elsewhere.
type DedupKeepConfirmState struct {
	Open   bool
	Focus  dlg.ID   // DedupKeepPrevious (default), DedupKeepThis or dlg.Cancel
	Dir    string   // the directory being kept, relative to the dedup root ("" = the root)
	Groups int      // duplicate groups that already keep a copy elsewhere
	Kept   []string // those kept copies, relative to the dedup root
}
