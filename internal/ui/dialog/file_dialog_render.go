package dialog

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// DeleteRowIconPainter draws file-list devicons for one delete dialog row; nil skips icons.
type DeleteRowIconPainter func(screen tcell.Screen, x, y int, entry DeleteListEntry, styles theme.Theme)

// FileDialogRect returns the outer rect DrawFileDialog will use for the given
// state, and ok=false when the dialog is not drawable (closed or width < 20).
// It is the single source of truth for the dialog's on-screen geometry, so the
// app can compare it across a keystroke to decide whether a dialog-rect overlay
// is valid (unchanged geometry) or a full render is required (shrunk geometry).
func FileDialogRect(layout Layout, state FileDialogState, deleteIconLead int) (Rect, bool) {
	if !state.Open {
		return Rect{}, false
	}

	width := fileDialogWidth(layout.Width, state, deleteIconLead)
	if width < 20 {
		return Rect{}, false
	}

	// Calculate height based on dialog type.
	var height int
	switch state.DialogType {
	case FileDialogDelete:
		height = fileDeleteDialogHeight(layout.Height, state)
	case FileDialogAddBookmark:
		height = 10
	case FileDialogRunForEach:
		if state.RunForEachHistoryOpen {
			height = runForEachHistoryPickerDialogHeight(layout.Height)
			break
		}
		helpLines := 0
		if msg := strings.TrimSpace(state.Message); msg != "" {
			helpLines = strings.Count(state.Message, "\n") + 1
		}
		// Help block + separator + command block + separator + 2 checkboxes + optional pool
		// section + separator + blank + buttons row.
		height = helpLines + 1 + runForEachCommandFieldRows(state) + 3 + 5
		if runForEachHasPoolSelector(state) {
			// Separator + label + pool radios ("No pool" + one per pool).
			height += 1 + 1 + (1 + len(state.RunForEachPools))
		}
	case FileDialogMassRename:
		switch state.MassRenamePhase {
		case MassRenamePhaseSavePrompt:
			height = massRenameSavePromptDialogHeight()
		case MassRenamePhaseLoadPicker, MassRenamePhaseHistoryPicker, MassRenamePhaseOverwritePicker:
			height = massRenamePatternPickerDialogHeight(layout.Height)
		default:
			height = massRenameDialogHeight(layout.Height, state)
		}
	default:
		if renameToolActive(state) {
			// Preview label + preview row + separator + options + separator + buttons.
			height = renameToolDialogHeight()
		} else if len(state.Fields) > 0 {
			height = len(state.Fields)*3 + 4 // +1 separator row above buttons
			if state.DialogType == FileDialogExtract {
				if msg := strings.TrimSpace(state.Message); msg != "" {
					height += strings.Count(state.Message, "\n") + 1
				}
			}
		} else {
			height = 5
		}
		if mkdirHasActions(state) {
			// Separator + 3 radio rows + blank row above the shared button strip.
			height += 1 + mkdirActionRowCount + 1
		}
		if renameHasFocusCheckbox(state) {
			height += 1 + renameFocusCheckboxRowCount + renameOpenOtherRows(state) + 1
		}
	}
	if height > layout.Height-2 {
		height = layout.Height - 2
	}
	if height < 5 {
		height = 5
	}

	return draw.CenteredDialogRect(layout, width, height), true
}

