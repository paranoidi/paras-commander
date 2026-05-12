package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func drawFileDialog(screen tcell.Screen, layout Layout, state FileDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}

	width := fileDialogWidth(layout.Width, state)
	if width < 20 {
		return
	}

	// Calculate height based on dialog type.
	var height int
	switch state.DialogType {
	case FileDialogDelete:
		height = fileDeleteDialogHeight(state)
	case FileDialogAddBookmark:
		height = 10
	case FileDialogRunForEach:
		helpLines := 0
		if msg := strings.TrimSpace(state.Message); msg != "" {
			helpLines = strings.Count(state.Message, "\n") + 1
		}
		// Help block + separator + fields (label / blank / input per field) + separator + buttons row.
		height = helpLines + 1 + len(state.Fields)*4 + 4
	default:
		if len(state.Fields) > 0 {
			height = len(state.Fields)*4 + 4 // +1 separator row above buttons
		} else {
			height = 5
		}
	}
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 5 {
		height = 5
	}

	dialogTitle := fileDialogTitle(state.DialogType)
	if dialogTitle == "" {
		return
	}

	rect := Rect{
		X:      (layout.Width - width) / 2,
		Y:      (layout.Height - height) / 2,
		Width:  width,
		Height: height,
	}
	borderStyle := drawDialogFrame(screen, rect, dialogTitle, styles)

	switch state.DialogType {
	case FileDialogDelete:
		drawFileDeleteDialogContent(screen, rect, state, styles)
	case FileDialogAddBookmark:
		drawAddBookmarkDialogContent(screen, rect, state, borderStyle, styles)
	case FileDialogRunForEach:
		if len(state.Fields) > 0 {
			drawRunForEachDialogFields(screen, rect, borderStyle, state, styles)
		}
	default:
		if len(state.Fields) > 0 {
			drawMultiFieldDialog(screen, rect, state, styles)
		}
	}

	// Draw buttons at the bottom.
	buttonY := rect.Y + rect.Height - 2
	drawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	if state.DialogType == FileDialogDelete {
		drawDeleteButtons(screen, rect, buttonY, state, styles)
	} else {
		drawOkCancelButtons(screen, rect, buttonY, state, styles)
	}
}

func fileDialogTitle(dialogType FileDialogType) string {
	switch dialogType {
	case FileDialogRename:
		return "Rename"
	case FileDialogMkdir:
		return "Create directory"
	case FileDialogDelete:
		return "Delete"
	case FileDialogChmod:
		return "Chmod"
	case FileDialogChown:
		return "Chown"
	case FileDialogSymlink:
		return "Create symlink"
	case FileDialogHardlink:
		return "Create hardlink"
	case FileDialogAddBookmark:
		return "Add bookmark"
	case FileDialogRunForEach:
		return "Run for each"
	default:
		return ""
	}
}

func fileDialogWidth(screenWidth int, state FileDialogState) int {
	minWidth := 30
	// Compute the widest field label + value.
	for _, field := range state.Fields {
		labelLength := utf8.RuneCountInString(field.Label)
		contentWidth := max(labelLength, utf8.RuneCountInString(field.Value))
		fw := contentWidth + 6
		if fw > minWidth {
			minWidth = fw
		}
	}
	// For delete dialog, use the message.
	if state.DialogType == FileDialogDelete {
		lineWidth := 30
		for _, line := range strings.Split(state.Message, "\n") {
			lw := utf8.RuneCountInString(line) + 4
			if lw > lineWidth {
				lineWidth = lw
			}
		}
		if lineWidth > minWidth {
			minWidth = lineWidth
		}
	}
	// For add-bookmark dialog, also fit the read-only path line stored in Message.
	if state.DialogType == FileDialogAddBookmark {
		lineWidth := utf8.RuneCountInString(state.Message) + 4
		if lineWidth > minWidth {
			minWidth = lineWidth
		}
	}
	if state.DialogType == FileDialogRunForEach && strings.TrimSpace(state.Message) != "" {
		for _, line := range strings.Split(state.Message, "\n") {
			lw := utf8.RuneCountInString(line) + 4
			if lw > minWidth {
				minWidth = lw
			}
		}
	}
	if minWidth > screenWidth-4 {
		minWidth = screenWidth - 4
	}
	return max(20, minWidth)
}

func fileDeleteDialogHeight(state FileDialogState) int {
	lineCount := 1
	if state.Message != "" {
		lineCount = strings.Count(state.Message, "\n") + 1
	}
	height := lineCount + 4 // message + top/bottom borders + separator + buttons
	if height < 5 {
		height = 5
	}
	return height
}

