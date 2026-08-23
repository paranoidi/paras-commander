package dialog

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DrawFilterDialog renders the panel Filter modal.
func DrawFilterDialog(screen tcell.Screen, layout Layout, state FilterDialogState, styles theme.Theme) {
	const title = "Filter"

	width := 54
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 30 {
		return
	}

	height := filterDialogHeight(state, layout.Height)
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	textX := draw.DialogTextX(rect)
	optionX := draw.DialogOptionX(rect)
	inputWidth := draw.DialogContentWidth(rect)

	y := rect.Y + 1
	innerBottom := rect.Y + rect.Height - 2

	draw.DrawDialogRadio(screen, optionX, y, "Shell patterns", 'S', state.PatternMode == panel.GroupPatternShell, state.Focus == FilterFocusShellRadio, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optionX, y, "Regular expression", 'R', state.PatternMode == panel.GroupPatternRegex, state.Focus == FilterFocusRegexRadio, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optionX, y, "Simple", 'I', state.PatternMode == panel.GroupPatternSimple, state.Focus == FilterFocusSimpleRadio, styles)
	y++
	if y >= innerBottom {
		return
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	primitive.Text(screen, textX, y, inputWidth, "Pattern:", styles.DialogText.Background(dbg))
	if preview := filterPreviewText(state, styles); preview != "" {
		pw := utf8.RuneCountInString(preview)
		px := textX + inputWidth - pw
		if px > textX+len("Pattern:") {
			primitive.Text(screen, px, y, pw, preview, styles.DialogText.Background(dbg))
		}
	}
	y++
	if y >= innerBottom {
		return
	}
	y++ // blank row between label and input
	if y >= innerBottom {
		return
	}
	draw.DrawScrollingDialogInput(screen, textX, y, inputWidth, draw.ScrollingInputState{Value: state.Text, Cursor: state.TextCursor, Scroll: state.TextScroll}, state.Focus == FilterFocusPattern, filterPatternInvalid(state), styles)
	y++
	if y >= innerBottom {
		return
	}

	if hint := filterPatternHintText(state); hint != "" && y < innerBottom {
		primitive.Text(screen, textX, y, inputWidth, hint, filterPatternHintStyle(styles, dbg))
		y++
		if y >= innerBottom {
			return
		}
	}

	col2X := optionX + utf8.RuneCountInString(draw.CheckboxText("Files only", false)) + 3 // +1 pad +2 gap
	draw.DrawDialogCheckbox(screen, optionX, y, "Files only", 'F', state.FilesOnly, state.Focus == FilterFocusFilesOnly, styles)
	draw.DrawDialogCheckbox(screen, col2X, y, "Directories only", 'D', state.DirsOnly, state.Focus == FilterFocusDirsOnly, styles)
	y++
	if y >= innerBottom {
		return
	}

	if FilterShowsCaseSensitive(state) {
		draw.DrawDialogCheckbox(screen, optionX, y, "Case sensitive", 'E', state.CaseSensitive, state.Focus == FilterFocusCase, styles)
	}
	y++
	if y >= innerBottom {
		return
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)

	form := NewDialogLinearForm(7)
	buttonY := rect.Y + rect.Height - 2
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, state.Focus == form.OKIndex(), state.Focus == form.CancelIndex(), styles)
}

// filterPreviewText formats the live match-count preview shown on the Pattern row:
// "<files> <file-icon> <folders> <folder-icon>", omitting either count when it's zero. Empty
// when the preview is hidden or both counts are zero.
func filterPreviewText(state FilterDialogState, styles theme.Theme) string {
	if !state.PreviewShow {
		return ""
	}
	var parts []string
	if state.PreviewFiles > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", state.PreviewFiles, styles.SymbolFile()))
	}
	if state.PreviewFolders > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", state.PreviewFolders, styles.SymbolFolder()))
	}
	return strings.Join(parts, " ")
}
