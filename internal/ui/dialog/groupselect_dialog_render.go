package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
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

	// top(1) + blank(1) + label(1) + blank(1) + input(1) + blank(1) + cb-row1(1) + cb-row2(1) + cb-row3(1) + sep(1) + buttons(1) + bot(1) = 12
	height := 12
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 12 {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	itemBg := dbg
	textX := draw.DialogTextX(rect)
	optionX := draw.DialogOptionX(rect)
	inputWidth := rect.Width - 4

	// Label row (rect.Y+2)
	primitive.Text(screen, textX, rect.Y+2, inputWidth, "Pattern:", styles.DialogText.Background(itemBg))

	// Input row (rect.Y+4, blank row Y+3)
	draw.DrawScrollingDialogInput(screen, textX, rect.Y+4, inputWidth, state.Text, state.TextCursor, state.TextScroll, "", state.Focus == 0, false, styles)

	// Checkbox row 1: Files only (focus 1) | Directories only (focus 2)
	cbRow1Y := rect.Y + 6
	draw.DrawDialogCheckbox(screen, optionX, cbRow1Y, "Files only", 'F', state.FilesOnly, state.Focus == 1, styles)
	cb1W := utf8.RuneCountInString(draw.CheckboxText("Files only", state.FilesOnly)) + 1
	gap := 2
	draw.DrawDialogCheckbox(screen, optionX+cb1W+gap, cbRow1Y, "Directories only", 'D', state.DirsOnly, state.Focus == 2, styles)

	// Checkbox row 2: Case sensitive (focus 3)
	cbRow2Y := rect.Y + 7
	draw.DrawDialogCheckbox(screen, optionX, cbRow2Y, "Case sensitive", 'S', state.CaseSensitive, state.Focus == 3, styles)

	// Checkbox row 3: Using shell patterns (focus 4)
	cbRow3Y := rect.Y + 8
	draw.DrawDialogCheckbox(screen, optionX, cbRow3Y, "Using shell patterns", 'U', state.UseShellPatterns, state.Focus == 4, styles)

	// Separator
	sepY := rect.Y + 9
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

	// Button row
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == 5},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == 6},
	}, styles)
}
