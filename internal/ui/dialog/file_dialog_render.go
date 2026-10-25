package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/primitive"
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
	if FileDialogUsesSpec(state.DialogType) {
		p, ok := dlg.Place(layout.Width, layout.Height, FileDialogView(state, FileDialogViewContext{Layout: layout, IconLead: deleteIconLead}))
		return p.Rect, ok
	}

	// Calculate height based on dialog type.
	var height int
	switch state.DialogType {
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
		if len(state.Fields) > 0 {
			height = len(state.Fields)*3 + 4 // +1 separator row above buttons
			if state.DialogType == FileDialogExtract {
				if msg := strings.TrimSpace(state.Message); msg != "" {
					height += strings.Count(state.Message, "\n") + 1
				}
			}
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

	return draw.CenteredDialogRect(layout, width, height), true
}

func DrawFileDialog(screen tcell.Screen, layout Layout, state FileDialogState, ctx DialogRenderContext, paintDeleteIcon DeleteRowIconPainter) {
	styles := ctx.Styles
	deleteIconLead := ctx.IconLead
	if FileDialogUsesSpec(state.DialogType) {
		if state.Open {
			drawFileDialogSpec(screen, layout, state, ctx, paintDeleteIcon)
		}
		return
	}
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
		if len(state.Fields) > 0 {
			drawDropdown = drawMultiFieldDialog(screen, rect, state, styles)
		}
	}

	// Draw buttons at the bottom. Separator ends on buttonY-2; buttonY-1 is the
	// mandatory surface-only blank row (a separator is not that row).
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-2, borderStyle)
	drawOkCancelButtons(screen, rect, buttonY, state, styles)

	if drawDropdown != nil {
		drawDropdown(ctx.ScrollbarStyle)
	}
}

func renameToolActive(state FileDialogState) bool {
	return FileDialogHasRenamePhase(state.DialogType) && state.RenamePhase != RenamePhaseMain
}

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
			"Pattern",
			"Replacement",
		} {
			lw := utf8.RuneCountInString(label) + 8
			if lw > minWidth {
				minWidth = lw
			}
		}
		// Radio row and options row share four columns; the row starts on the left margin
		// cell (marker's leading space), so +3 = left border + right margin + right border.
		markerW := state.MassRenameMarkerWidth
		if markerW == 0 {
			markerW = utf8.RuneCountInString(draw.RadioText("", false)) // ASCII markers, the widest
		}
		_, rowW := massRenameColumns(markerW)
		minWidth = max(minWidth, rowW+3)
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
		draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, "No pool (unlimited)", 0, selected, focused, false, styles)
		y++
	}
	for i, name := range state.RunForEachPools {
		if y >= innerBottom {
			return
		}
		focused := state.FocusedField == baseFocus+1+i
		selected := strings.TrimSpace(state.RunForEachPool) == strings.TrimSpace(name)
		draw.DrawDialogRadio(screen, draw.DialogOptionX(rect), y, name, 0, selected, focused, false, styles)
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
	if state.DialogType == FileDialogMassRename && MassRenamePickerPhase(state.MassRenamePhase) {
		return 1 // list=0, OK=1
	}
	if state.DialogType == FileDialogRunForEach && state.RunForEachHistoryOpen {
		return 1 // list=0, OK=1
	}
	if state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain {
		return massRenameContentEnd(state)
	}
	// SavePrompt phase (Name/Description fields) falls through to the generic formula below,
	// same as any other two-field dialog: len(state.Fields) == 2.
	return len(state.Fields) + runForEachExtraFocusRows(state)
}

// fileDialogCancelFocusIndex returns the focus index for the Cancel/No button.
func fileDialogCancelFocusIndex(state FileDialogState) int {
	if state.DialogType == FileDialogMassRename && MassRenamePickerPhase(state.MassRenamePhase) {
		return 2
	}
	if state.DialogType == FileDialogRunForEach && state.RunForEachHistoryOpen {
		return 2
	}
	if state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain {
		return massRenameContentEnd(state) + 2
	}
	return len(state.Fields) + runForEachExtraFocusRows(state) + 1
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

// mkdirHasActions reports whether the mkdir dialog should render and accept
// post-mkdir action radio rows.
func mkdirHasActions(state FileDialogState) bool {
	return state.DialogType == FileDialogMkdir && state.MkdirShowActions
}

// renameHasFocusCheckbox reports whether the single-file rename main dialog
// should render and accept the focus-after-rename checkbox.
func renameHasFocusCheckbox(state FileDialogState) bool {
	return FileDialogHasRenamePhase(state.DialogType) && state.RenamePhase == RenamePhaseMain
}

const renameOpenOtherLabel = "Open in the other panel after rename"

// renameHasOpenOtherCheckbox reports whether the rename dialog offers the open-in-other-panel
// checkbox (single directory rename only).
func renameHasOpenOtherCheckbox(state FileDialogState) bool {
	return state.DialogType == FileDialogRename && state.RenamePhase == RenamePhaseMain && state.RenameSourceIsDir
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
