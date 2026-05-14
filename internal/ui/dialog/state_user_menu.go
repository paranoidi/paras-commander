package dialog

import "github.com/paranoidi/paras-commander/internal/usermenu"

// UserMenuDialogState is the F2 user menu modal.
type UserMenuDialogState struct {
	Open         bool
	Title        string
	Entries      []usermenu.MenuEntry
	Selected     int
	Focus        int // 0..len(Entries)-1 list, OK, Cancel (DialogLinearForm)
	ScrollOffset int
	SourcePath   string
}
