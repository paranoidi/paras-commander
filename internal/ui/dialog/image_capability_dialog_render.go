package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// ImageCapabilityDialogRadio describes one Auto/Sixel/Kitty protocol-override radio row.
type ImageCapabilityDialogRadio struct {
	Protocol string
	Label    string
	Shortcut rune
}

// ImageCapabilityDialogRadios returns the Active protocol radio rows in display order.
func ImageCapabilityDialogRadios() []ImageCapabilityDialogRadio {
	return []ImageCapabilityDialogRadio{
		{config.PreviewImageProtocolAuto, "Auto", 'a'},
		{config.PreviewImageProtocolSixel, "Sixel", 'i'},
		{config.PreviewImageProtocolKitty, "Kitty", 't'},
	}
}

const (
	imageCapabilityDialogFocusSixelCheckbox = 0
	imageCapabilityDialogFocusKittyCheckbox = 1
	imageCapabilityDialogFocusPlaceholder   = 2
	// ImageCapabilityDialogFocusProtocolRadio is the first protocol radio; the rest follow in
	// ImageCapabilityDialogRadios order.
	ImageCapabilityDialogFocusProtocolRadio = 3
	imageCapabilityDialogFocusOK            = 6
	imageCapabilityDialogFocusCancel        = 7
)

// ImageCapabilityDialogForm is the dialog's checkbox/radio/button focus layout, shared by the
// render and key-handling code: checkboxes(0-2) | protocol radios(3-5) | buttons(6-7).
func ImageCapabilityDialogForm() DialogLinearForm {
	return NewDialogLinearForm(6).WithSegments(0, 3, 6)
}

// DrawImageCapabilityDialog renders the M-F3 image terminal-capabilities modal.
func DrawImageCapabilityDialog(screen tcell.Screen, layout Layout, state ImageCapabilityDialogState, styles theme.Theme) {
	// height: border, prompt, blank, 3 checkboxes, separator, label, blank, 3 protocol radios,
	// separator, blank, buttons, border.
	const width, height = 46, 16
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Image Terminal Capabilities", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	textX, optionX, textW := draw.DialogTextX(rect), draw.DialogOptionX(rect), draw.DialogContentWidth(rect)

	y := rect.Y + 1
	primitive.Text(screen, textX, y, textW, "Confirm terminal capabilities:", textStyle)
	y += 2
	draw.DrawDialogCheckbox(screen, optionX, y, "Sixel supported", 's', state.SixelSupported, state.Focus == imageCapabilityDialogFocusSixelCheckbox, styles)
	y++
	draw.DrawDialogCheckbox(screen, optionX, y, "Kitty supported", 'k', state.KittySupported, state.Focus == imageCapabilityDialogFocusKittyCheckbox, styles)
	y++
	draw.DrawDialogCheckbox(screen, optionX, y, "Kitty placeholder supported", 'p', state.KittyPlaceholderSupported, state.Focus == imageCapabilityDialogFocusPlaceholder, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, textX, y, textW, "Active protocol:", textStyle)
	y += 2
	for i, r := range ImageCapabilityDialogRadios() {
		draw.DrawDialogRadio(screen, optionX, y, r.Label, r.Shortcut, state.Protocol == r.Protocol, state.Focus == ImageCapabilityDialogFocusProtocolRadio+i, styles)
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)

	buttonY := rect.Y + rect.Height - 2
	okFocused := state.Focus == imageCapabilityDialogFocusOK
	cancelFocused := state.Focus == imageCapabilityDialogFocusCancel
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, okFocused, cancelFocused, styles)
}
