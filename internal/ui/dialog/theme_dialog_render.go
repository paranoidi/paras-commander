package dialog

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// themePreviewRows is how many rows the preview pane paints (a blank row, four input demos of
// three rows, two checkboxes, a blank row, a rule and the buttons); on a short screen it is cut off.
const themePreviewRows = 18

// themeMinListRows is the fewest theme rows the dialog shows; below that it is not drawn.
const themeMinListRows = 4

// Widget IDs of the theme dialog. The list is driven by the filter input and has no focus of its
// own; the dialog's Focus is ThemeFilter, dlg.OK or dlg.Cancel.
const (
	ThemeFilter dlg.ID = "theme-filter"
	ThemeList   dlg.ID = "themes"
)

// themePreviewPane is the reserved region the live widget preview is painted into.
const themePreviewPane dlg.ID = "theme-preview"

// themeDialogListWidth is the width of the left (list) column: the widest label plus the radio
// marker and spacing.
func themeDialogListWidth(state ThemeDialogState) int {
	listWidth := 0
	for _, choice := range state.Choices {
		// marker (3) + label + spacing
		listWidth = max(listWidth, utf8.RuneCountInString(choice.Label)+6)
	}
	return max(16, listWidth)
}

// ThemeDialogListRows returns the theme list's height on layout, or 0 when the dialog does not fit.
func ThemeDialogListRows(state ThemeDialogState, layout Layout, styles theme.Theme) int {
	return fittedListHeight(themeFrame(state, layout, styles), layout, ThemeList)
}

// EnsureThemeListScroll keeps Selected in range and visible in the theme list of height listRows.
func EnsureThemeListScroll(state *ThemeDialogState, listRows int) {
	EnsurePickerScroll(len(state.Ranked), &state.Selected, &state.ListScroll, listRows)
}

// ThemeDialogView describes the theme dialog: a fuzzy filter input steering a radio-marked list
// (the cursor row's radio is lit) next to a reserved preview pane (painted by DrawThemeDialog),
// over OK / Cancel, fitted to layout: on a short screen the list and the pane shrink together.
func ThemeDialogView(state ThemeDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	spec, _ := fillList(themeFrame(state, layout, styles), layout, ThemeList, func(height, _ int) dlg.Item {
		list := dlg.Item{
			ID: ThemeList, Kind: dlg.KindList, Driven: true, Markers: true,
			Cursor: state.Selected, Offset: state.ListScroll, Height: height,
		}
		for i, ent := range state.Ranked {
			var row dlg.ListRow
			if ent >= 0 && ent < len(state.Choices) {
				row = dlg.ListRow{Text: state.Choices[ent].Label, Marked: i == state.Selected}
				if ent < len(state.MatchRanges) {
					row.Match = state.MatchRanges[ent]
				}
			}
			list.Rows = append(list.Rows, row)
		}
		return list
	})
	return spec
}

// themeFrame is the theme dialog spec with an empty list as tall as the choices.
func themeFrame(state ThemeDialogState, layout Layout, styles theme.Theme) dlg.Spec {
	const previewWidthMin = 40 // minimum width for the right column
	listWidth := themeDialogListWidth(state)
	filter := dlg.Item{
		ID: ThemeFilter, Kind: dlg.KindInput, Drives: ThemeList, LeadingIcon: styles.IconSearchIcon(),
		Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll,
	}
	return dlg.Spec{
		Title: "Theme", Width: listWidth + 10 + previewWidthMin, MinWidth: listWidth + 10, Unclamped: true, Margin: true,
		Groups: []dlg.Group{{
			ColWidths: []int{listWidth + 2},
			Columns: [][]dlg.Item{
				{filter, {ID: ThemeList, Kind: dlg.KindList, Height: len(state.Choices), MinHeight: themeMinListRows}},
				{{ID: themePreviewPane, Kind: dlg.KindSpacer, Height: themePreviewRows}},
			},
		}},
		Buttons: dlg.OKCancelButtons(),
	}
}

