package dialog

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// EnsureFindListScroll keeps Selected row visible in a list of height listRows.
func EnsureFindListScroll(state *FindDialogState, listRows int) {
	n := len(state.Ranked)
	if n == 0 || listRows <= 0 {
		state.ListScroll = 0
		return
	}
	if state.Selected < 0 {
		state.Selected = 0
	}
	if state.Selected >= n {
		state.Selected = n - 1
	}
	if state.ListScroll > state.Selected {
		state.ListScroll = state.Selected
	}
	if state.Selected >= state.ListScroll+listRows {
		state.ListScroll = state.Selected - listRows + 1
	}
}

// CenterFindListScroll scrolls the list so Selected is vertically centered in the viewport.
// Use this after applying a rank update to avoid jarring jumps.
func CenterFindListScroll(state *FindDialogState, listRows int) {
	n := len(state.Ranked)
	if n == 0 || listRows <= 0 {
		state.ListScroll = 0
		return
	}
	if state.Selected < 0 {
		state.Selected = 0
	}
	if state.Selected >= n {
		state.Selected = n - 1
	}
	scroll := state.Selected - listRows/2
	if scroll < 0 {
		scroll = 0
	}
	maxScroll := n - listRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	state.ListScroll = scroll
}

func formatIndexedCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.2fK", float64(n)/1e3)
	}
	return fmt.Sprintf("%.2fM", float64(n)/1e6)
}

func findDialogTitle(state FindDialogState, styles theme.Theme) string {
	name := "Find"
	if state.JumpMode {
		name = "Jump to directory"
	}
	title := name
	count := formatIndexedCount(state.IndexedCount)
	if state.Indexing {
		title = fmt.Sprintf("%s (%s%s)", name, count, string(primitive.Ellipsis))
		if state.LevelTotal > 0 {
			icon := styles.IconMenuJob("scanning")
			depth := styles.IconDialogDepth()
			if styles.UseNerdfontIcons {
				depth += " " // the glyph renders wide and crowds the number
			}
			title = fmt.Sprintf("%s %s %d · %d%% %c", title, depth, state.Depth, state.LevelDone*100/state.LevelTotal, icon)
		}
	} else if state.IndexDone && state.IndexedCount > 0 {
		title = fmt.Sprintf("%s (%s)", name, count)
	}
	return title
}

// Max-depth input on the query row: icon (or "Depth" label) and its space, then one digit,
// the caret cell and a padding cell; a one-cell gap separates it from the query.
const (
	findDialogDepthInputCells = 3
	findDialogDepthGap        = 1
)

func findDialogDepthFieldWidth(styles theme.Theme) int {
	return draw.LeadingIconCols(styles.IconDialogDepth()) + findDialogDepthInputCells
}

const findDialogPreferredWidth = 125 // preferred frame width; clamped to the terminal in FindDialogMetrics

// FindRowIconPainter draws file-list devicons for one find dialog row; nil skips icons.
type FindRowIconPainter func(screen tcell.Screen, x, y int, entry FindEntry, styles theme.Theme)

// DrawFindDialog paints the find dialog from its widget spec (FindDialogView), then the row
// devicons over the leading cells each list row reserves for them (the overlay escape hatch:
// devicons are colored per entry type by the panel icon painter). selectionSizeLabel is the
// unpadded marked-selection size ("" = none).
func DrawFindDialog(screen tcell.Screen, layout Layout, state FindDialogState, ctx DialogRenderContext, paintIcon FindRowIconPainter, selectionSizeLabel string, rowMarks RowMarksFunc) {
	iconLead := ctx.IconLead
	if !ctx.ShowIcons || paintIcon == nil {
		iconLead = 0
	}
	spec := FindDialogView(&state, FindViewContext{
		Layout: layout, Styles: ctx.Styles, IconLead: iconLead, ScrollbarStyle: ctx.ScrollbarStyle,
		SelectionLabel: selectionSizeLabel, RowMarks: rowMarks,
	})
	p, ok := dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, ctx.Styles)
	if !ok || iconLead == 0 {
		return
	}
	list, _ := p.Item(FindList)
	p.EachListRow(FindList, func(row, y int) {
		if ent, ok := state.FindEntryAt(state.Ranked[row]); ok {
			paintIcon(screen, list.Rect.X, y, ent, ctx.Styles)
		}
	})
}
