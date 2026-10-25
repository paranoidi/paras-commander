package dialog

import (
	"time"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// DrawDedupProgressDialog paints the find-duplicates scan progress modal from its widget spec
// (see DedupProgressView).
func DrawDedupProgressDialog(
	screen tcell.Screen,
	layout Layout,
	state DedupProgressDialogState,
	snap comparepkg.DedupSnapshot,
	styles theme.Theme,
	userHomeDir string,
) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, DedupProgressView(snap, userHomeDir, time.Now()), state.Focus, styles)
}
