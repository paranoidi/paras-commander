package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// DrawUserMenuDialog draws the F2 user menu list with OK/Cancel.
func DrawUserMenuDialog(screen tcell.Screen, layout Layout, state UserMenuDialogState, styles theme.Theme) {
	n := len(state.Entries)
	if n == 0 {
		return
	}
	form := NewDialogLinearForm(n)
	visibleRows := UserMenuListViewportRows(layout, n)
	height := visibleRows + 4
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 6 {
		return
	}
	width := 64
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 32 {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	title := state.Title
	if title == "" {
		title = "User menu"
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	leftCol := rect.X + 2
	y := rect.Y + 1

	for row := 0; row < visibleRows; row++ {
		idx := state.ScrollOffset + row
		if idx >= n {
			break
		}
		e := state.Entries[idx]
		label := fmt.Sprintf("%s  %s", e.Key, e.Title)
		var sh rune
		if k := []rune(e.Key); len(k) > 0 {
			sh = k[0]
		}
		draw.DrawDialogRadio(screen, leftCol, y, label, sh, state.Selected == idx, state.Focus == idx, styles)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	okFocused := state.Focus == form.OKIndex()
	cancelFocused := state.Focus == form.CancelIndex()
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}

// UserMenuListViewportRows returns how many entry rows fit in the user menu dialog body.
func UserMenuListViewportRows(layout Layout, entryCount int) int {
	if entryCount <= 0 {
		return 0
	}
	maxBody := layout.Height - 6
	if maxBody < 3 {
		maxBody = 3
	}
	if entryCount < maxBody {
		return entryCount
	}
	return maxBody
}
// UserMenuEnsureScroll keeps ScrollOffset consistent with Selected and viewport.
func UserMenuEnsureScroll(state *UserMenuDialogState, visibleRows int) {
	if visibleRows < 1 {
		return
	}
	n := len(state.Entries)
	if n == 0 {
		state.ScrollOffset = 0
		return
	}
	if state.Selected < 0 {
		state.Selected = 0
	}
	if state.Selected >= n {
		state.Selected = n - 1
	}
	if state.ScrollOffset > state.Selected {
		state.ScrollOffset = state.Selected
	}
	if state.ScrollOffset+visibleRows <= state.Selected {
		state.ScrollOffset = state.Selected - visibleRows + 1
	}
	if state.ScrollOffset < 0 {
		state.ScrollOffset = 0
	}
	maxScroll := n - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if state.ScrollOffset > maxScroll {
		state.ScrollOffset = maxScroll
	}
}