func DrawThemeDialog(screen tcell.Screen, layout Layout, state ThemeDialogState, styles theme.Theme) {
	if len(state.Choices) == 0 {
		return
	}
	focus := state.Focus
	if focus == "" {
		focus = ThemeFilter
	}
	p, ok := dlg.Draw(screen, layout.Width, layout.Height, ThemeDialogView(state, layout, styles), focus, styles)
	if !ok {
		return
	}
	pane, _ := p.Item(themePreviewPane)
	drawThemePreview(screen, p, pane.Rect, styles)
}

// drawThemePreview paints the widget preview in the pane right of the list: the vertical rule
// (with its tees on the frame), sample inputs, checkboxes and buttons in the theme being previewed.
func drawThemePreview(screen tcell.Screen, p dlg.Placement, pane draw.Rect, styles theme.Theme) {
	rect := p.Rect
	_, dbg, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(dbg)
	sepCol := pane.X + 1 // the rule sits one cell into the pane
	previewLeft := sepCol + 1
	previewWidth := rect.X + rect.Width - 2 - previewLeft
	if previewWidth < 20 {
		previewWidth = 20
	}
	outerSepY := p.SepY

	// Vertical separator between the columns (excluding top, outer-sep and bottom rows).
	for y := rect.Y + 1; y < outerSepY; y++ {
		screen.SetContent(sepCol, y, '│', nil, borderStyle)
	}

	previewY := rect.Y + 1
	// --- Input field examples (dialog input vs placeholder, focus cursor) ---
	previewY++ // blank line after dialog top
	labelStyle := styles.DialogText.Background(dbg)
	demoPath := "/home/user/demo.txt"
	cursorEnd := utf8.RuneCountInString(demoPath)
	// drawInputField paints end-of-line cursor as a reversed space (looks like a stray block in the preview).
	cursorOnLastRune := max(0, cursorEnd-1)
	inputWidth := previewWidth - 1

	// The pane is only as tall as the list: on a short screen the preview is cut off at the outer
	// separator instead of spilling over the frame.
	fits := func(y int) bool { return y < outerSepY }

	drawThemePreviewInputPair := func(label string, field FileDialogField, focused bool) {
		if fits(previewY) {
			primitive.Text(screen, previewLeft, previewY, previewWidth, " "+label, labelStyle)
		}
		previewY++
		if inputWidth > 0 && fits(previewY) {
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
	if fits(previewY) {
		draw.DrawDialogCheckbox(screen, previewLeft, previewY, "Selected", 'S', true, false, false, styles)
	}
	previewY++
	if fits(previewY) {
		draw.DrawDialogCheckbox(screen, previewLeft, previewY, "Unselected", 'U', false, false, false, styles)
	}
	previewY++

	// --- Separator ---
	previewY++ // blank

	// Draw a simulated separator using ─ characters across the preview width,
	// connecting to the dialog right border with ┤.
	previewHSepY := previewY
	previewSepStyle := borderStyle
	rightBorderCol := rect.X + rect.Width - 1 // dialog right border column
	for x := previewLeft; x <= rightBorderCol && fits(previewHSepY); x++ {
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
	if fits(previewY) {
		draw.DrawDialogButton(screen, btnStartX, previewY, "Selected", 'S', false, false, false, styles)
		btnStartX += draw.DialogButtonWidth("Selected") + btnGap
		draw.DrawDialogButton(screen, btnStartX, previewY, "Unselected", 'U', true, false, false, styles)
	}

	// Override box-drawing connector characters at the vertical separator column
	// where it intersects horizontal lines. Must be after all draw calls.
	screen.SetContent(sepCol, rect.Y, '┬', nil, borderStyle) // top border: tee down
	if fits(previewHSepY) {
		screen.SetContent(sepCol, previewHSepY, '├', nil, borderStyle) // preview separator: tee right
	}
	screen.SetContent(sepCol, outerSepY, '┴', nil, borderStyle) // outer separator: tee up (no vertical below)
}