func DrawFileDialog(screen tcell.Screen, layout Layout, state FileDialogState, ctx DialogRenderContext, paintDeleteIcon DeleteRowIconPainter) {
	styles := ctx.Styles
	showIcons := ctx.ShowIcons
	deleteIconLead := ctx.IconLead
	rect, ok := FileDialogRect(layout, state, deleteIconLead)
	if !ok {
		return
	}

	dialogTitle := fileDialogOuterTitle(state)
	if dialogTitle == "" {
		return
	}

	borderStyle := draw.DrawDialogFrame(screen, rect, dialogTitle, styles)

	// drawDropdown paints an open path-completion dropdown last, over the buttons.
	var drawDropdown func(uiscrollbar.Style)

	switch state.DialogType {
	case FileDialogDelete:
		drawFileDeleteDialogContent(screen, rect, state, borderStyle, styles, showIcons, deleteIconLead, paintDeleteIcon)
	case FileDialogAddBookmark:
		drawAddBookmarkDialogContent(screen, rect, state, borderStyle, styles)
	case FileDialogRunForEach:
		if state.RunForEachHistoryOpen {
			drawRunForEachHistoryPickerContent(screen, rect, state.RunForEachHistoryPicker, borderStyle, styles)
		} else if len(state.Fields) > 0 {
			drawRunForEachDialogFields(screen, rect, borderStyle, state, styles)
		}
	case FileDialogMassRename:
		switch state.MassRenamePhase {
		case MassRenamePhaseSavePrompt:
			drawMassRenameSavePromptContent(screen, rect, state, borderStyle, styles)
		case MassRenamePhaseLoadPicker, MassRenamePhaseOverwritePicker:
			drawMassRenamePatternPickerContent(screen, rect, state.MassRenameLoadPicker, borderStyle, styles)
		case MassRenamePhaseHistoryPicker:
			drawMassRenamePatternPickerContent(screen, rect, state.MassRenameHistoryPicker, borderStyle, styles)
		default:
			drawMassRenameDialog(screen, rect, state, borderStyle, styles, ctx.ScrollbarStyle)
		}
	default:
		if renameToolActive(state) {
			drawRenameToolContent(screen, rect, state, borderStyle, styles)
		} else if len(state.Fields) > 0 {
			drawDropdown = drawMultiFieldDialog(screen, rect, state, styles)
		}
		if mkdirHasActions(state) {
			drawMkdirActionRows(screen, rect, state, borderStyle, styles)
		}
		if renameHasFocusCheckbox(state) {
			drawRenameFocusCheckbox(screen, rect, state, borderStyle, styles)
		}
	}

	// Draw buttons at the bottom. Separator ends on buttonY-2; buttonY-1 is the
	// mandatory surface-only blank row (a separator is not that row).
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	if state.DialogType == FileDialogDelete {
		drawDeleteButtons(screen, rect, buttonY, state, styles)
	} else {
		drawOkCancelButtons(screen, rect, buttonY, state, styles)
	}

	if drawDropdown != nil {
		drawDropdown(ctx.ScrollbarStyle)
	}
}

func renameToolActive(state FileDialogState) bool {
	return FileDialogHasRenamePhase(state.DialogType) && state.RenamePhase != RenamePhaseMain
}

func renameToolDialogHeight() int { return 10 }

func fileDialogOuterTitle(state FileDialogState) string {
	if FileDialogHasRenamePhase(state.DialogType) {
		switch state.RenamePhase {
		case RenamePhaseSanitize:
			return "Sanitize"
		case RenamePhaseSlugify:
			return "Slugify"
		case RenamePhaseEncoding:
			return "Encoding"
		default:
			if state.DialogType == FileDialogDuplicate {
				return "Duplicate"
			}
			return "Rename"
		}
	}
	if state.DialogType == FileDialogMassRename {
		switch state.MassRenamePhase {
		case MassRenamePhaseSavePrompt:
			return "Save pattern"
		case MassRenamePhaseLoadPicker:
			return "Load pattern"
		case MassRenamePhaseOverwritePicker:
			return "Overwrite pattern"
		case MassRenamePhaseHistoryPicker:
			return "Pattern history"
		}
	}
	if state.DialogType == FileDialogRunForEach && state.RunForEachHistoryOpen {
		return "History"
	}
	if state.DialogType == FileDialogDelete && state.DeleteDanglingDirs {
		if len(state.DeleteEntries) == 1 {
			return "Delete empty leftover dir?"
		}
		return "Delete empty leftover dirs?"
	}
	return fileDialogTitle(state.DialogType)
}