func drawRunForEachDialogFields(screen tcell.Screen, rect Rect, borderStyle tcell.Style, state FileDialogState, styles theme.Theme) {
	_, dbg, _ := styles.DialogSurface.Decompose()
	fieldStartY := rect.Y + 1
	if msg := strings.TrimSpace(state.Message); msg != "" {
		labelStyle := styles.DialogText.Background(dbg)
		y := fieldStartY
		for _, line := range strings.Split(state.Message, "\n") {
			if y >= rect.Y+rect.Height-3 {
				break
			}
			lineWidth := rect.Width - 4
			if lineWidth > 0 {
				primitive.Text(screen, rect.X+2, y, lineWidth, line, labelStyle)
			}
			y++
		}
		drawDialogHSeparator(screen, rect, y, borderStyle)
		fieldStartY = y + 1
	}
	for i, field := range state.Fields {
		y := fieldStartY + i*4
		if y >= rect.Y+rect.Height-3 {
			break
		}
		labelWidth := rect.Width - 4
		if labelWidth <= 0 {
			continue
		}
		fieldStyle := styles.DialogText.Background(dbg)
		primitive.Text(screen, rect.X+2, y, labelWidth, field.Label+":", fieldStyle)

		inputY := y + 2
		if inputY >= rect.Y+rect.Height-3 {
			continue
		}
		drawInputField(screen, rect.X+2, inputY, rect.Width-4, field, i == state.FocusedField, styles)
	}
}

func drawMultiFieldDialog(screen tcell.Screen, rect Rect, state FileDialogState, styles theme.Theme) {
	fieldStartY := rect.Y + 1
	for i, field := range state.Fields {
		y := fieldStartY + i*4
		if y >= rect.Y+rect.Height-3 {
			break
		}
		labelWidth := rect.Width - 4
		if labelWidth <= 0 {
			continue
		}
		fieldStyle := styles.DialogText
		if state.Message != "" && i == state.FocusedField {
			fieldStyle = styles.StatusWarn
		}
		_, dbg, _ := styles.DialogSurface.Decompose()
		fieldStyle = fieldStyle.Background(dbg)
		primitive.Text(screen, rect.X+2, y, labelWidth, field.Label+":", fieldStyle)

		// Blank line between label and input.
		inputY := y + 2
		if inputY >= rect.Y+rect.Height-3 {
			continue
		}
		drawInputField(screen, rect.X+2, inputY, rect.Width-4, field, i == state.FocusedField, styles)
	}
}

func drawInputField(screen tcell.Screen, x, y, width int, field FileDialogField, focused bool, styles theme.Theme) {
	if field.PathPicker && width > 2 {
		pickerFocused := field.PickerFocused && focused
		drawPathInputRow(screen, x, y, width, field, focused, pickerFocused, false, styles)
		return
	}
	if width <= 0 {
		return
	}
	style, placeholderStyle := styles.DialogInputPair(focused)
	prefillPending := field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill
	textStyle := style
	if prefillPending {
		textStyle = placeholderStyle
	}

	runes := []rune(field.Value)
	length := len(runes)
	cursor, scroll := EnsureScrollInputVisible(length, field.Cursor, 0, width)

	for i := 0; i < width; i++ {
		idx := scroll + i
		ch := ' '
		if idx < length {
			ch = runes[idx]
		}
		st := textStyle
		if focused && idx == cursor {
			st = textStyle.Reverse(true)
		}
		screen.SetContent(x+i, y, ch, nil, st)
	}

	if scroll > 0 && (!focused || cursor != scroll) {
		screen.SetContent(x, y, '◀', nil, style)
	}
	if scroll+width < length && (!focused || cursor != scroll+width-1) {
		screen.SetContent(x+width-1, y, '▶', nil, style)
	}
}

