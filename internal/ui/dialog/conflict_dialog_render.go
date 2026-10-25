package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawConflictDialog paints the quick job-blocker modal from its widget spec (see
// ConflictDialogView).
func DrawConflictDialog(screen tcell.Screen, layout Layout, state ConflictDialogState, styles theme.Theme, userHomeDir string) {
	dlg.Draw(screen, layout.Width, layout.Height, ConflictDialogView(state, userHomeDir), *state.ConflictDialogFocus(), styles)
}

// ConflictPromptText is the question under the file summaries; it carries the Reprompt note.
func ConflictPromptText(c *jobs.ConflictEvent) string {
	if c != nil && c.Mismatch {
		if c.DestIsDir {
			return "Destination is a directory"
		}
		return "Destination is a file"
	}
	if c != nil && c.Reprompt != "" {
		return c.Reprompt + " Overwrite this file?"
	}
	return "Overwrite this file?"
}

func formatConflictDialogBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fM", float64(n)/(1024*1024))
}
