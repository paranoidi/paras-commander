package dialog

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// Rows of the app preview page's samples (ThemeAppSamples): the menu bar and its pulldown, one row
// per job status and the progress bars.
const (
	ThemeAppMenuRows     = 5
	ThemeAppJobRows      = 9
	ThemeAppProgressRows = 3
	// ThemePanelRows is the height of the file panel sample (frame, header and entry rows).
	ThemePanelRows = 18
)

// ThemeSamplePainter paints one app sample at x, y, width wide, leaving every row from maxY on
// alone. The samples draw with the app's own chrome helpers, which package dialog cannot reach, so
// the caller of DrawThemeDialog supplies them.
type ThemeSamplePainter func(screen tcell.Screen, x, y, width, maxY int, styles theme.Theme)

// ThemeAppSamples are the samples of the app preview pages; a nil painter leaves its rows blank.
type ThemeAppSamples struct {
	MenuBar  ThemeSamplePainter // ThemeAppMenuRows rows: the bar, then its open pulldown
	Jobs     ThemeSamplePainter // ThemeAppJobRows rows: one per job status
	Progress ThemeSamplePainter // ThemeAppProgressRows rows: active, inactive and completed bars
	Panel    ThemeSamplePainter // ThemePanelRows rows: a file panel with its frame, header and entries
}

// themePreviewCtx is what a preview page paints with: the pane's geometry (y is the page's first row,
// maxY the outer separator the page is cut off at) and the caller's app samples. A page that wants the
// sample tooltip sets activeInput.
type themePreviewCtx struct {
	screen               tcell.Screen
	left, y, width       int
	rightBorderCol, maxY int
	app                  ThemeAppSamples
	styles               theme.Theme
	activeInput          draw.Rect
}

// themePreviewPage is one page of the theme picker's preview pane: the rows it needs, whether the
// whole pane is filled with the app's content background first (appBG) and its painter.
type themePreviewPage struct {
	rows  int
	appBG bool
	paint func(c *themePreviewCtx)
}

// themePreviewPages are the preview pages in cycling order; add a page by appending an entry. The pane
// is as tall as the tallest page, so cycling never resizes the dialog.
var themePreviewPages = []themePreviewPage{
	{rows: themeInputPageRows, paint: paintThemeInputPage},
	{rows: ThemeAppMenuRows + ThemeAppJobRows + ThemeAppProgressRows + themeAppSectionGaps, appBG: true, paint: paintThemeAppPage},
	{rows: ThemePanelRows, appBG: true, paint: paintThemePanelPage},
}

// ThemePreviewPageCount is how many preview pages the picker cycles through.
func ThemePreviewPageCount() int { return len(themePreviewPages) }

// themePreviewPageRows is the rows of the tallest page.
func themePreviewPageRows() int {
	rows := 0
	for _, page := range themePreviewPages {
		rows = max(rows, page.rows)
	}
	return rows
}

// themePreviewRows is the pane height the dialog asks for; on a short screen the page is cut off.
func themePreviewRows() int { return themePreviewPageRows() }

// themePreviewTooltip is the sample tooltip text drawn under the active sample input, as a dialog draws the tooltip of its focused widget.
const themePreviewTooltip = "Tooltips explain the focused widget."

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
	const previewWidthMin = 60 // width the right column asks for
	previewRows := themePreviewRows()
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
				{{ID: themePreviewPane, Kind: dlg.KindSpacer, Height: previewRows}},
			},
		}},
		Buttons: dlg.OKCancelButtons(),
	}
}

// DrawThemeDialog paints the theme dialog; app supplies the samples of the app preview page.
func DrawThemeDialog(screen tcell.Screen, layout Layout, state ThemeDialogState, styles theme.Theme, app ThemeAppSamples) {
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
	drawThemePreview(screen, p, pane.Rect, state.Page, app, styles)
}

// drawThemePreview paints the widget preview in the pane right of the list: the vertical rule
// (with its tees on the frame) and the samples of preview page index page in the theme being previewed.
func drawThemePreview(screen tcell.Screen, p dlg.Placement, pane draw.Rect, page int, app ThemeAppSamples, styles theme.Theme) {
	rect := p.Rect
	_, dbg, _ := styles.DialogSurface.Decompose()
	borderStyle := styles.DialogFrame.Background(dbg)
	sepCol := pane.X + 1 // the rule sits one cell into the pane
	previewLeft := sepCol + 1
	previewWidth := rect.X + rect.Width - 1 - previewLeft // up to the right border
	if previewWidth < 20 {
		previewWidth = 20
	}
	outerSepY := p.SepY

	// Vertical separator between the columns (excluding top, outer-sep and bottom rows).
	for y := rect.Y + 1; y < outerSepY; y++ {
		screen.SetContent(sepCol, y, '│', nil, borderStyle)
	}

	// The page gets the whole right column inside the frame: from the first content row to the outer
	// separator (a short terminal cuts it off there), between the vertical rule and the right border.
	rightBorderCol := rect.X + rect.Width - 1 // dialog right border column
	ctx := &themePreviewCtx{
		screen: screen, left: previewLeft, y: rect.Y + 1, width: previewWidth,
		rightBorderCol: rightBorderCol, maxY: outerSepY, app: app, styles: styles,
	}
	if page >= 0 && page < len(themePreviewPages) {
		if themePreviewPages[page].appBG {
			_, appBG, _ := styles.PanelActiveSurface.Decompose() // ui.auxPanelContentBG
			fillStyle := tcell.StyleDefault.Background(appBG)
			for y := ctx.y; y < outerSepY; y++ {
				for x := previewLeft; x < rightBorderCol; x++ {
					screen.SetContent(x, y, ' ', nil, fillStyle)
				}
			}
		}
		themePreviewPages[page].paint(ctx)
	}

	// Last, so it overlays the samples below the input like a dialog's own tooltip.
	if ctx.activeInput.Width > 0 {
		dlg.PaintTooltipFor(screen, p, ctx.activeInput, themePreviewTooltip, styles)
	}

	// Override box-drawing connector characters at the vertical separator column
	// where it meets the frame. Must be after all draw calls.
	screen.SetContent(sepCol, rect.Y, '┬', nil, borderStyle)    // top border: tee down
	screen.SetContent(sepCol, outerSepY, '┴', nil, borderStyle) // outer separator: tee up (no vertical below)
}

