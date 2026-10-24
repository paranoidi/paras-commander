package dialog

import (
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// SFTPConnectDialogState is the SSH/SFTP connect dialog (known hosts + manual URI).
type SFTPConnectDialogState struct {
	Open         bool
	PanelID      int
	DisplayLines []string
	Query        string
	QueryCursor  int // rune offset of caret within Query (0..len(runes))
	QueryScroll  int // first visible rune offset for horizontal scrolling
	Ranked       []int
	MatchRanges  [][]search.Range // len == len(DisplayLines); highlights on DisplayLines
	Selected     int              // host list cursor row (index into Ranked)
	ListScroll   int              // first visible host row
	Location     FileDialogField
	// Focus is SFTPFilter ("" = the filter, which also steers the host list), SFTPLocation,
	// dlg.OK or dlg.Cancel.
	Focus dlg.ID
}
