package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DeleteRowIconPainter draws file-list devicons for one delete dialog row; nil skips icons.
type DeleteRowIconPainter func(screen tcell.Screen, x, y int, entry DeleteListEntry, styles theme.Theme)

// FileDialogRect returns the outer rect DrawFileDialog will use for the given
// state, and ok=false when the dialog is not drawable (closed or width < 20).
// It is the single source of truth for the dialog's on-screen geometry, so the
// app can compare it across a keystroke to decide whether a dialog-rect overlay
// is valid (unchanged geometry) or a full render is required (shrunk geometry).
func FileDialogRect(layout Layout, state FileDialogState, deleteIconLead int) (Rect, bool) {
	if !state.Open || fileDialogWidth(layout.Width, state, deleteIconLead) < 20 {
		return Rect{}, false
	}
	p, ok := dlg.Place(layout.Width, layout.Height, FileDialogView(state, FileDialogViewContext{Layout: layout, IconLead: deleteIconLead}))
	return p.Rect, ok
}

// DrawFileDialog paints the open file dialog: the spec FileDialogView describes, then the overlays
// on its rects (the delete list's devicons, the focused path field's completion dropdown).
func DrawFileDialog(screen tcell.Screen, layout Layout, state FileDialogState, ctx DialogRenderContext, paintDeleteIcon DeleteRowIconPainter) {
	if state.Open {
		drawFileDialogSpec(screen, layout, state, ctx, paintDeleteIcon)
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
	if state.DialogType == FileDialogIndexTags && state.IndexTagsRemove {
		return "Remove tags"
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
	case FileDialogIndexTags:
		return "Add tags"
	case FileDialogIndexInit:
		return "fileindex"
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

// drawMultiFieldDialog returns a non-nil func that paints the open path-completion dropdown;
// the caller runs it after everything else so nothing overdraws it.
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

// renameFocusCheckboxLabel returns the focus-after checkbox caption for rename-like dialogs.
func renameFocusCheckboxLabel(state FileDialogState) string {
	if state.DialogType == FileDialogDuplicate {
		return "Focus after duplicate"
	}
	return "Focus after rename"
}