// themeInputPageRows is the rows of the input page: a blank row, four input demos of three rows,
// two checkboxes, a blank row and the buttons.
const themeInputPageRows = 17

// paintThemeInputPage paints sample inputs (a tooltip under the active one), checkboxes and buttons.
func paintThemeInputPage(c *themePreviewCtx) {
	screen, styles, previewLeft, previewWidth := c.screen, c.styles, c.left, c.width
	_, dbg, _ := styles.DialogSurface.Decompose()
	labelStyle := styles.DialogText.Background(dbg)
	demoPath := "/home/user/demo.txt"
	cursorEnd := utf8.RuneCountInString(demoPath)
	// drawInputField paints end-of-line cursor as a reversed space (looks like a stray block in the preview).
	cursorOnLastRune := max(0, cursorEnd-1)
	inputWidth := previewWidth - 2 // a column of margin either side
	fits := func(y int) bool { return y < c.maxY }

	previewY := c.y + 1 // blank line after dialog top
	drawThemePreviewInputPair := func(label string, field FileDialogField, focused bool) {
		if fits(previewY) {
			primitive.Text(screen, previewLeft, previewY, previewWidth, " "+label, labelStyle)
		}
		previewY++
		if inputWidth > 0 && fits(previewY) {
			drawInputField(screen, previewLeft+1, previewY, inputWidth, field, focused, styles)
			if focused && c.activeInput.Width == 0 {
				c.activeInput = draw.Rect{X: previewLeft + 1, Y: previewY, Width: inputWidth, Height: 1}
			}
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

	if fits(previewY) {
		draw.DrawDialogCheckbox(screen, previewLeft, previewY, "Selected", 'S', true, false, false, styles)
	}
	previewY++
	if fits(previewY) {
		// Shows dialog.accent_alt: the mnemonic as drawn when it needs Alt (focus in an input).
		altStyles := styles
		altStyles.DialogAccent = styles.DialogAccentAlt
		draw.DrawDialogCheckbox(screen, previewLeft, previewY, "Unselected", 'U', false, false, false, altStyles)
	}
	previewY += 2 // blank row before the buttons

	const btnGap = 2
	btnWidth := draw.DialogButtonWidth("Selected") + btnGap + draw.DialogButtonWidth("Unselected")
	// The destructive button is as the delete dialog draws its focused one; it is left out when
	// the pane is too narrow for all three.
	withDelete := btnWidth+btnGap+draw.DialogButtonWidth("Delete") <= previewWidth
	if withDelete {
		btnWidth += btnGap + draw.DialogButtonWidth("Delete")
	}
	btnStartX := max(previewLeft, previewLeft+(previewWidth-btnWidth)/2)
	if fits(previewY) {
		draw.DrawDialogButton(screen, btnStartX, previewY, "Selected", 'S', false, false, false, styles)
		btnStartX += draw.DialogButtonWidth("Selected") + btnGap
		draw.DrawDialogButton(screen, btnStartX, previewY, "Unselected", 'U', true, false, false, styles)
		if withDelete {
			btnStartX += draw.DialogButtonWidth("Unselected") + btnGap
			draw.DrawDialogButton(screen, btnStartX, previewY, "Delete", 'D', true, true, false, styles)
		}
	}
}

// paintThemeAppPage paints the app samples as one little embedded app screen (the pane is already
// filled with the Jobs view's content background), stacked without rules and cut off at maxY.
// themeAppSectionGaps is the blank rows between the app page's menu, jobs and progress sections.
const themeAppSectionGaps = 2

func paintThemeAppPage(c *themePreviewCtx) {
	y := c.y
	for i, sample := range []struct {
		paint ThemeSamplePainter
		rows  int
	}{{c.app.MenuBar, ThemeAppMenuRows}, {c.app.Jobs, ThemeAppJobRows}, {c.app.Progress, ThemeAppProgressRows}} {
		if i > 0 {
			y++ // blank row between sections
		}
		if sample.paint != nil && y < c.maxY {
			sample.paint(c.screen, c.left+1, y, c.width-2, c.maxY, c.styles)
		}
		y += sample.rows
	}
}

// paintThemePanelPage paints the file panel sample across the pane (already filled with the app
// background); the painter clips it to maxY.
func paintThemePanelPage(c *themePreviewCtx) {
	if c.app.Panel != nil && c.y < c.maxY {
		c.app.Panel(c.screen, c.left, c.y, c.width, c.maxY, c.styles)
	}
}