// drawPathInputRow draws text in the first width-2 cells, the path-picker glyph in the
// next cell, and leaves the rightmost cell blank (row background).
// When pathInvalid is true, uses dialog.input.*.error for the row (see Theme.DialogInputBaseStyle).
// The text area scrolls horizontally to keep the caret visible; overflow markers (◀/▶) appear on
// the edge text cells when content is hidden in that direction.
func drawPathInputRow(screen tcell.Screen, x, y, width int, field FileDialogField, rowFocused bool, pickerFocused bool, pathInvalid bool, styles theme.Theme) {
	if width <= 2 {
		return
	}
	textW := width - 2
	var style, placeholderStyle tcell.Style
	if pathInvalid {
		style = styles.DialogInputBaseStyle(rowFocused, true)
		placeholderStyle = style
	} else {
		style, placeholderStyle = styles.DialogInputPair(rowFocused)
	}
	prefillPending := field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill
	textStyle := style
	if prefillPending && !pathInvalid {
		textStyle = placeholderStyle
	}

	primitive.Text(screen, x, y, width, "", style)

	runes := []rune(field.Value)
	length := len(runes)
	textFocused := rowFocused && !pickerFocused
	cursor, scroll := EnsureScrollInputVisible(length, field.Cursor, 0, textW)

	for i := 0; i < textW; i++ {
		idx := scroll + i
		ch := ' '
		if idx < length {
			ch = runes[idx]
		}
		st := textStyle
		if textFocused && idx == cursor {
			st = textStyle.Reverse(true)
		}
		screen.SetContent(x+i, y, ch, nil, st)
	}

	if scroll > 0 && (!textFocused || cursor != scroll) {
		screen.SetContent(x, y, '◀', nil, style)
	}
	if scroll+textW < length && (!textFocused || cursor != scroll+textW-1) {
		screen.SetContent(x+textW-1, y, '▶', nil, style)
	}

	glyphX := x + textW
	symStr := styles.SymbolPathPicker()
	symR := ' '
	if sr := []rune(symStr); len(sr) > 0 {
		symR = sr[0]
	}
	glyphStyle := textStyle
	if rowFocused && pickerFocused {
		glyphStyle = styles.DialogAccent
	}
	screen.SetContent(glyphX, y, symR, nil, glyphStyle)

	tailX := x + width - 1
	screen.SetContent(tailX, y, ' ', nil, style)
}

// fileDialogFocusIndex returns the focus index for the OK/Yes button.
func fileDialogOKFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogDelete {
		return 0
	}
	return len(state.Fields)
}

// fileDialogCancelFocusIndex returns the focus index for the Cancel/No button.
func fileDialogCancelFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogDelete {
		return 1
	}
	return len(state.Fields) + 1
}

func drawOkCancelButtons(screen tcell.Screen, rect Rect, y int, state FileDialogState, styles theme.Theme) {
	okFocusIdx := fileDialogOKFocusIndex(state)
	cancelFocusIdx := fileDialogCancelFocusIndex(state)

	drawDialogButtonRowCentered(screen, rect, y, []DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.FocusedField == okFocusIdx},
		{Label: "Cancel", Shortcut: 'C', Focused: state.FocusedField == cancelFocusIdx},
	}, styles)
}

func drawDeleteButtons(screen tcell.Screen, rect Rect, y int, state FileDialogState, styles theme.Theme) {
	drawDialogButtonRowCentered(screen, rect, y, []DialogButtonSpec{
		{Label: "Yes", Shortcut: 'Y', Focused: state.FocusedField == 0},
		{Label: "No", Shortcut: 'N', Focused: state.FocusedField == 1},
	}, styles)
}

func drawAddBookmarkDialogContent(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	if rect.Width < 4 || rect.Height < 10 {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	leftCol := rect.X + 2
	innerWidth := rect.Width - 4

	primitive.Text(screen, leftCol, rect.Y+1, innerWidth, "Path:", textStyle)
	pathValue := state.Message
	if utf8.RuneCountInString(pathValue) > innerWidth {
		pathValue = primitive.TruncateRight(pathValue, innerWidth)
	}
	primitive.Text(screen, leftCol, rect.Y+2, innerWidth, pathValue, textStyle)

	drawDialogHSeparator(screen, rect, rect.Y+3, borderStyle)

	primitive.Text(screen, leftCol, rect.Y+4, innerWidth, "Name:", textStyle)

	if len(state.Fields) > 0 {
		focused := state.FocusedField == 0
		drawInputField(screen, leftCol, rect.Y+6, innerWidth, state.Fields[0], focused, styles)
	}
}

func drawFileDeleteDialogContent(screen tcell.Screen, rect Rect, state FileDialogState, styles theme.Theme) {
	if state.Message == "" {
		return
	}
	lines := strings.Split(state.Message, "\n")
	_, dbg, _ := styles.DialogSurface.Decompose()
	style := styles.DialogText.Background(dbg)
	if state.DialogType == FileDialogDelete {
		style = styles.StatusWarn.Background(dbg)
	}
	for i, line := range lines {
		y := rect.Y + 1 + i
		if y >= rect.Y+rect.Height-3 {
			break
		}
		primitive.Text(screen, rect.X+2, y, rect.Width-4, line, style)
	}
}
