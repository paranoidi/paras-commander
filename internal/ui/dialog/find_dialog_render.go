package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func findDialogTitle(state FindDialogState) string {
	title := "Find"
	if state.Indexing {
		title = fmt.Sprintf("Find (%d…)", state.IndexedCount)
	} else if state.IndexDone && state.IndexedCount > 0 {
		title = fmt.Sprintf("Find (%d)", state.IndexedCount)
	}
	return title
}

const findDialogPreferredWidth = 117 // 50% wider than the history/path picker default (78).

// FindRowIconPainter draws file-list devicons for one find dialog row; nil skips icons.
type FindRowIconPainter func(screen tcell.Screen, x, y int, entry FindEntry, styles theme.Theme)

func DrawFindDialog(screen tcell.Screen, layout Layout, state FindDialogState, styles theme.Theme, showIcons bool, iconLead int, paintIcon FindRowIconPainter) {
	width := findDialogPreferredWidth
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 54 {
		return
	}

	listH := layout.Height - 14
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	// top + filter(3) + sep + checkbox(es) + sep + list + sep + buttons
	checkboxRows := 1
	if state.ShowSearchSelectionsOption {
		checkboxRows = 2
	}
	baseHeight := 9 + checkboxRows
	height := baseHeight + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - baseHeight
		if listH < 4 {
			return
		}
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, findDialogTitle(state), styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	leftCol := rect.X + 2
	inputWidth := rect.Width - 4
	cbCol := rect.X + 1
	fileListCol := leftCol
	fileListWidth := inputWidth

	primitive.Text(screen, leftCol, rect.Y+1, inputWidth, "Filter:", styles.DialogText.Background(itemBg))

	filterFocused := state.Focus == 0
	draw.DrawScrollingDialogInput(screen, leftCol, rect.Y+3, inputWidth, state.Query, state.QueryCursor, state.QueryScroll, "", filterFocused, false, styles)

	sepAfterFilter := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepAfterFilter, borderStyle)

	cbY := rect.Y + 5
	draw.DrawDialogCheckbox(screen, cbCol, cbY, "Stay on current volume", 'V', state.StayOnCurrentVolume, state.Focus == 1, styles)
	cb1W := utf8.RuneCountInString(draw.CheckboxText("Stay on current volume", state.StayOnCurrentVolume)) + 1
	const cbGap = 4
	draw.DrawDialogCheckbox(screen, cbCol+cb1W+cbGap, cbY, "Only directories", 'D', state.OnlyDirectories, state.Focus == state.FindDialogOnlyDirsFocus(), styles)
	if state.ShowSearchSelectionsOption {
		draw.DrawDialogCheckbox(screen, cbCol, cbY+1, "Search only from selections", 'S', state.SearchOnlySelections, state.Focus == state.FindDialogSelectionsFocus(), styles)
	}

	sepAfterCheckbox := rect.Y + 5 + checkboxRows
	draw.DrawDialogHSeparator(screen, rect, sepAfterCheckbox, borderStyle)

	listTop := sepAfterCheckbox + 1
	if !showIcons {
		iconLead = 0
	}
	rowWidth := fileListWidth - iconLead
	if rowWidth < 8 {
		rowWidth = 8
	}
	listFocused := state.Focus == 0
	for row := 0; row < listH; row++ {
		y := listTop + row
		idxInRank := state.ListScroll + row
		baseStyle := styles.DialogText.Background(itemBg)
		line := ""
		var ranges []search.Range
		isCursor := false
		marked := false
		var ent FindEntry
		hasEntry := false
		if idxInRank < len(state.Ranked) {
			entIdx := state.Ranked[idxInRank]
			if entIdx >= 0 && entIdx < len(state.Entries) {
				ent = state.Entries[entIdx]
				hasEntry = true
				line = ent.RelLine
				ranges = state.MatchRanges[entIdx]
				if state.MarkedPaths != nil {
					marked = state.MarkedPaths[ent.AbsPath(state.RootPath)]
				}
			}
			isCursor = listFocused && idxInRank == state.Selected
		}
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			baseStyle = styles.DialogOptionActive
			matchStyle = styles.FuzzyHighlightCursor
		} else if marked {
			baseStyle = styles.DialogOptionSelected
		}
		_, bg, _ := baseStyle.Decompose()
		matchStyle = matchStyle.Background(bg)
		textX := fileListCol
		if showIcons && paintIcon != nil && iconLead > 0 {
			if hasEntry {
				paintIcon(screen, fileListCol, y, ent, styles)
			}
			textX = fileListCol + iconLead
		}
		text, spans := fuzzyPathRowContent(line, ranges, rowWidth, matchStyle)
		primitive.StyledText(screen, textX, y, rowWidth, text, baseStyle, spans)
	}

	sepAfterList := listTop + listH
	draw.DrawDialogHSeparator(screen, rect, sepAfterList, borderStyle)

	buttonY := rect.Y + rect.Height - 2
	okFocused := state.Focus == state.FindDialogOKFocus()
	cancelFocused := state.Focus == state.FindDialogCancelFocus()
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)
}
