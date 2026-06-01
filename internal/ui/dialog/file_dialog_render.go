package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DeleteRowIconPainter draws file-list devicons for one delete dialog row; nil skips icons.
type DeleteRowIconPainter func(screen tcell.Screen, x, y int, entry DeleteListEntry, styles theme.Theme)

func DrawFileDialog(screen tcell.Screen, layout Layout, state FileDialogState, styles theme.Theme, showIcons bool, deleteIconLead int, paintDeleteIcon DeleteRowIconPainter) {
	if !state.Open {
		return
	}

	width := fileDialogWidth(layout.Width, state, deleteIconLead)
	if width < 20 {
		return
	}

	// Calculate height based on dialog type.
	var height int
	switch state.DialogType {
	case FileDialogDelete:
		height = fileDeleteDialogHeight(layout.Height, state)
	case FileDialogAddBookmark:
		height = 10
	case FileDialogRunForEach:
		helpLines := 0
		if msg := strings.TrimSpace(state.Message); msg != "" {
			helpLines = strings.Count(state.Message, "\n") + 1
		}
		// Help block + separator + fields (label / blank / input per field) + separator + buttons row.
		height = helpLines + 1 + len(state.Fields)*4 + 4
	case FileDialogMassRename:
		height = massRenameDialogHeight(layout.Height, state)
	default:
		if renameToolActive(state) {
			// Preview label + blank + preview row + separator + options + separator + buttons.
			height = renameToolDialogHeight()
		} else if len(state.Fields) > 0 {
			height = len(state.Fields)*4 + 4 // +1 separator row above buttons
		} else {
			height = 5
		}
		if mkdirHasActions(state) {
			// Separator + 3 radio rows added above the buttons separator.
			height += 1 + mkdirActionRowCount
		}
		if renameHasFocusCheckbox(state) {
			height += 1 + renameFocusCheckboxRowCount
		}
	}
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 5 {
		height = 5
	}

	dialogTitle := fileDialogOuterTitle(state)
	if dialogTitle == "" {
		return
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, dialogTitle, styles)

	switch state.DialogType {
	case FileDialogDelete:
		drawFileDeleteDialogContent(screen, rect, state, styles, showIcons, deleteIconLead, paintDeleteIcon)
	case FileDialogAddBookmark:
		drawAddBookmarkDialogContent(screen, rect, state, borderStyle, styles)
	case FileDialogRunForEach:
		if len(state.Fields) > 0 {
			drawRunForEachDialogFields(screen, rect, borderStyle, state, styles)
		}
	case FileDialogMassRename:
		drawMassRenameDialog(screen, rect, state, borderStyle, styles)
	default:
		if renameToolActive(state) {
			drawRenameToolContent(screen, rect, state, borderStyle, styles)
		} else if len(state.Fields) > 0 {
			drawMultiFieldDialog(screen, rect, state, styles)
		}
		if mkdirHasActions(state) {
			drawMkdirActionRows(screen, rect, state, borderStyle, styles)
		}
		if renameHasFocusCheckbox(state) {
			drawRenameFocusCheckbox(screen, rect, state, borderStyle, styles)
		}
	}

	// Draw buttons at the bottom.
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	if state.DialogType == FileDialogDelete {
		drawDeleteButtons(screen, rect, buttonY, state, styles)
	} else {
		drawOkCancelButtons(screen, rect, buttonY, state, styles)
	}
}

func renameToolActive(state FileDialogState) bool {
	return state.DialogType == FileDialogRename && state.RenamePhase != RenamePhaseMain
}

func renameToolDialogHeight() int { return 10 }

func fileDialogOuterTitle(state FileDialogState) string {
	if state.DialogType == FileDialogRename {
		switch state.RenamePhase {
		case RenamePhaseSanitize:
			return "Sanitize"
		case RenamePhaseSlugify:
			return "Slugify"
		default:
			return "Rename"
		}
	}
	return fileDialogTitle(state.DialogType)
}

