package dialog

import "github.com/paranoidi/paras-commander/internal/search"

// SFTPConnectDialogState is the SSH/SFTP link dialog (known hosts + manual URI).
type SFTPConnectDialogState struct {
	Open         bool
	PanelID      int
	DisplayLines []string
	Query        string
	QueryCursor  int // rune offset of caret within Query (0..len(runes))
	QueryScroll  int // first visible rune offset for horizontal scrolling
	Ranked       []int
	MatchRanges  [][]search.Range // len == len(DisplayLines); highlights on DisplayLines
	Selected     int
	ListScroll   int
	Location     FileDialogField
	// Focus: 0=list+query, 1=location input, 2=OK, 3=Cancel
	Focus int
}
