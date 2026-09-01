package ui

import "github.com/paranoidi/paras-commander/internal/ui/dialog"

// rowMarksResolver closes over pin/job state so the Find, History, Bookmarks/path-picker,
// Dedup, and Compare row-mark call sites share one resolution path. dialog.RowMarksFunc stays
// plain-data-in/plain-data-out so internal/ui/dialog never needs to import internal/ui.
func rowMarksResolver(pinnedItems []PinnedItem, jobMarks []JobPathMark) dialog.RowMarksFunc {
	pinned := PinnedPathSet(pinnedItems)
	return func(absPath string) dialog.RowMarks {
		if absPath == "" {
			return dialog.RowMarks{}
		}
		_, isPinned := pinned[absPath]
		hasJob, status, write := EntryPathJobMarkStatus(absPath, jobMarks)
		return dialog.RowMarks{Pinned: isPinned, HasJob: hasJob, JobStatus: status, JobWrite: write}
	}
}