func fileDialogTitle(dialogType FileDialogType) string {
	switch dialogType {
	case FileDialogRename:
		return "Rename"
	case FileDialogDuplicate:
		return "Duplicate"
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
	minWidth := fileDialogBaseMinWidth
	// Field row width follows labels only; values scroll in drawInputField.
	for _, field := range state.Fields {
		fw := utf8.RuneCountInString(field.Label) + 6
		if fw > minWidth {
			minWidth = fw
		}
	}
	if len(state.Fields) > 0 {
		minWidth = max(minWidth, PreferredFormDialogWidth)
	}
	// Rename/duplicate: if the name (plus cursor cell) doesn't fit the fixed
	// width, switch to a wide mode covering 80% of the terminal.
	if FileDialogHasRenamePhase(state.DialogType) {
		for _, field := range state.Fields {
			if utf8.RuneCountInString(field.Value)+1+4 > minWidth {
				minWidth = max(minWidth, WideDialogWidth(screenWidth))
				break
			}
		}
	}
	// For delete dialog, use cached layout width from open (see ComputeDeleteDialogLayoutMinWidth).
	if state.DialogType == FileDialogDelete {
		if state.DeleteLayoutMinWidth > minWidth {
			minWidth = state.DeleteLayoutMinWidth
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
		// widest label plus the marker icons and outer dialog padding (1+marker+label+1+border).
		for _, r := range MkdirActionRadioSpecs() {
			lw := utf8.RuneCountInString(r.Label) + 8
			if lw > minWidth {
				minWidth = lw
			}
		}
	}
	if renameHasFocusCheckbox(state) {
		lw := utf8.RuneCountInString(draw.CheckboxText(renameFocusCheckboxLabel(state), true)) + 4
		if renameHasOpenOtherCheckbox(state) {
			lw = max(lw, utf8.RuneCountInString(draw.CheckboxText(renameOpenOtherLabel, true))+4)
		}
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
			"External $EDITOR",
			"Show only modified",
			"Trim whitespace",
			"Case insensitive",
			"Pattern",
			"Replacement",
		} {
			lw := utf8.RuneCountInString(label) + 8
			if lw > minWidth {
				minWidth = lw
			}
		}
		// Options row: three checkboxes on one line (Show only modified | Trim whitespace | Case insensitive).
		optsRow := utf8.RuneCountInString(draw.CheckboxText("Show only modified", false)) +
			utf8.RuneCountInString(draw.CheckboxText("Trim whitespace", false)) +
			utf8.RuneCountInString(draw.CheckboxText("Case insensitive", false)) + 10 // gaps + margins
		if optsRow > minWidth {
			minWidth = optsRow
		}
		for i := 0; i < len(state.MassRenamePreviewBefore); i++ {
			lb := state.MassRenamePreviewBefore[i]
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
		if errMsg := strings.TrimSpace(state.MassRenameComputeError); errMsg != "" {
			if lw := utf8.RuneCountInString(errMsg) + 4; lw > minWidth {
				minWidth = lw
			}
		}
		if h := massRenamePatternHintText(state); h != "" {
			// Pattern hint shares the label row (right-aligned), so the dialog must be wide
			// enough for label + gap + hint together, not just the hint alone.
			hw := utf8.RuneCountInString(massRenamePatternLabelText(state)) + 1 + utf8.RuneCountInString(h) + 4
			if hw > minWidth {
				minWidth = hw
			}
		}
		// Reserve the regex-mode replacement hint width in every mode so switching to Regex
		// doesn't widen the dialog. Rendered as "<info icon> <hint>"; reserve the ASCII
		// fallback, the widest icon form.
		hw := utf8.RuneCountInString(theme.IconDialogInfoASCII) + 1 + utf8.RuneCountInString(ops.MassRenameReplacementSyntaxHint) + 4
		if hw > minWidth {
			minWidth = hw
		}
	}
	if minWidth > screenWidth-4 {
		minWidth = screenWidth - 4
	}
	return max(fileDialogFloorWidth, minWidth)
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
	y := fieldStartY
	innerBottom := rect.Y + rect.Height - 2
	for i, field := range state.Fields {
		if y >= innerBottom {
			break
		}
		labelWidth := rect.Width - 4
		if labelWidth <= 0 {
			break
		}
		fieldStyle := styles.DialogText.Background(dbg)
		primitive.Text(screen, draw.DialogTextX(rect), y, labelWidth, field.Label+":", fieldStyle)
		y++
		if y >= innerBottom {
			break
		}
		drawInputField(screen, draw.DialogTextX(rect), y, rect.Width-4, field, i == state.FocusedField, styles)
		y++
		if i == 0 {
			if hint := runForEachCommandErrorText(state); hint != "" && y < innerBottom {
				primitive.Text(screen, draw.DialogTextX(rect), y, rect.Width-4, hint, dialogErrorTextStyle(styles, dbg))
				y++
			} else if preview := runForEachPreviewText(state); preview != "" && y < innerBottom {
				primitive.Text(screen, draw.DialogTextX(rect), y, rect.Width-4, "→ "+preview, runForEachPreviewStyle(styles, dbg))
				y++
			}
		}
	}

	if y >= rect.Y+rect.Height-3 {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y < innerBottom {
		draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y, "Run in each selected directory", 'R', state.RunForEachInDirs, state.FocusedField == len(state.Fields), false, styles)
	}
	y++
	if y < innerBottom {
		draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y, "Allocate pseudo-TTY (interactive)", 'T', state.RunForEachPTY, state.FocusedField == len(state.Fields)+1, false, styles)
	}
	y++

	if !runForEachHasPoolSelector(state) {
		return
	}

	sectionY := y
	if sectionY >= rect.Y+rect.Height-3 {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, sectionY, borderStyle)

	y = sectionY + 1
	if y >= innerBottom {
		return
	}
	labelWidth := rect.Width - 4
	if labelWidth <= 0 {
		return
	}
	labelStyle := styles.DialogText.Background(dbg)
	primitive.Text(screen, rect.X+2, y, labelWidth, "Worker pool (optional):", labelStyle)
	y++ // pool radios sit directly beneath the label

	baseFocus := len(state.Fields) + 2
	if y < innerBottom {
		focused := state.FocusedField == baseFocus
		selected := strings.TrimSpace(state.RunForEachPool) == ""
		draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, "No pool (unlimited)", 0, selected, focused, styles)
		y++
	}
	for i, name := range state.RunForEachPools {
		if y >= innerBottom {
			return
		}
		focused := state.FocusedField == baseFocus+1+i
		selected := strings.TrimSpace(state.RunForEachPool) == strings.TrimSpace(name)
		draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, name, 0, selected, focused, styles)
		y++
	}
}

