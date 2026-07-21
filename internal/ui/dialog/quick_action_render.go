package dialog

import (
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// QuickActionShortcuts returns the activation letter for each item (pinned Key when
// set and free, otherwise the same dynamic picks as other dialog mnemonics). No
// letters are reserved — the widget has no OK/Cancel buttons.
func QuickActionShortcuts(items []QuickActionItem) []rune {
	labels := make([]string, len(items))
	configured := make([]rune, len(items))
	for i, it := range items {
		labels[i] = it.Label
		configured[i] = it.Key
	}
	return assignDialogMnemonics(labels, configured, false)
}

// QuickActionIndexForKey returns the item index whose shortcut matches r (case-insensitive).
func QuickActionIndexForKey(items []QuickActionItem, r rune) (int, bool) {
	if r == 0 || !unicode.IsLetter(r) {
		return 0, false
	}
	shortcuts := QuickActionShortcuts(items)
	lr := unicode.ToLower(r)
	for i, sh := range shortcuts {
		if sh != 0 && sh == lr {
			return i, true
		}
	}
	return 0, false
}

// QuickActionViewportRows returns how many item rows fit in the quick-action dialog body.
func QuickActionViewportRows(layout Layout, itemCount int) int {
	if itemCount <= 0 {
		return 0
	}
	maxBody := layout.Height - 4
	if maxBody < 3 {
		maxBody = 3
	}
	if itemCount < maxBody {
		return itemCount
	}
	return maxBody
}

// QuickActionEnsureScroll keeps ScrollOffset consistent with Selected and viewport.
func QuickActionEnsureScroll(state *QuickActionState, visibleRows int) {
	if visibleRows < 1 {
		return
	}
	n := len(state.Items)
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

// DrawQuickActionDialog draws a buttonless, letter-activated list modal: full-row
// selection bar, letter column, no separators/buttons/help footer.
func DrawQuickActionDialog(screen tcell.Screen, layout Layout, state QuickActionState, styles theme.Theme) {
	n := len(state.Items)
	if n == 0 {
		return
	}
	visibleRows := QuickActionViewportRows(layout, n)
	if visibleRows < 1 {
		return
	}
	height := visibleRows + 2
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 3 {
		return
	}

	maxLabel := 0
	for _, it := range state.Items {
		if w := utf8.RuneCountInString(it.Label); w > maxLabel {
			maxLabel = w
		}
	}
	width := maxLabel + 7
	if tw := utf8.RuneCountInString(state.Title) + 6; tw > width {
		width = tw
	}
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 8 {
		width = 8
	}

	rect := quickActionRect(layout, state, width, height)

	draw.DrawDialogFrameStyled(screen, rect, state.Title,
		styles.DialogQuickActionSurface, styles.DialogQuickActionFrame, styles.DialogQuickActionTitle,
		styles.QuickActionBorderGlyphs())
	_, dbg, _ := styles.DialogQuickActionSurface.Decompose()

	shortcuts := QuickActionShortcuts(state.Items)
	labelX := rect.X + 5
	labelWidth := rect.Width - 6
	if labelWidth < 0 {
		labelWidth = 0
	}
	for row := 0; row < visibleRows; row++ {
		idx := state.ScrollOffset + row
		if idx >= n {
			break
		}
		item := state.Items[idx]
		y := rect.Y + 1 + row

		base := styles.DialogQuickActionText.Background(dbg)
		if state.Selected == idx {
			base = styles.DialogQuickActionListSelected
		}
		primitive.Text(screen, rect.X+1, y, rect.Width-2, "", base)

		if sh := shortcuts[idx]; sh != 0 {
			accent := draw.AccentGlyphStyle(base, styles.DialogQuickActionAccent)
			primitive.Text(screen, rect.X+2, y, 1, string(sh), accent)
		}
		primitive.Text(screen, labelX, y, labelWidth, item.Label, base)
	}
}

// quickActionRect returns the dialog rect: centered, or anchored and clamped inside
// layout (min against layout.Width-w / layout.Height-h, floor 0) — same clamping
// shape as the pulldown menu.
func quickActionRect(layout Layout, state QuickActionState, width, height int) Rect {
	if !state.Anchored {
		return draw.CenteredDialogRect(layout, width, height)
	}
	x := state.AnchorX
	if maxX := layout.Width - width; x > maxX {
		x = maxX
	}
	if x < 0 {
		x = 0
	}
	y := state.AnchorY
	if maxY := layout.Height - height; y > maxY {
		y = maxY
	}
	if y < 0 {
		y = 0
	}
	return Rect{X: x, Y: y, Width: width, Height: height}
}
