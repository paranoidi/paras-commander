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
// In scope mode (state.ScopeDirs) it asks to search state.Dir or only the selection.
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

	lead, tail := "Results for ", " are kept."
	line2 := fmt.Sprintf("%d groups, %s wasted", groups, formatDedupByteSize(wasted))
	first, second := "Show", "Rescan"
	if n := len(state.ScopeDirs); n > 0 {
		root, lead, tail = state.Dir, "Search ", ""
		line2 = fmt.Sprintf("or only the %d selected directories?", n)
		if n == 1 {
			line2 = "or only the selected directory?"
		}
		first, second = "Selected", "Directory"
	}
	path := primitive.FitPathForWidth(primitive.PathWithHomeTilde(root, userHomeDir), max(textW-len(lead)-len(tail), 1))
	primitive.Text(screen, textX, rect.Y+1, textW, lead+path+tail, textStyle)
	primitive.Text(screen, textX, rect.Y+2, textW, line2, textStyle)

	draw.DrawDialogButtonRowCentered(screen, rect, rect.Y+rect.Height-2, []draw.DialogButtonSpec{
		{Label: first, Shortcut: rune(first[0]), Focused: state.ButtonFocus == 0},
		{Label: second, Shortcut: rune(second[0]), Focused: state.ButtonFocus == 1},
		{Label: "Cancel", Shortcut: 'C', Focused: state.ButtonFocus == 2},
	}, styles)
}
