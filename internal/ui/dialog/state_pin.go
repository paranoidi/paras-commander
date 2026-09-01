package dialog

import "github.com/paranoidi/paras-commander/internal/search"

// PinDialogState is the Pin dialog's own UI state (fuzzy filter + cursor/scroll). The
// pinned items themselves live in Model.PinnedItems (internal/ui), not here, so pins
// survive the dialog closing — PinDialogItem below is a per-render projection of that
// list, not a duplicate copy. There is no Focus field (unlike History's list/OK/Cancel):
// Pin has no button row, so the query+list are always the sole focus target.
type PinDialogState struct {
	Open        bool
	Query       string
	QueryCursor int              // rune offset of caret within Query (0..len(runes))
	QueryScroll int              // first visible rune offset for horizontal scrolling
	Ranked      []int            // indices into Model.PinnedItems (rank order)
	MatchRanges [][]search.Range // len == len(Model.PinnedItems); highlights on Path
	Selected    int              // index into Ranked
	ListScroll  int
}

// PinDialogItem is one display-ready pin-list row, converted from ui.PinnedItem by the
// caller in package ui (this package cannot import ui — ui already imports dialog).
type PinDialogItem struct {
	Path        string
	IsDir       bool
	PathMissing bool
}
