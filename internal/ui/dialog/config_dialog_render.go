package dialog

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

const configDialogScrollbarStyleLabel = "Scrollbar style:"

// configDialogScrollbarColumns returns label and option x positions for the centered scrollbar-style block.
func configDialogScrollbarColumns(rect draw.Rect) (labelCol, optionCol int) {
	label := configDialogScrollbarStyleLabel
	labelW := utf8.RuneCountInString(label)
	contentWidth := draw.DialogContentWidth(rect)
	centerCol := draw.DialogTextX(rect) + contentWidth/2
	labelCol = centerCol - labelW/2
	optionCol = labelCol - 1
	return labelCol, optionCol
}

const configDialogHorizontalSplitLabel = "Start in horizontal split mode"

func DrawConfigDialog(screen tcell.Screen, layout Layout, state ConfigDialogState, styles theme.Theme) {
	const (
		width     = 54
		minWidth  = 38
		minHeight = 23
	)
	rect, ok := draw.ClampCenteredDialogRect(layout, width, minHeight, minWidth, minHeight)
	if !ok {
		return
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, "Configuration", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	primaryCol := draw.DialogTextX(rect)
	leftOptionCol := draw.DialogOptionX(rect)
	y := rect.Y + 1
	primitive.Text(screen, primaryCol, y, rect.Width-4, "View options:", styles.DialogText.Background(dbg))
	y++
	y++
	draw.DrawDialogCheckbox(screen, leftOptionCol, y, "Show file icons", 'f', state.ShowFileIcons, state.Focus == 0, styles)
	y++
	draw.DrawDialogCheckbox(screen, leftOptionCol, y, "Zoom active panel", 'z', state.ZoomActivePanel, state.Focus == 1, styles)
	y++
	draw.DrawDialogCheckbox(screen, leftOptionCol, y, "Shrunken shows only name", 's', state.ShrunkenShowsNameOnly, state.Focus == 2, styles)
	y++
	draw.DrawDialogCheckbox(screen, leftOptionCol, y, configDialogHorizontalSplitLabel, 'h', state.PaneSplitStacked, state.Focus == 3, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	scrollModeLabel := "Scroll mode:"
	sbLabel := configDialogScrollbarStyleLabel
	sbLabelW := utf8.RuneCountInString(sbLabel)
	rightLabelCol, rightOptionCol := configDialogScrollbarColumns(rect)

	primitive.Text(screen, primaryCol, y, utf8.RuneCountInString(scrollModeLabel), scrollModeLabel, styles.DialogText.Background(dbg))
	primitive.Text(screen, rightLabelCol, y, sbLabelW, sbLabel, styles.DialogText.Background(dbg))
	y++
	y++

	scrollRadios := panel.ScrollModeDialogRadios()
	sbRadios := uiscrollbar.DialogRadios()
	scrollMode := panel.EffectiveScrollMode(state.ScrollMode)
	sb := uiscrollbar.EffectiveStyle(state.PanelScrollbar)
	scrollRows := max(len(scrollRadios), len(sbRadios))
	for i := 0; i < scrollRows; i++ {
		if i < len(scrollRadios) {
			r := scrollRadios[i]
			draw.DrawDialogRadio(screen, leftOptionCol, y, r.Label, r.Shortcut, scrollMode == r.Mode, state.Focus == ConfigDialogScrollModeFocus(i), styles)
		}
		if i < len(sbRadios) {
			r := sbRadios[i]
			draw.DrawDialogRadio(screen, rightOptionCol, y, r.Label, r.Shortcut, sb == r.Style, state.Focus == ConfigDialogScrollbarFocus(i), styles)
		}
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, primaryCol, y, rect.Width-4, "Default listing format:", styles.DialogText.Background(dbg))
	y++
	y++

	lf := panel.EffectiveListFormat(state.ListFormat)
	listRadios := panel.ListFormatDialogRadios()
	for i, r := range listRadios {
		draw.DrawDialogRadio(screen, leftOptionCol, y, r.Label, r.Shortcut, lf == r.Format, state.Focus == configDialogFocusListingFirst+i, styles)
		y++
	}

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	okFocused := state.Focus == configDialogFocusOK
	cancelFocused := state.Focus == configDialogFocusCancel
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, okFocused, cancelFocused, styles)
}
