package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// HelpKeyPromptCancel is the prompt's Cancel button. It has no mnemonic: every key is the one
// being looked up, so only the mouse reaches it.
const HelpKeyPromptCancel = dlg.ID("cancel")

// helpKeyPromptWidth fits the title and the message.
const helpKeyPromptWidth = 44

// HelpKeyPromptView is the find-by-keypress prompt drawn over the help dialog.
func HelpKeyPromptView() dlg.Spec {
	return dlg.Spec{
		Title: "Find action by keypress", Width: helpKeyPromptWidth, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: "Press any key to see what it does."},
		}}}},
		Buttons: []dlg.Button{{ID: HelpKeyPromptCancel, Label: "Cancel"}},
	}
}

// DrawHelpKeyPrompt renders the prompt from its widget spec.
func DrawHelpKeyPrompt(screen tcell.Screen, layout Layout, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, HelpKeyPromptView(), HelpKeyPromptCancel, styles)
}
