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
	title := "Find"
	count := formatIndexedCount(state.IndexedCount)
	if state.Indexing {
		title = fmt.Sprintf("Find (%s%s)", count, string(primitive.Ellipsis))
		if state.WalkWorkers > 0 {
			icon := styles.SymbolMenuJob("scanning")
			title = fmt.Sprintf("%s %d %c", title, state.WalkWorkers, icon)
		}
	} else if state.IndexDone && state.IndexedCount > 0 {
		title = fmt.Sprintf("Find (%s)", count)
	}
	return title
}

const findDialogPreferredWidth = 117 // 50% wider than the history/path picker default (78).

// FindRowIconPainter draws file-list devicons for one find dialog row; nil skips icons.
type FindRowIconPainter func(screen tcell.Screen, x, y int, entry FindEntry, styles theme.Theme)

func DrawFindDialog(screen tcell.Screen, layout Layout, state FindDialogState, ctx DialogRenderContext, paintIcon FindRowIconPainter, selectionSizeLabel string) {
	styles := ctx.Styles
	showIcons := ctx.ShowIcons
	iconLead := ctx.IconLead
	width, height, listH, ok := FindDialogMetrics(layout, state.ShowSearchSelectionsOption)
	if !ok {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, findDialogTitle(state, styles), styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	itemBg := dbg
	primaryCol := rect.X + 2
	inputWidth := rect.Width - 4
	cbCol := rect.X + 1
	fileListCol := primaryCol
	fileListWidth := inputWidth

	primitive.Text(screen, primaryCol, rect.Y+1, inputWidth, "Filter:", styles.DialogText.Background(itemBg))

	filterFocused := state.Focus == 0
	draw.DrawScrollingDialogInput(screen, primaryCol, rect.Y+3, inputWidth, draw.ScrollingInputState{Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll}, filterFocused, false, styles)

	sepAfterFilter := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepAfterFilter, borderStyle)

	cbY := rect.Y + 5
	draw.DrawDialogRadio(screen, cbCol, cbY, "Only directories", 'D', state.OnlyDirectories, state.Focus == state.FindDialogOnlyDirsFocus(), styles)
	radio1W := utf8.RuneCountInString(draw.RadioText("Only directories", state.OnlyDirectories)) + 1
	const cbGap = 4
	draw.DrawDialogRadio(screen, cbCol+radio1W+cbGap, cbY, "Only files", 'L', state.OnlyFiles, state.Focus == state.FindDialogOnlyFilesFocus(), styles)
	radio2W := utf8.RuneCountInString(draw.RadioText("Only files", state.OnlyFiles)) + 1
	volumeX := cbCol + radio1W + cbGap + radio2W + cbGap
	draw.DrawDialogCheckbox(screen, volumeX, cbY, "Stay on current volume", 'V', state.StayOnCurrentVolume, state.Focus == state.FindDialogStayOnVolumeFocus(), styles)
	volumeW := utf8.RuneCountInString(draw.CheckboxText("Stay on current volume", state.StayOnCurrentVolume)) + 1
	draw.DrawDialogCheckbox(screen, volumeX+volumeW+cbGap, cbY, "Include hidden", 'I', state.IncludeHidden, state.Focus == state.FindDialogIncludeHiddenFocus(), styles)
	if state.ShowSearchSelectionsOption {
		draw.DrawDialogCheckbox(screen, cbCol, cbY+1, "Search only from selected directories", 'S', state.SearchOnlySelections, state.Focus == state.FindDialogSelectionsFocus(), styles)
	}

	checkboxRows := 1
	if state.ShowSearchSelectionsOption {
		checkboxRows = 2
	}
	sepAfterCheckbox := rect.Y + 5 + checkboxRows
	draw.DrawDialogHSeparator(screen, rect, sepAfterCheckbox, borderStyle)

	listTop := sepAfterCheckbox + 1
	buttonY := rect.Y + rect.Height - 2
	if maxListH := buttonY - listTop - 1; maxListH < listH {
		listH = maxListH
	}
	if listH < 0 {
		listH = 0
	}
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
		if y >= buttonY {
			break
		}
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
			if ent, ok := state.FindEntryAt(entIdx); ok {
				hasEntry = true
				line = ent.RelLine
				ranges = state.MatchRanges[entIdx]
				if idxInRank < len(state.RankDisplayLines) {
					if dl := state.RankDisplayLines[idxInRank]; dl != "" {
						line = dl
					}
				}
				if state.MarkedPaths != nil {
					marked = state.MarkedPaths[ent.AbsPath(state.RootPath)]
				}
			} else if idxInRank < len(state.RankDisplayLines) {
				line = state.RankDisplayLines[idxInRank]
				if entIdx >= 0 {
					ranges = state.MatchRanges[entIdx]
				}
			}
			isCursor = listFocused && idxInRank == state.Selected
		}
		matchStyle := styles.FuzzyHighlight
		if isCursor {
			baseStyle = styles.DialogOptionRowStyle(true, marked)
			matchStyle = styles.FuzzyHighlightCursor
		} else if marked {
			baseStyle = styles.DialogOptionRowStyle(false, true)
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

	draw.DrawDialogListScrollbar(screen, rect, listTop, listH, len(state.Ranked), state.ListScroll, ctx.ScrollbarStyle, borderStyle, styles)

	sepAfterList := listTop + listH
	if sepAfterList >= buttonY {
		sepAfterList = buttonY - 1
	}
	if selectionSizeLabel != "" {
		labelStyle := styles.DialogStatusSelectionSizeStyle()
		draw.DrawDialogHSeparatorWithCenteredLabel(screen, rect, sepAfterList, borderStyle, labelStyle, selectionSizeLabel)
	} else {
		draw.DrawDialogHSeparator(screen, rect, sepAfterList, borderStyle)
	}

	okFocused := state.Focus == state.FindDialogOKFocus()
	cancelFocused := state.Focus == state.FindDialogCancelFocus()
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, okFocused, cancelFocused, styles)
}
