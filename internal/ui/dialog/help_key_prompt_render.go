package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// helpKeyPromptWidth fits the title and the message.
const helpKeyPromptWidth = 44

// HelpKeyPromptView is the find-by-keypress prompt drawn over the help dialog. It has no buttons:
// every key is the one being looked up.
func HelpKeyPromptView() dlg.Spec {
	return dlg.Spec{
		Title: "Find action by keypress", Width: helpKeyPromptWidth, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: "Press any key to see what it does."},
		}}}},
	}
}

// DrawHelpKeyPrompt renders the prompt from its widget spec.
func DrawHelpKeyPrompt(screen tcell.Screen, layout Layout, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, HelpKeyPromptView(), "", styles)
}