func fileDialogTitle(dialogType FileDialogType) string {
	switch dialogType {
	case FileDialogRename:
		return "Rename"
	case FileDialogMkdir:
		return "Create directory"
	case FileDialogDelete:
		return "Delete ?"
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
	case FileDialogMassRename:
		return "Mass rename"
	case FileDialogExtract:
		return "Extract"
	case FileDialogSFTPConnect:
		return "SFTP"
	case FileDialogSFTPPassword:
		return "SSH password"
	default:
		return ""
	}
}

func fileDialogWidth(screenWidth int, state FileDialogState, deleteListIconLead int) int {
	minWidth := 30
	// Field row width follows labels only; values scroll in drawInputField / drawPathInputRow.
	for _, field := range state.Fields {
		fw := utf8.RuneCountInString(field.Label) + 6
		if fw > minWidth {
			minWidth = fw
		}
	}
	if len(state.Fields) > 0 {
		minWidth = max(minWidth, PreferredFormDialogWidth)
	}
	// For delete dialog, use summary, warning, and listed names (plus devicon strip when shown).
	if state.DialogType == FileDialogDelete {
		iconLead := deleteListIconLead
		if iconLead < 0 {
			iconLead = 0
		}
		lineWidth := 30
		for _, line := range []string{state.DeleteSummary, state.DeleteWarning} {
			if line == "" {
				continue
			}
			lw := utf8.RuneCountInString(line) + 4
			if lw > lineWidth {
				lineWidth = lw
			}
		}
		for _, entry := range state.DeleteEntries {
			lw := utf8.RuneCountInString(entry.Name) + 4 + iconLead
			if lw > lineWidth {
				lineWidth = lw
			}
		}
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
	if mkdirHasActions(state) {
		// Radios render as " (*) Label" with a leading marker; reserve room for the
		// widest label plus the marker glyphs and outer dialog padding (1+marker+label+1+border).
		for _, r := range MkdirActionRadioSpecs() {
			lw := utf8.RuneCountInString(r.Label) + 8
			if lw > minWidth {
				minWidth = lw
			}
		}
	}
	if renameHasFocusCheckbox(state) {
		lw := utf8.RuneCountInString(draw.CheckboxText("Focus after rename", true)) + 4
		if lw > minWidth {
			minWidth = lw
		}
	}
	if renameToolActive(state) {
		for _, label := range renameToolOptionLabels(state) {
			lw := utf8.RuneCountInString(draw.CheckboxText(label, true)) + 4
			if lw > minWidth {
				minWidth = lw
			}
		}
		if len(state.Fields) > 0 {
			pvw := utf8.RuneCountInString(renameToolPreviewText(state)) + 4
			if pvw > minWidth {
				minWidth = pvw
			}
			pl := utf8.RuneCountInString("Preview:") + 4
			if pl > minWidth {
				minWidth = pl
			}
		}
	}
	if state.DialogType == FileDialogMassRename {
		for _, label := range []string{
			"Simple (replace text)",
			"Regular expression",
			"Case insensitive find",
			"Pattern",
			"Replacement",
		} {
			lw := utf8.RuneCountInString(label) + 8
			if lw > minWidth {
				minWidth = lw
			}
		}
		for i := 0; i < len(state.MassRenamePreviewBefore); i++ {
			lb := state.MassRenamePreviewBefore[i]
			if strings.HasPrefix(lb, "!") {
				continue
			}
			lw := utf8.RuneCountInString(lb)
			rw := 0
			if i < len(state.MassRenamePreviewAfter) {
				rw = utf8.RuneCountInString(state.MassRenamePreviewAfter[i])
			}
			// Two equal columns plus one space between: inner >= 2*max(lw,rw)+1; outer adds horizontal padding.
			pairOuter := 2*max(lw, rw) + 1 + 4
			if pairOuter > minWidth {
				minWidth = pairOuter
			}
		}
		if h := massRenamePatternHintText(state); h != "" {
			hw := utf8.RuneCountInString(h) + 4
			if hw > minWidth {
				minWidth = hw
			}
		}
		if h := massRenameReplacementHintText(state); h != "" {
			hw := utf8.RuneCountInString(h) + 4
			if hw > minWidth {
				minWidth = hw
			}
		}
	}
	if minWidth > screenWidth-4 {
		minWidth = screenWidth - 4
	}
	return max(20, minWidth)
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
		draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
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
			fieldStyle = styles.MessageWarn
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
	invalid := field.InputInvalid
	var style, placeholderStyle tcell.Style
	if invalid {
		style = styles.DialogInputBaseStyle(focused, true)
		placeholderStyle = style
	} else {
		style, placeholderStyle = styles.DialogInputPair(focused)
	}
	prefillPending := field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill
	if prefillPending {
		textStyle := placeholderStyle
		if invalid {
			textStyle = style
		}
		markerStyle := styles.DialogInputBaseStyle(focused, false)
		runes := []rune(field.Value)
		length := len(runes)
		cursor, scroll := draw.EnsureScrollInputVisible(length, field.Cursor, field.Scroll, width)
		lay := draw.ScrollingInputLayoutFor(scroll, width, length)
		for i := 0; i < lay.TextCols; i++ {
			idx := scroll + i
			ch := ' '
			if idx < length {
				ch = runes[idx]
			}
			st := textStyle
			if focused && idx == cursor {
				st = textStyle.Reverse(true)
			}
			screen.SetContent(x+lay.LeftPad+i, y, ch, nil, st)
		}
		if lay.LeftPad > 0 {
			screen.SetContent(x, y, '◀', nil, markerStyle)
		}
		if lay.RightPad > 0 {
			screen.SetContent(x+width-1, y, '▶', nil, markerStyle)
		}
		return
	}
	draw.PaintScrollingInputContent(
		screen, x, y, width,
		field.Value, "",
		field.Cursor, field.Scroll,
		focused, invalid, focused,
		styles,
	)
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
	rowStyle := styles.DialogInputBaseStyle(rowFocused, false)
	committedStyle := styles.DialogInputBaseStyle(rowFocused, pathInvalid)
	_, placeholderStyle := styles.DialogInputPair(rowFocused)
	prefillPending := field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill
	textFocused := rowFocused && !pickerFocused

	primitive.Text(screen, x, y, width, "", rowStyle)

	if prefillPending {
		textStyle := placeholderStyle
		if pathInvalid {
			textStyle = committedStyle
		}
		markerStyle := styles.DialogInputBaseStyle(rowFocused, false)
		runes := []rune(field.Value)
		length := len(runes)
		cursor, scroll := draw.EnsureScrollInputVisible(length, field.Cursor, field.Scroll, textW)
		lay := draw.ScrollingInputLayoutFor(scroll, textW, length)
		for i := 0; i < lay.TextCols; i++ {
			idx := scroll + i
			ch := ' '
			if idx < length {
				ch = runes[idx]
			}
			st := textStyle
			if textFocused && idx == cursor {
				st = textStyle.Reverse(true)
			}
			screen.SetContent(x+lay.LeftPad+i, y, ch, nil, st)
		}
		if lay.LeftPad > 0 {
			screen.SetContent(x, y, '◀', nil, markerStyle)
		}
		if lay.RightPad > 0 {
			screen.SetContent(x+textW-1, y, '▶', nil, markerStyle)
		}
	} else {
		draw.PaintScrollingInputContent(
			screen, x, y, textW,
			field.Value, field.CompletionSuffix,
			field.Cursor, field.Scroll,
			textFocused, pathInvalid, rowFocused,
			styles,
		)
	}

	glyphX := x + textW
	symStr := styles.SymbolPathPicker()
	symR := ' '
	if sr := []rune(symStr); len(sr) > 0 {
		symR = sr[0]
	}
	glyphStyle := rowStyle
	if prefillPending && !pathInvalid {
		glyphStyle = placeholderStyle
	}
	if rowFocused && pickerFocused {
		glyphStyle = styles.DialogAccent
	}
	screen.SetContent(glyphX, y, symR, nil, glyphStyle)

	tailX := x + width - 1
	screen.SetContent(tailX, y, ' ', nil, rowStyle)
}

// fileDialogFocusIndex returns the focus index for the OK/Yes button.
func fileDialogOKFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogDelete {
		return 0
	}
	if state.DialogType == FileDialogMassRename {
		return massRenameContentEnd(state)
	}
	if renameToolActive(state) {
		return renameToolOptionCount()
	}
	return len(state.Fields) + mkdirExtraFocusRows(state) + renameExtraFocusRows(state)
}

