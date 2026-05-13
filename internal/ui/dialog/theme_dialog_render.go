package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

const (
	// Preview column minimum inner height: top pad(1) + four input demos (label+input+gap each) + checkboxes(2) + pad(1) + sep(1) + buttons(1)
	themeDialogPreviewRows = 17
	// List uses y = rect.Y+1 through y = rect.Y+rect.Height-4 (row above outer separator at Height-3).
	themeDialogListViewportExtra = 4
)

// ThemeDialogListViewportRows is how many theme rows fit in the left column; use for PageUp/PageDown in the theme dialog.
func ThemeDialogListViewportRows(layout geom.Layout, choiceCount int) int {
	dh := themeDialogClampedHeight(layout, choiceCount)
	if dh < 8 {
		return 1
	}
	return dh - themeDialogListViewportExtra
}

func themeDialogClampedHeight(layout geom.Layout, choiceCount int) int {
	const chromeHeight = 5 // border(1) + blank(1) + outer-sep(1) + outer-buttons(1) + border(1)
	dialogHeight := max(choiceCount, themeDialogPreviewRows) + chromeHeight
	if dialogHeight > layout.Height-2 {
		dialogHeight = layout.Height - 2
	}
	return dialogHeight
}

func DrawThemeDialog(screen tcell.Screen, layout Layout, state ThemeDialogState, styles theme.Theme) {
	if len(state.Choices) == 0 {
		return
	}

	const previewWidthMin = 40 // minimum width for the right column

	// Calculate list column width.
	listWidth := 0
	for _, choice := range state.Choices {
		// marker (3) + label + spacing
		w := utf8.RuneCountInString(choice.Label) + 6
		if w > listWidth {
			listWidth = w
		}
	}
	listWidth = max(16, listWidth)

	// Total dialog width: list column + separator + preview column + margins.
	dialogWidth := listWidth + 10 + previewWidthMin
	if dialogWidth > layout.Width-4 {
		dialogWidth = layout.Width - 4
	}
	if dialogWidth < listWidth+10 {
		dialogWidth = listWidth + 10
	}

	dialogHeight := themeDialogClampedHeight(layout, len(state.Choices))
	if dialogHeight < 8 {
		return
	}

	rect := draw.CenteredDialogRect(layout, dialogWidth, dialogHeight)
	borderStyle := draw.DrawDialogFrame(screen, rect, "Theme", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	// Layout columns.
	leftCol := rect.X + 2 // 1 space margin at left
	listRightEdge := leftCol + listWidth
	sepCol := listRightEdge + 2 // space before vertical separator
	previewLeft := sepCol + 1   // after vertical line
	previewWidth := rect.X + rect.Width - 2 - previewLeft
	if previewWidth < 20 {
		previewWidth = 20
	}

	// ============================================================
	// LEFT COLUMN: Theme list (radio items)
	// ============================================================
	visibleRows := ThemeDialogListViewportRows(layout, len(state.Choices))
	start := themeDialogStart(state.Selected, visibleRows, len(state.Choices))
	listTopY := rect.Y + 1

	for row := 0; row < visibleRows && start+row < len(state.Choices); row++ {
		choice := state.Choices[start+row]
		idx := start + row
		y := listTopY + row

		style := draw.DialogOptionRowStyle(false, idx == state.Selected, styles)
		if state.Focus == 0 && idx == state.Selected {
			style = draw.DialogOptionRowStyle(true, true, styles)
		}
		marker := "( )"
		if idx == state.Selected {
			marker = "(*)"
		}
		part := " " + marker + " "
		primitive.Text(screen, leftCol, y, utf8.RuneCountInString(part), part, style)
		labelX := leftCol + utf8.RuneCountInString(part)
		labelMax := listRightEdge - labelX
		if labelMax > 0 {
			text := fmt.Sprintf("%-*s", labelMax, choice.Label)
			primitive.Text(screen, labelX, y, labelMax, text, style)
		}
	}

	// ============================================================
	// VERTICAL SEPARATOR: Draw │ between columns (excluding top, outer-sep, bottom rows)
	// ============================================================
	outerSepY := rect.Y + rect.Height - 3
	sepStyle := borderStyle
	for y := rect.Y + 1; y < outerSepY; y++ {
		screen.SetContent(sepCol, y, '│', nil, sepStyle)
	}

	// ============================================================
	// RIGHT COLUMN: Widget preview
	// ============================================================
	previewY := rect.Y + 1
	// Check we have room before the outer separator.
	if previewY >= outerSepY {
		return
	}

	// --- Input field examples (dialog input vs placeholder, focus cursor) ---
	previewY++ // blank line after dialog top
	labelStyle := styles.DialogText.Background(dbg)
	demoPath := "/home/user/demo.txt"
	cursorEnd := utf8.RuneCountInString(demoPath)
	// drawInputField paints end-of-line cursor as a reversed space (looks like a stray block in the preview).
	cursorOnLastRune := max(0, cursorEnd-1)
	inputWidth := previewWidth - 1

	drawThemePreviewInputPair := func(label string, field FileDialogField, focused bool) {
		primitive.Text(screen, previewLeft, previewY, previewWidth, " "+label, labelStyle)
		previewY++
		if inputWidth > 0 {
			drawInputField(screen, previewLeft+1, previewY, inputWidth, field, focused, styles)
		}
		previewY++ // empty row after input area
		previewY++
	}

	drawThemePreviewInputPair("Inactive input with text:", FileDialogField{
		Value: demoPath, Cursor: cursorEnd,
	}, false)
	drawThemePreviewInputPair("Inactive input with default value:", FileDialogField{
		Value: demoPath, Prefill: demoPath, PrefillPending: true, Cursor: cursorEnd,
	}, false)
	drawThemePreviewInputPair("Active input with text:", FileDialogField{
		Value: demoPath, Cursor: cursorOnLastRune,
	}, true)
	drawThemePreviewInputPair("Active input with default value:", FileDialogField{
		Value: demoPath, Prefill: demoPath, PrefillPending: true, Cursor: cursorOnLastRune,
	}, true)

	// --- Checkboxes ---
	draw.DrawDialogCheckbox(screen, previewLeft, previewY, "Selected", 'S', true, false, styles)
	previewY++
	draw.DrawDialogCheckbox(screen, previewLeft, previewY, "Unselected", 'U', false, false, styles)
	previewY++

	// --- Separator ---
	previewY++ // blank

	// Draw a simulated separator using ─ characters across the preview width,
	// connecting to the dialog right border with ┤.
	previewHSepY := previewY
	previewSepStyle := borderStyle
	rightBorderCol := rect.X + rect.Width - 1 // dialog right border column
	for x := previewLeft; x <= rightBorderCol; x++ {
		ch := '─'
		if x == rightBorderCol {
			ch = '┤'
		}
		screen.SetContent(x, previewHSepY, ch, nil, previewSepStyle)
	}
	previewY++

	// --- Buttons ---
	const btnGap = 2
	btnWidth := draw.DialogButtonWidth("Selected") + btnGap + draw.DialogButtonWidth("Unselected")
	btnStartX := previewLeft + (previewWidth-btnWidth)/2
	if btnStartX < previewLeft {
		btnStartX = previewLeft
	}
	draw.DrawDialogButton(screen, btnStartX, previewY, "Selected", 'S', false, styles)
	btnStartX += draw.DialogButtonWidth("Selected") + btnGap
	draw.DrawDialogButton(screen, btnStartX, previewY, "Unselected", 'U', true, styles)

	// ============================================================
	// OUTER SEPARATOR
	// ============================================================
	draw.DrawDialogHSeparator(screen, rect, outerSepY, borderStyle)

	// ============================================================
	// OUTER BUTTONS
	// ============================================================
	btnY := outerSepY + 1
	okFocused := state.Focus == 1
	cancelFocused := state.Focus == 2
	draw.DrawDialogButtonRowCentered(screen, rect, btnY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: okFocused},
		{Label: "Cancel", Shortcut: 'C', Focused: cancelFocused},
	}, styles)

	// Override box-drawing connector characters at the vertical separator column
	// where it intersects horizontal lines. Must be after all draw calls.
	screen.SetContent(sepCol, rect.Y, '┬', nil, borderStyle)       // top border: tee down
	screen.SetContent(sepCol, previewHSepY, '├', nil, borderStyle) // preview separator: tee right
	screen.SetContent(sepCol, outerSepY, '┴', nil, borderStyle)    // outer separator: tee up (no vertical below)
}

func themeDialogStart(selected, visibleRows, total int) int {
	return geom.ScrollOffset(selected, visibleRows, total)
}
