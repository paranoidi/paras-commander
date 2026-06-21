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
		return 3 // 3 radios, no fields or checkboxes
	}
	n := 3 + 2 // mode radios + find + replace
	if state.MassRenameMode == MassRenameModeUISimple {
		n++ // case-insensitive checkbox
	}
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
	// 3 radios + sep + two fields (label+input each) + checkbox-area row + sep before preview.
	// The checkbox-area row is always reserved; it renders blank in Regex / ExternalEditor mode.
	fixed := 3 + 1 + 4 + 1 + 1
	if massRenameShowsPatternHint(state) {
		fixed++
	}
	if massRenameShowsReplacementHint(state) {
		fixed++
	}
	height := 1 + fixed + vp + 2 // top pad + body + buttons row + bottom border
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
	overhead := 13
	if mode == MassRenameModeUIExternalEditor {
		// ExternalEditor omits fields (4) + checkbox-area (1) + sep (1) = 6 rows → overhead shrinks by 5.
		// (The sep-before-preview is shared but one sep is also absent from the ExternalEditor content.)
		overhead = 8
	}
	maxBody := layoutHeight - overhead
	if maxBody < 3 {
		maxBody = 3
	}
	return maxBody
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
	leftCol := rect.X + 2
	innerW := rect.Width - 4
	if innerW <= 0 {
		return
	}
	y := rect.Y + 1
	innerBottom := rect.Y + rect.Height - 2
	warnStyle := styles.MessageWarn.Background(dbg)

	draw.DrawDialogRadio(screen, leftCol, y, "Simple (replace text)", 'S', state.MassRenameMode == MassRenameModeUISimple, state.FocusedField == 0, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, leftCol, y, "Regular expression", 'R', state.MassRenameMode == MassRenameModeUIRegex, state.FocusedField == 1, styles)
	y++
	if y >= innerBottom {
		return
	}
	draw.DrawDialogRadio(screen, leftCol, y, "External $EDITOR", 'E', state.MassRenameMode == MassRenameModeUIExternalEditor, state.FocusedField == 2, styles)
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
			primitive.Text(screen, leftCol, y, innerW, field.Label+":", labelStyle)
			y++
			if y >= innerBottom {
				return
			}
			drawInputField(screen, leftCol, y, innerW, field, state.FocusedField == focusIdx, styles)
			y++
			if fi == 0 && state.MassRenameMode == MassRenameModeUIRegex {
				if hint := massRenamePatternHintText(state); hint != "" && y < innerBottom {
					primitive.Text(screen, leftCol, y, innerW, hint, massRenamePatternHintStyle(styles, dbg))
					y++
				}
			}
			if fi == 1 && state.MassRenameMode == MassRenameModeUIRegex {
				if hint := massRenameReplacementHintText(state); hint != "" && y < innerBottom {
					primitive.Text(screen, leftCol, y, innerW, hint, massRenameReplacementHintStyle(styles, dbg))
					y++
				}
			}
		}

		// Checkbox row is always consumed to keep dialog height identical across modes.
		// Regex mode leaves it blank; the dialog surface background fills the row.
		if y >= innerBottom {
			return
		}
		if state.MassRenameMode == MassRenameModeUISimple {
			// One cell left of radios/fields so "[ ]" aligns with "( )" marker column.
			draw.DrawDialogCheckbox(screen, leftCol-1, y, "Case insensitive find", 'i', state.MassRenameCaseFold, state.FocusedField == 5, styles)
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
	leftW := innerW / 2
	rightW := innerW - leftW
	if innerW >= 3 {
		sepW = 1
		avail := innerW - sepW
		leftW = avail / 2
		rightW = avail - leftW
	}
	if leftW < 1 {
		leftW = 1
		rightW = innerW - sepW - leftW
		if rightW < 0 {
			rightW = 0
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
			primitive.Text(screen, leftCol, y, innerW, lb, warnStyle)
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
		lbText, lbSpans := massRenameBeforePreviewRow(lb, removed, replaced, leftW, beforeBase, beforeRemoved, beforeReplaced)
		rowError := i < len(afterErrorRows) && afterErrorRows[i]
		rbText, rbSpans := massRenameAfterPreviewRow(rb, added, rightW, afterBase, afterAdded, afterError.Background(dbg), rowError)
		primitive.StyledText(screen, leftCol, y, leftW, lbText, beforeBase, lbSpans)
		if sepW == 1 {
			screen.SetContent(leftCol+leftW, y, ' ', nil, beforeBase)
		}
		primitive.StyledText(screen, leftCol+leftW+sepW, y, rightW, rbText, afterBase, rbSpans)
		y++
	}
}