// fileDialogCancelFocusIndex returns the focus index for the Cancel/No button.
func fileDialogCancelFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogDelete {
		return 1
	}
	if state.DialogType == FileDialogMassRename {
		return massRenameContentEnd(state) + 1
	}
	if renameToolActive(state) {
		return renameToolOptionCount() + 1
	}
	return len(state.Fields) + mkdirExtraFocusRows(state) + renameExtraFocusRows(state) + 1
}

func renameToolOptionCount() int { return 2 }

func renameToolOptionLabels(state FileDialogState) []string {
	if state.RenamePhase == RenamePhaseSanitize {
		return []string{`Replace "." with space`, `Replace "_" with space`}
	}
	return []string{`Replace space with "."`, `Replace space with "_"`}
}

// renameToolPreviewText returns the current name as it would look after applying
// the selected sanitize or slugify options (for the preview row).
func renameToolPreviewText(state FileDialogState) string {
	if len(state.Fields) < 1 {
		return ""
	}
	v := state.Fields[0].Value
	switch state.RenamePhase {
	case RenamePhaseSanitize:
		return ApplyRenameSanitize(v, state.RenameSanitizeDots, state.RenameSanitizeUnderscores)
	case RenamePhaseSlugify:
		return ApplyRenameSlugify(v, state.RenameSlugifySep)
	default:
		return v
	}
}

