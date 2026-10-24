package dialog

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
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

// ThemeList is the theme list's widget ID; the dialog's Focus is ThemeList, dlg.OK or dlg.Cancel.
const ThemeList dlg.ID = "themes"

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

// ThemeDialogView describes the theme dialog: a radio-marked list that wraps and stays centered
// on the cursor next to a reserved preview pane (painted by DrawThemeDialog), over OK / Cancel.
// layout fixes how many rows the list shows (ThemeDialogListViewportRows).
func ThemeDialogView(state ThemeDialogState, layout Layout) dlg.Spec {
	const previewWidthMin = 40 // minimum width for the right column
	listWidth := themeDialogListWidth(state)
	rows := ThemeDialogListViewportRows(layout, len(state.Choices))
	list := dlg.Item{
		ID: ThemeList, Kind: dlg.KindList, Cursor: state.Selected, Height: rows,
		Wrap: true, CenterScroll: true, PageStep: rows, Markers: true,
	}
	for i, choice := range state.Choices {
		list.Rows = append(list.Rows, dlg.ListRow{Text: choice.Label, Marked: i == state.Selected})
	}
	return dlg.Spec{
		Title: "Theme", Width: listWidth + 10 + previewWidthMin, MinWidth: listWidth + 10, Unclamped: true,
		PlainButtonKeys: true,
		Groups: []dlg.Group{{
			ColWidths: []int{listWidth + 2},
			Columns: [][]dlg.Item{
				{list},
				{{ID: themePreviewPane, Kind: dlg.KindSpacer, Height: rows}},
			},
		}},
		Buttons: []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}, {ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}

func DrawThemeDialog(screen tcell.Screen, layout Layout, state ThemeDialogState, styles theme.Theme) {
	if len(state.Choices) == 0 || themeDialogClampedHeight(layout, len(state.Choices)) < 8 {
		return
	}
	spec := ThemeDialogView(state, layout)
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
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
