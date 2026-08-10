package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// massRenameContentEnd returns the FocusedField index of the OK button for a mass rename dialog.
func massRenameContentEnd(state FileDialogState) int {
	switch state.MassRenameMode {
	case MassRenameModeUIExternalEditor:
		return 6 // 4 radios + show-modified + strip
	case MassRenameModeUICapitalize:
		return 8 // 4 radios + show-modified + strip + capitalize-each-word + treat-punctuation
	default:
		return 9 // 4 radios + find + replace + show-modified + strip + case
	}
}

// MassRenameApplyFocusIndex returns the FocusedField index of the mass-rename
// dialog's Apply button.
func MassRenameApplyFocusIndex(state FileDialogState) int {
	return massRenameContentEnd(state) + 1
}

// MassRenameEnsurePreviewScroll clamps scroll so the viewport fits.
func MassRenameEnsurePreviewScroll(state *FileDialogState, viewportRows, totalRows int) {
	if viewportRows <= 0 || totalRows <= 0 {
		state.MassRenamePreviewScroll = 0
		return
	}
	maxScroll := totalRows - viewportRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if state.MassRenamePreviewScroll > maxScroll {
		state.MassRenamePreviewScroll = maxScroll
	}
	if state.MassRenamePreviewScroll < 0 {
		state.MassRenamePreviewScroll = 0
	}
}

// massRenameDialogHeight returns the dialog outer height for mass rename.
// The dialog is sized identically for all three modes (Simple / Regex / ExternalEditor) so
// switching modes does not resize it. ExternalEditor skips the fields section at render time,
// which gives the preview area the freed rows automatically.
func massRenameDialogHeight(layoutHeight int, state FileDialogState) int {
	maxVP := MassRenamePreviewViewportRows(layoutHeight, MassRenameModeUISimple)
	previewCount := len(state.MassRenamePreviewBefore)
	if previewCount < 1 {
		previewCount = 1
	}
	vp := maxVP
	if previewCount < vp {
		vp = previewCount
	}
	// 4 radios + options checkbox row + sep + two fields (label+input each) + sep before preview.
	fixed := 4 + 1 + 1 + 4 + 1
	if massRenameShowsPatternHint(state) {
		fixed++
	}
	if massRenameShowsReplacementHint(state) {
		fixed++
	}
	height := 1 + fixed + vp + 3 // top pad + body + sep-above-buttons + buttons row + bottom border
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	if height < 12 {
		height = 12
	}
	return height
}

// MassRenamePreviewViewportRows returns the preview page size for PgUp/PgDn scrolling.
// ExternalEditor and Capitalize skip (or shrink) the fields section, so their effective
// viewport is larger.
func MassRenamePreviewViewportRows(layoutHeight int, mode MassRenameModeUI) int {
	// Base overhead matches the Simple/Regex fixed layout (see massRenameDialogHeight).
	// +1 accounts for the separator row above buttons drawn outside drawMassRenameDialog.
	overhead := 15
	switch mode {
	case MassRenameModeUIExternalEditor:
		// ExternalEditor omits fields (4) + sep-before-preview (1) = 5 rows.
		overhead = 10
	case MassRenameModeUICapitalize:
		// Capitalize's two single-item checkbox rows replace four field rows (net -2).
		overhead = 13
	}
	maxBody := layoutHeight - overhead
	if maxBody < 3 {
		maxBody = 3
	}
	return maxBody
}

// MassRenameFindFieldFocus is FocusedField for the Find / Pattern input (0-3 are mode radios).
const MassRenameFindFieldFocus = 4

// MassRenameModeRadioFocus returns the FocusedField index of the radio for mode.
func MassRenameModeRadioFocus(mode MassRenameModeUI) int {
	switch mode {
	case MassRenameModeUIRegex:
		return 1
	case MassRenameModeUIExternalEditor:
		return 2
	case MassRenameModeUICapitalize:
		return 3
	default:
		return 0
	}
}

// MassRenameShowModifiedFocusIdx returns the FocusedField index of the "Show only modified" checkbox.
func MassRenameShowModifiedFocusIdx(state FileDialogState) int {
	switch state.MassRenameMode {
	case MassRenameModeUIExternalEditor, MassRenameModeUICapitalize:
		return 4
	default:
		return 6
	}
}