func drawRenameToolContent(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	leftCol := rect.X + 2
	innerWidth := rect.Width - 4
	innerBottom := rect.Y + rect.Height - 2
	y := rect.Y + 1
	if y >= innerBottom || innerWidth <= 0 {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	labelStyle := styles.DialogText.Background(dbg)
	primitive.Text(screen, leftCol, y, innerWidth, "Preview:", labelStyle)
	y += 2 // blank line between label and preview value (AGENTS.md dialog input layout)
	if y >= innerBottom {
		return
	}
	preview := renameToolPreviewText(state)
	if utf8.RuneCountInString(preview) > innerWidth {
		preview = primitive.TruncateRight(preview, innerWidth)
	}
	primitive.Text(screen, leftCol, y, innerWidth, preview, labelStyle)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}
	if state.RenamePhase == RenamePhaseSanitize {
		draw.DrawDialogCheckbox(screen, leftCol, y, `Replace "." with space`, '.', state.RenameSanitizeDots, state.FocusedField == 0, styles)
		y++
		if y < innerBottom {
			draw.DrawDialogCheckbox(screen, leftCol, y, `Replace "_" with space`, '_', state.RenameSanitizeUnderscores, state.FocusedField == 1, styles)
		}
	} else {
		dotSel := state.RenameSlugifySep == RenameSlugifyDot
		usSel := state.RenameSlugifySep == RenameSlugifyUnderscore
		draw.DrawDialogRadio(screen, leftCol, y, `Replace space with "."`, '.', dotSel, state.FocusedField == 0, styles)
		y++
		if y < innerBottom {
			draw.DrawDialogRadio(screen, leftCol, y, `Replace space with "_"`, '_', usSel, state.FocusedField == 1, styles)
		}
	}
}

// mkdirActionRowCount is the number of radio rows shown for mkdir post-actions
// when MkdirShowActions is enabled.
const mkdirActionRowCount = 3

// mkdirHasActions reports whether the mkdir dialog should render and accept
// post-mkdir action radio rows.
func mkdirHasActions(state FileDialogState) bool {
	return state.DialogType == FileDialogMkdir && state.MkdirShowActions
}

// mkdirExtraFocusRows returns the number of focus rows contributed by the
// mkdir radio section, or 0 when not applicable.
func mkdirExtraFocusRows(state FileDialogState) int {
	if mkdirHasActions(state) {
		return mkdirActionRowCount
	}
	return 0
}

