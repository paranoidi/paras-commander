package dialog

import "github.com/paranoidi/paras-commander/internal/theme"

// DialogRenderContext carries shared paint options for dialogs that show paths and/or icons.
type DialogRenderContext struct {
	Styles      theme.Theme
	UserHomeDir string
	ShowIcons   bool
	IconLead    int
}
