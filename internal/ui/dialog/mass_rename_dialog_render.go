package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// massRenameContentEnd returns the FocusedField index of the OK button for a mass rename dialog.
func massRenameContentEnd(state FileDialogState) int {
	n := 2 + 2 // mode radios + find + replace
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
func massRenameDialogHeight(layoutHeight int, state FileDialogState) int {
	maxVP := MassRenamePreviewViewportRows(layoutHeight)
	previewCount := len(state.MassRenamePreviewBefore)
	if previewCount < 1 {
		previewCount = 1
	}
	vp := maxVP
	if previewCount < vp {
		vp = previewCount
	}
	// Radios(2) + sep + two fields (label+input each) + [regex pattern compile hint] + [checkbox+sep in simple] + sep + vp + hint + buttons (global sep above buttons only).
	fixed := 2 + 1 + 4 + 1
	if state.MassRenameMode == MassRenameModeUIRegex && strings.TrimSpace(state.MassRenamePatternCompileHint) != "" {
		fixed++
	}
	if state.MassRenameMode == MassRenameModeUISimple {
		fixed += 1 + 1 // checkbox + sep
	}
	fixed += 1                   // hint line after preview
	height := 1 + fixed + vp + 2 // inner top pad + body + buttons row + bottom
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	if height < 12 {
		height = 12
	}
	return height
}

// MassRenamePreviewViewportRows returns the number of preview lines shown for a terminal height.
func MassRenamePreviewViewportRows(layoutHeight int) int {
	maxBody := layoutHeight - 14
	if maxBody < 3 {
		maxBody = 3
	}
	return maxBody
}

func drawMassRenameDialog(screen tcell.Screen, rect Rect, state FileDialogState, borderStyle tcell.Style, styles theme.Theme) {
	_, dbg, _ := styles.DialogSurface.Decompose()
	labelStyle := styles.DialogText.Background(dbg)
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
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	for fi := 0; fi < 2 && fi < len(state.Fields); fi++ {
		field := state.Fields[fi]
		focusIdx := 2 + fi
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
			if hint := strings.TrimSpace(state.MassRenamePatternCompileHint); hint != "" {
				if y >= innerBottom {
					return
				}
				primitive.Text(screen, leftCol, y, innerW, hint, warnStyle)
				y++
			}
		}
	}

	if state.MassRenameMode == MassRenameModeUISimple {
		if y >= innerBottom {
			return
		}
		// One cell left of radios/fields so "[ ]" aligns with "( )" marker column.
		draw.DrawDialogCheckbox(screen, leftCol-1, y, "Case insensitive find", 'i', state.MassRenameCaseFold, state.FocusedField == 4, styles)
		y++
	}

	if y >= innerBottom {
		return
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	vp := innerBottom - y - 2 // hint row below preview; row innerBottom-1 is the global separator above buttons
	if vp < 1 {
		vp = 1
	}
	before := state.MassRenamePreviewBefore
	after := state.MassRenamePreviewAfter
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
		if utf8.RuneCountInString(lb) > leftW {
			lb = primitive.TruncateRight(lb, leftW)
		}
		if utf8.RuneCountInString(rb) > rightW {
			rb = primitive.TruncateRight(rb, rightW)
		}
		primitive.Text(screen, leftCol, y, leftW, lb, labelStyle)
		if sepW == 1 {
			screen.SetContent(leftCol+leftW, y, ' ', nil, labelStyle)
		}
		primitive.Text(screen, leftCol+leftW+sepW, y, rightW, rb, labelStyle)
		y++
	}
	if y >= innerBottom {
		return
	}
	hint := "PgUp/PgDn scroll"
	hintX := leftCol + 1
	lineW := innerW - 2 // one column inset on the left + one on the right
	if lineW < 1 {
		lineW = 1
		hintX = leftCol
	}
	if hintX+lineW > leftCol+innerW {
		hintX = leftCol + innerW - lineW
	}
	if hintX < leftCol {
		hintX = leftCol
	}
	hl := utf8.RuneCountInString(hint)
	if hl > lineW {
		hint = primitive.TruncateRight(hint, lineW)
		hl = utf8.RuneCountInString(hint)
	}
	pad := lineW - hl
	if pad < 0 {
		pad = 0
	}
	primitive.Text(screen, hintX, y, lineW, strings.Repeat(" ", pad)+hint, labelStyle)
}