// MassRenameStripFocusIdx returns the FocusedField index of the "Trim whitespace" checkbox.
func MassRenameStripFocusIdx(state FileDialogState) int {
	switch state.MassRenameMode {
	case MassRenameModeUIExternalEditor, MassRenameModeUICapitalize:
		return 5
	default:
		return 7
	}
}

// MassRenameCaseFocusIdx returns the FocusedField index of the "Case insensitive" checkbox,
// or -1 when the checkbox is not shown (External $EDITOR / Capitalize modes).
func MassRenameCaseFocusIdx(state FileDialogState) int {
	switch state.MassRenameMode {
	case MassRenameModeUIExternalEditor, MassRenameModeUICapitalize:
		return -1
	default:
		return 8
	}
}

// MassRenameCapEachWordFocusIdx returns the FocusedField index of the "Capitalize each word"
// checkbox, or -1 when not shown (only visible in Capitalize mode).
func MassRenameCapEachWordFocusIdx(state FileDialogState) int {
	if state.MassRenameMode == MassRenameModeUICapitalize {
		return 6
	}
	return -1
}

// MassRenameCapPunctFocusIdx returns the FocusedField index of the "Treat punctuation as
// separators" checkbox, or -1 when not shown (only visible in Capitalize mode).
func MassRenameCapPunctFocusIdx(state FileDialogState) int {
	if state.MassRenameMode == MassRenameModeUICapitalize {
		return 7
	}
	return -1
}

