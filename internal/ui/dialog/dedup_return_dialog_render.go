package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DrawDedupReturnDialog paints the "kept results" modal; groups and wasted are
// the kept results' summary (already filtered by the view's ignore-empty toggle).
func DrawDedupReturnDialog(
	screen tcell.Screen,
	layout Layout,
	state DedupReturnDialogState,
	root string,
	groups int,
	wasted int64,
	styles theme.Theme,
	userHomeDir string,
) {
	if !state.Open {
		return
	}
	// message + stats + blank + buttons + borders
	rect := draw.CenteredDialogRect(layout, max(PreferredFormDialogWidth, 52), 6)
	draw.DrawDialogFrame(screen, rect, "Find Duplicates", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	textX := draw.DialogTextX(rect)
	textW := draw.DialogContentWidth(rect)

	const lead, tail = "Results for ", " are kept."
	path := primitive.FitPathForWidth(primitive.PathWithHomeTilde(root, userHomeDir), max(textW-len(lead)-len(tail), 1))
	primitive.Text(screen, textX, rect.Y+1, textW, lead+path+tail, textStyle)
	primitive.Text(screen, textX, rect.Y+2, textW, fmt.Sprintf("%d groups, %s wasted", groups, formatDedupByteSize(wasted)), textStyle)

	draw.DrawDialogButtonRowCentered(screen, rect, rect.Y+rect.Height-2, []draw.DialogButtonSpec{
		{Label: "Show", Shortcut: 'S', Focused: state.ButtonFocus == 0},
		{Label: "Rescan", Shortcut: 'R', Focused: state.ButtonFocus == 1},
		{Label: "Cancel", Shortcut: 'C', Focused: state.ButtonFocus == 2},
	}, styles)
}
