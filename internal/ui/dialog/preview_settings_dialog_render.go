package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// PreviewSettingsDialogRadio describes one radio row shared by the Active protocol and Image
// metadata radio groups: Value is the persisted config string, Label/Shortcut drive rendering
// and the mnemonic.
type PreviewSettingsDialogRadio struct {
	Value    string
	Label    string
	Shortcut rune
}

// PreviewSettingsDialogProtocolRadios returns the Active protocol radio rows in display order.
func PreviewSettingsDialogProtocolRadios() []PreviewSettingsDialogRadio {
	return []PreviewSettingsDialogRadio{
		{config.PreviewImageProtocolAuto, "Auto", 'a'},
		{config.PreviewImageProtocolSixel, "Sixel", 'i'},
		{config.PreviewImageProtocolKitty, "Kitty", 't'},
	}
}

// PreviewSettingsDialogImageMetadataRadios returns the Image metadata radio rows in display order.
func PreviewSettingsDialogImageMetadataRadios() []PreviewSettingsDialogRadio {
	return []PreviewSettingsDialogRadio{
		{config.PreviewImageMetadataOff, "Off", 'f'},
		{config.PreviewImageMetadataBasic, "Basic", 'b'},
		{config.PreviewImageMetadataEssentials, "Essentials", 'e'},
		{config.PreviewImageMetadataFull, "Full", 'u'},
	}
}

const (
	previewSettingsDialogFocusSixelCheckbox = 0
	previewSettingsDialogFocusKittyCheckbox = 1
	previewSettingsDialogFocusPlaceholder   = 2
	previewSettingsDialogFocusProtocolFirst = 3
	previewSettingsDialogFocusMetadataFirst = 6
	previewSettingsDialogFocusVideoMetadata = 10
	previewSettingsDialogFocusOK            = 11
	previewSettingsDialogFocusCancel        = 12
)

// PreviewSettingsDialogForm is the dialog's checkbox/radio/button focus layout, shared by the
// render and key-handling code: checkboxes(0-2) | protocol radios(3-5) | metadata radios(6-9) |
// video checkbox(10) | buttons(11-12).
func PreviewSettingsDialogForm() DialogLinearForm {
	return NewDialogLinearForm(11).WithSegments(0, 3, 6, 10)
}

// DrawPreviewSettingsDialog renders the M-F3 preview settings modal.
func DrawPreviewSettingsDialog(screen tcell.Screen, layout Layout, state PreviewSettingsDialogState, styles theme.Theme) {
	const width, height = 46, 22
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Preview settings", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	textX, optionX, textW := draw.DialogTextX(rect), draw.DialogOptionX(rect), draw.DialogContentWidth(rect)

	y := rect.Y + 1
	primitive.Text(screen, textX, y, textW, "Confirm terminal capabilities:", textStyle)
	y++
	draw.DrawDialogCheckbox(screen, optionX, y, "Sixel supported", 's', state.SixelSupported, state.Focus == previewSettingsDialogFocusSixelCheckbox, false, styles)
	y++
	draw.DrawDialogCheckbox(screen, optionX, y, "Kitty supported", 'k', state.KittySupported, state.Focus == previewSettingsDialogFocusKittyCheckbox, false, styles)
	y++
	draw.DrawDialogCheckbox(screen, optionX, y, "Kitty placeholder supported", 'p', state.KittyPlaceholderSupported, state.Focus == previewSettingsDialogFocusPlaceholder, false, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, textX, y, textW, "Active protocol:", textStyle)
	y++
	for i, r := range PreviewSettingsDialogProtocolRadios() {
		draw.DrawDialogRadio(screen, optionX, y, r.Label, r.Shortcut, state.Protocol == r.Value, state.Focus == previewSettingsDialogFocusProtocolFirst+i, styles)
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, textX, y, textW, "Image metadata:", textStyle)
	y++
	for i, r := range PreviewSettingsDialogImageMetadataRadios() {
		draw.DrawDialogRadio(screen, optionX, y, r.Label, r.Shortcut, state.ImageMetadata == r.Value, state.Focus == previewSettingsDialogFocusMetadataFirst+i, styles)
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	draw.DrawDialogCheckbox(screen, optionX, y, "Video metadata", 'v', state.VideoMetadata, state.Focus == previewSettingsDialogFocusVideoMetadata, false, styles)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	y++ // one blank row above the button row

	okFocused := state.Focus == previewSettingsDialogFocusOK
	cancelFocused := state.Focus == previewSettingsDialogFocusCancel
	draw.DrawOKCancelButtonRow(screen, rect, y, okFocused, cancelFocused, styles)
}
