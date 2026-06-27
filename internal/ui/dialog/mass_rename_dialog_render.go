package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// massRenameContentEnd returns the FocusedField index of the OK button for a mass rename dialog.
func massRenameContentEnd(state FileDialogState) int {
	if state.MassRenameMode == MassRenameModeUIExternalEditor {
		return 4 // 3 radios + show-modified checkbox
	}
	n := 3 + 2 // mode radios + find + replace
	if state.MassRenameMode == MassRenameModeUISimple {
		n++ // case-insensitive checkbox
	}
	n++ // show-only-modified checkbox
	return n
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
	// 3 radios + show-modified checkbox + sep + two fields (label+input each) + case-fold checkbox + sep before preview.
	// The case-fold checkbox row renders blank in Regex mode and is absent in ExternalEditor mode.
	// ExternalEditor also skips the fields and sep-before-preview.
	fixed := 3 + 1 + 1 + 4 + 1 + 1
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
// ExternalEditor skips the fields section, so its effective viewport is larger.
func MassRenamePreviewViewportRows(layoutHeight int, mode MassRenameModeUI) int {
	// Base overhead matches the Simple/Regex fixed layout (see massRenameDialogHeight).
	// +1 accounts for the separator row above buttons drawn outside drawMassRenameDialog.
	overhead := 15
	if mode == MassRenameModeUIExternalEditor {
		// ExternalEditor omits fields (4) + case-fold checkbox (1) + sep-before-preview (1) = 6 rows.
		overhead = 9
	}
	maxBody := layoutHeight - overhead
	if maxBody < 3 {
		maxBody = 3
	}
	return maxBody
}

// MassRenameShowModifiedFocusIdx returns the FocusedField index of the "Show only modified" checkbox.
func MassRenameShowModifiedFocusIdx(state FileDialogState) int {
	switch state.MassRenameMode {
	case MassRenameModeUISimple:
		return 6
	case MassRenameModeUIRegex:
		return 5
	default: // MassRenameModeUIExternalEditor
		return 3
	}
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

	// Show-only-modified checkbox (all modes), grouped with mode selection.
	showModifiedFocusIdx := MassRenameShowModifiedFocusIdx(state)
	draw.DrawDialogCheckbox(screen, optX, y, "Show only modified", 'm', state.MassRenameShowOnlyModified, state.FocusedField == showModifiedFocusIdx, styles)
	y++
	if y >= innerBottom {
		return
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	if state.MassRenameMode != MassRenameModeUIExternalEditor {
		for fi := 0; fi < 2 && fi < len(state.Fields); fi++ {
			field := state.Fields[fi]
			focusIdx := 3 + fi
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

		// Case-fold checkbox (Simple only); Regex mode leaves the row blank.
		if y >= innerBottom {
			return
		}
		if state.MassRenameMode == MassRenameModeUISimple {
			draw.DrawDialogCheckbox(screen, optX, y, "Case insensitive find", 'i', state.MassRenameCaseFold, state.FocusedField == 5, styles)
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
