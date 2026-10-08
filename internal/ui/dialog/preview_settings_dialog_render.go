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

// PreviewSettingsDialogCapabilityRadios returns the Auto/Yes/No rows shared by the three
// terminal-capability groups (Sixel, Kitty, Placeholder).
func PreviewSettingsDialogCapabilityRadios() []PreviewSettingsDialogRadio {
	return []PreviewSettingsDialogRadio{
		{config.PreviewTerminalCapabilityAuto, "auto", 0},
		{config.PreviewTerminalCapabilityYes, "yes", 0},
		{config.PreviewTerminalCapabilityNo, "no", 0},
	}
}

const (
	previewSettingsDialogFocusSixelFirst       = 0
	previewSettingsDialogFocusKittyFirst       = 3
	previewSettingsDialogFocusPlaceholderFirst = 6
	previewSettingsDialogFocusProtocolFirst    = 9
	previewSettingsDialogFocusMetadataFirst    = 12
	previewSettingsDialogFocusVideoMetadata    = 16
	previewSettingsDialogFocusOK               = 17
	previewSettingsDialogFocusCancel           = 18
)

// PreviewSettingsDialogForm is the dialog's radio/checkbox/button focus layout, shared by the
// render and key-handling code: sixel(0-2) | kitty(3-5) | placeholder(6-8) | protocol(9-11) |
// metadata(12-15) | video(16) | buttons(17-18).
func PreviewSettingsDialogForm() DialogLinearForm {
	return NewDialogLinearForm(17).WithSegments(0, 3, 6, 9, 12, 16)
}

// DrawPreviewSettingsDialog renders the M-F3 preview settings modal.
func DrawPreviewSettingsDialog(screen tcell.Screen, layout Layout, state PreviewSettingsDialogState, styles theme.Theme) {
	const width, height = 46, 28
	rect := draw.CenteredDialogRect(layout, width, height)

	borderStyle := draw.DrawDialogFrame(screen, rect, "Preview settings", styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	textX, optionX, textW := draw.DialogTextX(rect), draw.DialogOptionX(rect), draw.DialogContentWidth(rect)

	y := rect.Y + 1
	primitive.Text(screen, textX, y, textW, "Confirm terminal capabilities:", textStyle)
	y++
	y = drawPreviewSettingsCapabilityGroup(screen, optionX, y, "Sixel", state.Sixel, state.Focus, previewSettingsDialogFocusSixelFirst, styles)
	y = drawPreviewSettingsCapabilityGroup(screen, optionX, y, "Kitty", state.Kitty, state.Focus, previewSettingsDialogFocusKittyFirst, styles)
	y = drawPreviewSettingsCapabilityGroup(screen, optionX, y, "Placeholder", state.KittyPlaceholder, state.Focus, previewSettingsDialogFocusPlaceholderFirst, styles)
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, textX, y, textW, "Active protocol:", textStyle)
	y++
	for i, r := range PreviewSettingsDialogProtocolRadios() {
		draw.DrawDialogRadio(screen, optionX, y, r.Label, r.Shortcut, state.Protocol == r.Value, state.Focus == previewSettingsDialogFocusProtocolFirst+i, false, styles)
		y++
	}
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	primitive.Text(screen, textX, y, textW, "Image metadata:", textStyle)
	y++
	for i, r := range PreviewSettingsDialogImageMetadataRadios() {
		draw.DrawDialogRadio(screen, optionX, y, r.Label, r.Shortcut, state.ImageMetadata == r.Value, state.Focus == previewSettingsDialogFocusMetadataFirst+i, false, styles)
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

func drawPreviewSettingsCapabilityGroup(
	screen tcell.Screen,
	optionX int,
	y int,
	prefix string,
	value string,
	focus int,
	focusFirst int,
	styles theme.Theme,
) int {
	for i, r := range PreviewSettingsDialogCapabilityRadios() {
		draw.DrawDialogRadio(screen, optionX, y, prefix+" "+r.Label, r.Shortcut, value == r.Value, focus == focusFirst+i, false, styles)
		y++
	}
	return y
}
