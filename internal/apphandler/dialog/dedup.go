package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog"

// openDedupEmptyDirsConfirm opens the "remove directories left empty by this
// delete?" confirmation, shown after the delete-marked-files dialog is
// confirmed, but only when the delete would actually leave directories
// dangling. Defaults to Yes (index 0) — removing already-empty directories
// is low-risk, unlike the file deletion itself.
func (h *Handler) openDedupEmptyDirsConfirm() {
	dirs := h.dedup.EmptyDirsAfterDelete()
	if len(dirs) == 0 {
		h.dedup.DeleteMarked(false)
		return
	}
	h.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{Open: true, Focus: 0, Dirs: dirs}
}