// drawMultiFieldDialog returns a non-nil func that paints the open path-completion dropdown;
// the caller runs it after everything else so nothing overdraws it.
func drawMultiFieldDialog(screen tcell.Screen, rect Rect, state FileDialogState, styles theme.Theme) (drawDropdown func(uiscrollbar.Style)) {
	_, dbg, _ := styles.DialogSurface.Decompose()
	fieldStartY := rect.Y + 1
	for i, field := range state.Fields {
		y := fieldStartY + i*3
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
		fieldStyle = fieldStyle.Background(dbg)
		primitive.Text(screen, rect.X+2, y, labelWidth, field.Label+":", fieldStyle)

		// Input row sits directly beneath the label.
		inputY := y + 1
		if inputY >= rect.Y+rect.Height-3 {
			continue
		}
		drawInputField(screen, rect.X+2, inputY, rect.Width-4, field, i == state.FocusedField, styles)
		if warn := renameWhitespaceWarning(state, i); warn != "" && inputY+1 < rect.Y+rect.Height-3 {
			primitive.Text(screen, draw.DialogTextX(rect), inputY+1, draw.DialogContentWidth(rect), warn, dialogErrorTextStyle(styles, dbg))
		}
		if field.PathPicker && field.Completion.Open {
			x, y, scroll, comp := rect.X+2, inputY+1, field.Scroll, field.Completion
			drawDropdown = func(sb uiscrollbar.Style) {
				drawPathCompletionDropdown(screen, x, y, scroll, comp, sb, styles)
			}
		}
	}
	if state.DialogType == FileDialogExtract {
		if msg := strings.TrimSpace(state.Message); msg != "" && len(state.Fields) > 0 {
			y := fieldStartY + (len(state.Fields)-1)*3 + 2
			warn := styles.MessageWarn.Background(dbg)
			innerW := draw.DialogContentWidth(rect)
			textX := draw.DialogTextX(rect)
			for _, line := range strings.Split(state.Message, "\n") {
				if y >= rect.Y+rect.Height-3 {
					break
				}
				if innerW > 0 {
					primitive.Text(screen, textX, y, innerW, line, warn)
				}
				y++
			}
		}
	}
	return drawDropdown
}