func drawMassRenameDialog(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	_, dbg, _ := styles.DialogSurface.Decompose()
	labelStyle := styles.DialogText.Background(dbg)
	beforeBase := styles.DialogMassRenameBefore.Background(dbg)
	beforeRemoved := styles.DialogMassRenameBeforeRemoved
	beforeReplaced := styles.DialogMassRenameBeforeReplaced
	afterBase := styles.DialogMassRenameAfter.Background(dbg)
	afterAdded := styles.DialogMassRenameAfterAdded
	afterError := styles.DialogMassRenameAfterError
	primaryCol := rect.X + 2
	innerW := rect.Width - 4
	if innerW <= 0 {
		return
	}
	y := rect.Y + 1
	innerBottom := rect.Y + rect.Height - 2
	warnStyle := styles.MessageWarn.Background(dbg)

	optX := primaryCol - 1
	draw.DrawDialogRadio(screen, optX, y, "Simple (replace text)", 'S', state.MassRenameMode == MassRenameModeUISimple, state.FocusedField == 0, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optX, y, "Regular expression", 'R', state.MassRenameMode == MassRenameModeUIRegex, state.FocusedField == 1, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optX, y, "External $EDITOR", 'E', state.MassRenameMode == MassRenameModeUIExternalEditor, state.FocusedField == 2, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, optX, y, "Capitalize", 'a', state.MassRenameMode == MassRenameModeUICapitalize, state.FocusedField == 3, styles)
	y++
	if y >= innerBottom {
		return
	}

	// Options row: Show only modified | Trim whitespace | Case insensitive (Simple/Regex).
	showModifiedFocusIdx := MassRenameShowModifiedFocusIdx(state)
	stripFocusIdx := MassRenameStripFocusIdx(state)
	caseFocusIdx := MassRenameCaseFocusIdx(state)
	stripX := optX + utf8.RuneCountInString(draw.CheckboxText("Show only modified", false)) + 3
	caseX := stripX + utf8.RuneCountInString(draw.CheckboxText("Trim whitespace", false)) + 3
	draw.DrawDialogCheckbox(screen, optX, y, "Show only modified", 'm', state.MassRenameShowOnlyModified, state.FocusedField == showModifiedFocusIdx, styles)
	draw.DrawDialogCheckbox(screen, stripX, y, "Trim whitespace", 't', state.MassRenameStripSpaces, state.FocusedField == stripFocusIdx, styles)
	if caseFocusIdx >= 0 {
		draw.DrawDialogCheckbox(screen, caseX, y, "Case insensitive", 'i', state.MassRenameCaseFold, state.FocusedField == caseFocusIdx, styles)
	}
	y++
	if y >= innerBottom {
		return
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	switch state.MassRenameMode {
	case MassRenameModeUIExternalEditor:
		// No fields, no checkboxes: nothing to draw here.
	case MassRenameModeUICapitalize:
		if y >= innerBottom {
			return
		}
		draw.DrawDialogCheckbox(screen, optX, y, "Capitalize each word", 'w', state.MassRenameCapEachWord, state.FocusedField == MassRenameCapEachWordFocusIdx(state), styles)
		y++
		if y >= innerBottom {
			return
		}
		draw.DrawDialogCheckbox(screen, optX, y, "Treat punctuation as separators", 'p', state.MassRenameCapPunctSep, state.FocusedField == MassRenameCapPunctFocusIdx(state), styles)
		y++
		if y >= innerBottom {
			return
		}
		draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
		y++
		if y >= innerBottom {
			return
		}
	default: // Simple / Regex
		for fi := 0; fi < 2 && fi < len(state.Fields); fi++ {
			field := state.Fields[fi]
			focusIdx := MassRenameFindFieldFocus + fi
			if y >= innerBottom {
				return
			}
			primitive.Text(screen, primaryCol, y, innerW, field.Label+":", labelStyle)
			y++
			if y >= innerBottom {
				return
			}
			drawInputField(screen, primaryCol, y, innerW, field, state.FocusedField == focusIdx, styles)
			y++
			if fi == 0 && state.MassRenameMode == MassRenameModeUIRegex {
				if hint := massRenamePatternHintText(state); hint != "" && y < innerBottom {
					primitive.Text(screen, primaryCol, y, innerW, hint, massRenamePatternHintStyle(styles, dbg))
					y++
				}
			}
			if fi == 1 && state.MassRenameMode == MassRenameModeUIRegex {
				if hint := massRenameReplacementHintText(state); hint != "" && y < innerBottom {
					primitive.Text(screen, primaryCol, y, innerW, hint, massRenameReplacementHintStyle(styles, dbg))
					y++
				}
			}
		}

		if y >= innerBottom {
			return
		}
		draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
		y++
		if y >= innerBottom {
			return
		}
	}

	vp := innerBottom - y - 1 // row innerBottom-1 is the global separator above buttons
	if vp < 1 {
		vp = 1
	}
	before := state.MassRenamePreviewBefore
	after := state.MassRenamePreviewAfter
	beforeRemovedRanges := state.MassRenamePreviewBeforeRemoved
	beforeReplacedRanges := state.MassRenamePreviewBeforeReplaced
	afterAddedRanges := state.MassRenamePreviewAfterAdded
	afterErrorRows := state.MassRenamePreviewAfterError
	n := len(before)
	if len(after) < n {
		n = len(after)
	}
	scroll := state.MassRenamePreviewScroll
	maxScroll := n - vp
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	sepW := 0
	primaryW := innerW / 2
	secondaryW := innerW - primaryW
	if innerW >= 3 {
		sepW = 1
		avail := innerW - sepW
		primaryW = avail / 2
		secondaryW = avail - primaryW
	}
	if primaryW < 1 {
		primaryW = 1
		secondaryW = innerW - sepW - primaryW
		if secondaryW < 0 {
			secondaryW = 0
		}
	}
	for row := 0; row < vp && y < innerBottom; row++ {
		i := scroll + row
		if i >= n {
			break
		}
		lb := before[i]
		rb := ""
		if i < len(after) {
			rb = after[i]
		}
		if strings.HasPrefix(lb, "!") {
			primitive.Text(screen, primaryCol, y, innerW, lb, warnStyle)
			y++
			continue
		}
		var removed, replaced, added []search.Range
		if i < len(beforeRemovedRanges) {
			removed = beforeRemovedRanges[i]
		}
		if i < len(beforeReplacedRanges) {
			replaced = beforeReplacedRanges[i]
		}
		if i < len(afterAddedRanges) {
			added = afterAddedRanges[i]
		}
		lbText, lbSpans := massRenameBeforePreviewRow(lb, removed, replaced, primaryW, beforeBase, beforeRemoved, beforeReplaced)
		rowError := i < len(afterErrorRows) && afterErrorRows[i]
		rbText, rbSpans := massRenameAfterPreviewRow(rb, added, secondaryW, afterBase, afterAdded, afterError.Background(dbg), rowError)
		primitive.StyledText(screen, primaryCol, y, primaryW, lbText, beforeBase, lbSpans)
		if sepW == 1 {
			screen.SetContent(primaryCol+primaryW, y, ' ', nil, beforeBase)
		}
		primitive.StyledText(screen, primaryCol+primaryW+sepW, y, secondaryW, rbText, afterBase, rbSpans)
		y++
	}
}