const renameFocusCheckboxRowCount = 1

// renameHasFocusCheckbox reports whether the single-file rename main dialog
// should render and accept the focus-after-rename checkbox.
func renameHasFocusCheckbox(state FileDialogState) bool {
	return state.DialogType == FileDialogRename && state.RenamePhase == RenamePhaseMain
}

// renameExtraFocusRows returns the number of focus rows contributed by the
// rename focus checkbox, or 0 when not applicable.
func renameExtraFocusRows(state FileDialogState) int {
	if renameHasFocusCheckbox(state) {
		return renameFocusCheckboxRowCount
	}
	return 0
}

// drawRenameFocusCheckbox draws the focus-after-rename checkbox under the name input.
func drawRenameFocusCheckbox(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	if !renameHasFocusCheckbox(state) || len(state.Fields) == 0 {
		return
	}
	fieldsBottom := rect.Y + 1 + len(state.Fields)*4
	sepY := fieldsBottom
	if sepY >= rect.Y+rect.Height-2 {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)
	y := sepY + 1
	if y >= rect.Y+rect.Height-2 {
		return
	}
	leftCol := rect.X + 1
	focusIdx := len(state.Fields)
	draw.DrawDialogCheckbox(screen, leftCol, y, "Focus after rename", 'F', state.RenameFocusAfter, state.FocusedField == focusIdx, styles)
}

// drawMkdirActionRows draws the radio button section under the directory-name input
// for the mkdir-with-selections dialog. Focus indices for the radio rows start
// immediately after len(state.Fields).
func drawMkdirActionRows(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	if !mkdirHasActions(state) || len(state.Fields) == 0 {
		return
	}
	// drawMultiFieldDialog lays out each field as: label row, blank row, input row, blank row.
	// The first row after the last field block sits at rect.Y + 1 + len(Fields)*4.
	fieldsBottom := rect.Y + 1 + len(state.Fields)*4
	sepY := fieldsBottom
	if sepY >= rect.Y+rect.Height-2 {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)
	leftCol := rect.X + 2
	radios := MkdirActionRadioSpecs()
	baseFocus := len(state.Fields)
	for i, r := range radios {
		y := sepY + 1 + i
		if y >= rect.Y+rect.Height-2 {
			break
		}
		draw.DrawDialogRadio(screen, leftCol, y, r.Label, r.Shortcut, state.MkdirAction == r.Action, state.FocusedField == baseFocus+i, styles)
	}
}

func drawOkCancelButtons(screen tcell.Screen, rect Rect, y int, state FileDialogState, styles theme.Theme) {
	okFocusIdx := fileDialogOKFocusIndex(state)
	cancelFocusIdx := fileDialogCancelFocusIndex(state)
	okDisabled := state.DialogType == FileDialogMassRename && !FileDialogMassRenameOKEnabled(state)

	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.FocusedField == okFocusIdx, Disabled: okDisabled},
		{Label: "Cancel", Shortcut: 'C', Focused: state.FocusedField == cancelFocusIdx},
	}, styles)
}

// FileDialogOKFocusIndex returns the FocusedField index of the OK button.
func FileDialogOKFocusIndex(state FileDialogState) int {
	return fileDialogOKFocusIndex(state)
}

// FileDialogCancelFocusIndex returns the FocusedField index of the Cancel button.
func FileDialogCancelFocusIndex(state FileDialogState) int {
	return fileDialogCancelFocusIndex(state)
}

func drawDeleteButtons(screen tcell.Screen, rect Rect, y int, state FileDialogState, styles theme.Theme) {
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "Yes", Shortcut: 'Y', Focused: state.FocusedField == 0, Destructive: true},
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

	draw.DrawDialogHSeparator(screen, rect, rect.Y+3, borderStyle)

	primitive.Text(screen, leftCol, rect.Y+4, innerWidth, "Name:", textStyle)

	if len(state.Fields) > 0 {
		focused := state.FocusedField == 0
		drawInputField(screen, leftCol, rect.Y+6, innerWidth, state.Fields[0], focused, styles)
	}
}
