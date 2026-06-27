package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawGroupSelectDialog(screen tcell.Screen, layout Layout, state GroupSelectState, styles theme.Theme) {
	title := "Unselect group"
	if state.Mode == "select" {
		title = "Select group"
	}

	width := 54
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 30 {
		return
	}

	height := groupSelectDialogHeight(state, layout.Height)
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	itemBg := dbg
	textX := draw.DialogTextX(rect)
	optionX := draw.DialogOptionX(rect)
	inputWidth := draw.DialogContentWidth(rect)

	y := rect.Y + 1
	innerBottom := rect.Y + rect.Height - 2

	draw.DrawDialogRadio(screen, optionX, y, "Shell patterns", 'S', state.PatternMode == panel.GroupPatternShell, state.Focus == GroupSelectFocusShellRadio, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optionX, y, "Regular expression", 'R', state.PatternMode == panel.GroupPatternRegex, state.Focus == GroupSelectFocusRegexRadio, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optionX, y, "Simple", 'I', state.PatternMode == panel.GroupPatternSimple, state.Focus == GroupSelectFocusSimpleRadio, styles)
	y++
	if y >= innerBottom {
		return
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	primitive.Text(screen, textX, y, inputWidth, "Pattern:", styles.DialogText.Background(itemBg))
	y++
	if y >= innerBottom {
		return
	}
	y++ // blank row between label and input
	if y >= innerBottom {
		return
	}
	draw.DrawScrollingDialogInput(screen, textX, y, inputWidth, state.Text, state.TextCursor, state.TextScroll, "", state.Focus == GroupSelectFocusPattern, false, styles)
	y++
	if y >= innerBottom {
		return
	}

	if hint := groupSelectPatternHintText(state); hint != "" && y < innerBottom {
		primitive.Text(screen, textX, y, inputWidth, hint, groupSelectPatternHintStyle(styles, dbg))
		y++
		if y >= innerBottom {
			return
		}
	}

	// col2X aligns the second checkbox on every two-column row.
	col2X := optionX + utf8.RuneCountInString(draw.CheckboxText("Include meta columns", false)) + 3 // +1 pad +2 gap
	draw.DrawDialogCheckbox(screen, optionX, y, "Files only", 'F', state.FilesOnly, state.Focus == GroupSelectFocusFilesOnly, styles)
	draw.DrawDialogCheckbox(screen, col2X, y, "Directories only", 'D', state.DirsOnly, state.Focus == GroupSelectFocusDirsOnly, styles)
	y++
	if y >= innerBottom {
		return
	}

	if GroupSelectShowsCaseSensitive(state) {
		draw.DrawDialogCheckbox(screen, optionX, y, "Case sensitive", 'E', state.CaseSensitive, state.Focus == GroupSelectFocusCase, styles)
	}
	y++
	if y >= innerBottom {
		return
	}

	if state.MetaColumnCount > 0 {
		draw.DrawDialogCheckbox(screen, optionX, y, "Include meta columns", 'M', state.IncludeMetaColumns, state.Focus == GroupSelectFocusIncludeMeta, styles)
		draw.DrawDialogCheckbox(screen, col2X, y, "Only meta columns", 'N', state.OnlyMetaColumns, state.Focus == GroupSelectFocusOnlyMeta, styles)
		y++
		if y >= innerBottom {
			return
		}
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)

	form := NewDialogLinearForm(7)
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == form.OKIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == form.CancelIndex()},
	}, styles)
}