// drawInputField paints a dialog text input row, including the filesystem-completion ghost
// suffix when the field has one. drawInputFieldInvalid takes the error style explicitly (path
// validation); drawInputField uses field.InputInvalid.
// The text area scrolls horizontally to keep the caret visible; overflow markers (◀/▶) appear on
// the edge text cells when content is hidden in that direction.
func drawInputField(screen tcell.Screen, x, y, width int, field FileDialogField, focused bool, styles theme.Theme) {
	drawInputFieldInvalid(screen, x, y, width, field, focused, field.InputInvalid, styles)
}

func drawInputFieldInvalid(screen tcell.Screen, x, y, width int, field FileDialogField, focused, invalid bool, styles theme.Theme) {
	if width <= 0 {
		return
	}
	prefillPending := field.Prefill != "" && field.PrefillPending && field.Value == field.Prefill
	ghostSuffix := "" // empty for fields without path completion
	if !prefillPending {
		ghostSuffix = field.Completion.GhostSuffix(field.Value)
	}
	draw.PaintScrollingInputContent(
		screen, x, y, width,
		field.Value, ghostSuffix,
		field.Cursor, field.Scroll,
		focused, invalid, focused, prefillPending,
		"",
		styles,
	)
}

// DrawInputField paints a single dialog text input row (shared by file dialogs and inline query bars).
func DrawInputField(screen tcell.Screen, x, y, width int, field FileDialogField, focused bool, styles theme.Theme) {
	drawInputField(screen, x, y, width, field, focused, styles)
}

// fileDialogFocusIndex returns the focus index for the OK/Yes button.
func fileDialogOKFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogDelete {
		return 0
	}
	if state.DialogType == FileDialogMassRename && MassRenamePickerPhase(state.MassRenamePhase) {
		return 1 // list=0, OK=1
	}
	if state.DialogType == FileDialogRunForEach && state.RunForEachHistoryOpen {
		return 1 // list=0, OK=1
	}
	if state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain {
		return massRenameContentEnd(state)
	}
	if renameToolActive(state) {
		return renameToolOptionCount(state)
	}
	// SavePrompt phase (Name/Description fields) falls through to the generic formula below,
	// same as any other two-field dialog: len(state.Fields) == 2.
	return len(state.Fields) + mkdirExtraFocusRows(state) + renameExtraFocusRows(state) + runForEachExtraFocusRows(state)
}

// fileDialogCancelFocusIndex returns the focus index for the Cancel/No button.
func fileDialogCancelFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogDelete {
		return 1
	}
	if state.DialogType == FileDialogMassRename && MassRenamePickerPhase(state.MassRenamePhase) {
		return 2
	}
	if state.DialogType == FileDialogRunForEach && state.RunForEachHistoryOpen {
		return 2
	}
	if state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain {
		return massRenameContentEnd(state) + 2
	}
	if renameToolActive(state) {
		return renameToolOptionCount(state) + 1
	}
	return len(state.Fields) + mkdirExtraFocusRows(state) + renameExtraFocusRows(state) + runForEachExtraFocusRows(state) + 1
}

func renameToolOptionCount(state FileDialogState) int {
	switch state.RenamePhase {
	case RenamePhaseEncoding:
		return RenameEncodingOptionCount(state)
	case RenamePhaseSanitize, RenamePhaseSlugify:
		return 2
	default:
		return 2
	}
}

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
	case RenamePhaseEncoding:
		return RenameEncodingPreviewText(state)
	default:
		return v
	}
}

func drawRenameToolContent(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	primaryCol := draw.DialogTextX(rect)
	optionCol := draw.DialogOptionX(rect)
	innerWidth := draw.DialogContentWidth(rect)
	innerBottom := rect.Y + rect.Height - 2
	y := rect.Y + 1
	if y >= innerBottom || innerWidth <= 0 {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	labelStyle := styles.DialogText.Background(dbg)
	primitive.Text(screen, primaryCol, y, innerWidth, "Preview:", labelStyle)
	y++ // preview value sits directly beneath the label
	if y >= innerBottom {
		return
	}
	preview := renameToolPreviewText(state)
	if utf8.RuneCountInString(preview) > innerWidth {
		preview = primitive.TruncateRight(preview, innerWidth)
	}
	primitive.Text(screen, primaryCol, y, innerWidth, preview, labelStyle)
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
		draw.DrawDialogCheckbox(screen, optionCol, y, `Replace "." with space`, '.', state.RenameSanitizeDots, state.FocusedField == 0, false, styles)
		y++
		if y < innerBottom {
			draw.DrawDialogCheckbox(screen, optionCol, y, `Replace "_" with space`, '_', state.RenameSanitizeUnderscores, state.FocusedField == 1, false, styles)
		}
	} else if state.RenamePhase == RenamePhaseSlugify {
		dotSel := state.RenameSlugifySep == RenameSlugifyDot
		usSel := state.RenameSlugifySep == RenameSlugifyUnderscore
		draw.DrawDialogRadio(screen, optionCol, y, `Replace space with "."`, '.', dotSel, state.FocusedField == 0, styles)
		y++
		if y < innerBottom {
			draw.DrawDialogRadio(screen, optionCol, y, `Replace space with "_"`, '_', usSel, state.FocusedField == 1, styles)
		}
	} else if state.RenamePhase == RenamePhaseEncoding {
		for i := 0; i < len(state.RenameEncodingCandidates); i++ {
			if y >= innerBottom {
				break
			}
			label := RenameEncodingOptionLabel(state, i)
			shortcut := RenameEncodingOptionShortcut(state, i)
			sel := state.RenameEncodingSelected == i
			draw.DrawDialogRadio(screen, optionCol, y, label, shortcut, sel, state.FocusedField == i, styles)
			y++
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

// renameWhitespaceWarning returns the warning shown on the blank row under the rename/duplicate
// name input (field i) when the name has leading or trailing whitespace and focus has left it.
func renameWhitespaceWarning(state FileDialogState, i int) string {
	if !renameHasFocusCheckbox(state) || i != 0 || state.FocusedField == 0 {
		return ""
	}
	v := state.Fields[0].Value
	lead := strings.TrimLeftFunc(v, unicode.IsSpace) != v
	trail := strings.TrimRightFunc(v, unicode.IsSpace) != v
	switch {
	case lead && trail:
		return "Name has leading and trailing whitespace"
	case lead:
		return "Name has leading whitespace"
	case trail:
		return "Name has trailing whitespace"
	}
	return ""
}

// renameHasFocusCheckbox reports whether the single-file rename main dialog
// should render and accept the focus-after-rename checkbox.
func renameHasFocusCheckbox(state FileDialogState) bool {
	return FileDialogHasRenamePhase(state.DialogType) && state.RenamePhase == RenamePhaseMain
}

// renameExtraFocusRows returns the number of focus rows contributed by the
// rename focus checkbox, or 0 when not applicable.
func renameExtraFocusRows(state FileDialogState) int {
	if renameHasFocusCheckbox(state) {
		return renameFocusCheckboxRowCount + renameOpenOtherRows(state)
	}
	return 0
}

const renameOpenOtherLabel = "Open in the other panel after rename"

// renameHasOpenOtherCheckbox reports whether the rename dialog offers the open-in-other-panel
// checkbox (single directory rename only).
func renameHasOpenOtherCheckbox(state FileDialogState) bool {
	return state.DialogType == FileDialogRename && state.RenamePhase == RenamePhaseMain && state.RenameSourceIsDir
}

func renameOpenOtherRows(state FileDialogState) int {
	if renameHasOpenOtherCheckbox(state) {
		return 1
	}
	return 0
}

func runForEachHasPoolSelector(state FileDialogState) bool {
	return state.DialogType == FileDialogRunForEach && len(state.RunForEachPools) > 0
}

func runForEachExtraFocusRows(state FileDialogState) int {
	if state.DialogType != FileDialogRunForEach {
		return 0
	}
	// "Run in each selected directory" + "Allocate pseudo-TTY" checkbox rows.
	rows := 2
	if runForEachHasPoolSelector(state) {
		// "No pool" + one row per configured pool.
		rows += 1 + len(state.RunForEachPools)
	}
	return rows
}

// renameFocusCheckboxLabel returns the focus-after checkbox caption for rename-like dialogs.
func renameFocusCheckboxLabel(state FileDialogState) string {
	if state.DialogType == FileDialogDuplicate {
		return "Focus after duplicate"
	}
	return "Focus after rename"
}

// drawRenameFocusCheckbox draws the focus-after checkbox under the name input.
func drawRenameFocusCheckbox(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	if !renameHasFocusCheckbox(state) || len(state.Fields) == 0 {
		return
	}
	fieldsBottom := rect.Y + 1 + len(state.Fields)*3
	sepY := fieldsBottom
	if sepY >= rect.Y+rect.Height-2 {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)
	y := sepY + 1
	if y >= rect.Y+rect.Height-2 {
		return
	}
	draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y, renameFocusCheckboxLabel(state), 'A', state.RenameFocusAfter, state.FocusedField == len(state.Fields), false, styles)
	if renameHasOpenOtherCheckbox(state) && y+1 < rect.Y+rect.Height-2 {
		draw.DrawDialogCheckbox(screen, draw.DialogOptionX(rect), y+1, renameOpenOtherLabel, 'p', state.RenameOpenInOther, state.FocusedField == len(state.Fields)+1, false, styles)
	}
}

// drawMkdirActionRows draws the radio button section under the directory-name input
// for the mkdir-with-selections dialog. Focus indices for the radio rows start
// immediately after len(state.Fields).
func drawMkdirActionRows(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	if !mkdirHasActions(state) || len(state.Fields) == 0 {
		return
	}
	// drawMultiFieldDialog lays out each field as: label row, input row, blank row.
	// The first row after the last field block sits at rect.Y + 1 + len(Fields)*3.
	fieldsBottom := rect.Y + 1 + len(state.Fields)*3
	sepY := fieldsBottom
	if sepY >= rect.Y+rect.Height-2 {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)
	optionCol := draw.DialogOptionX(rect)
	radios := MkdirActionRadioSpecs()
	baseFocus := len(state.Fields)
	for i, r := range radios {
		y := sepY + 1 + i
		if y >= rect.Y+rect.Height-2 {
			break
		}
		draw.DrawDialogRadio(screen, optionCol, y, r.Label, r.Shortcut, state.MkdirAction == r.Action, state.FocusedField == baseFocus+i, styles)
	}
}

func drawOkCancelButtons(screen tcell.Screen, rect Rect, y int, state FileDialogState, styles theme.Theme) {
	okFocusIdx := fileDialogOKFocusIndex(state)
	cancelFocusIdx := fileDialogCancelFocusIndex(state)
	if state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain {
		disabled := !FileDialogMassRenameOKEnabled(state)
		specs := []draw.DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: state.FocusedField == okFocusIdx, Disabled: disabled},
			{Label: "Apply", Shortcut: 'A', Focused: state.FocusedField == MassRenameApplyFocusIndex(state), Disabled: disabled},
			{Label: "Cancel", Shortcut: 'C', Focused: state.FocusedField == cancelFocusIdx},
		}
		draw.DrawDialogButtonRowCentered(screen, rect, y, specs, styles)
		return
	}
	draw.DrawDialogButtonRowCentered(screen, rect, y, draw.OKCancelButtonSpecs(state.FocusedField == okFocusIdx, state.FocusedField == cancelFocusIdx), styles)
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
	if rect.Width < 4 || rect.Height < 9 {
		return
	}
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	primaryCol := rect.X + 2
	innerWidth := rect.Width - 4

	// Content rows sit directly beneath their label row.
	primitive.Text(screen, primaryCol, rect.Y+1, innerWidth, "Path:", textStyle)
	pathValue := state.Message
	if utf8.RuneCountInString(pathValue) > innerWidth {
		pathValue = primitive.TruncateRight(pathValue, innerWidth)
	}
	primitive.Text(screen, primaryCol, rect.Y+2, innerWidth, pathValue, textStyle)

	draw.DrawDialogHSeparator(screen, rect, rect.Y+3, borderStyle)

	primitive.Text(screen, primaryCol, rect.Y+4, innerWidth, "Name:", textStyle)

	if len(state.Fields) > 0 {
		focused := state.FocusedField == 0
		drawInputField(screen, primaryCol, rect.Y+5, innerWidth, state.Fields[0], focused, styles)
	}
}
