package dialog

import (
	"unicode/utf8"

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
		{config.PreviewImageProtocolUeberzug, "Überzug++", 'z'},
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
	previewSettingsDialogFocusMetadataFirst    = 13
	previewSettingsDialogFocusVideoMetadata    = 17
	previewSettingsDialogFocusOK               = 18
	previewSettingsDialogFocusCancel           = 19
)

// PreviewSettingsDialogForm is the dialog's radio/checkbox/button focus layout, shared by the
// render and key-handling code: sixel(0-2) | kitty(3-5) | placeholder(6-8) | protocol(9-12) |
// metadata(13-16) | video(17) | buttons(18-19).
func PreviewSettingsDialogForm() DialogLinearForm {
	return NewDialogLinearForm(18).WithSegments(0, 3, 6, 9, 13, 17)
}

// DrawPreviewSettingsDialog renders the M-F3 preview settings modal.
func DrawPreviewSettingsDialog(screen tcell.Screen, layout Layout, state PreviewSettingsDialogState, styles theme.Theme) {
	const width, height = 46, 23
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

// previewSettingsCapabilityLabelW is the label column of the capability rows: the longest
// label ("Placeholder") plus one space, so the radios line up.
const previewSettingsCapabilityLabelW = len("Placeholder") + 1

// drawPreviewSettingsCapabilityGroup draws the group label and its auto/yes/no radios on one
// row (Left/Right moves within the row, see
// PreviewSettingsDialogCapabilityMoveFocus).
func drawPreviewSettingsCapabilityGroup(
	screen tcell.Screen,
	optionX int,
	y int,
	label string,
	value string,
	focus int,
	focusFirst int,
	styles theme.Theme,
) int {
	_, dbg, _ := styles.DialogSurface.Decompose()
	primitive.Text(screen, optionX+1, y, previewSettingsCapabilityLabelW, label, styles.DialogText.Background(dbg))
	x := optionX + 1 + previewSettingsCapabilityLabelW - 1 // radio marker starts with a space
	for i, r := range PreviewSettingsDialogCapabilityRadios() {
		draw.DrawDialogRadio(screen, x, y, r.Label, r.Shortcut, value == r.Value, focus == focusFirst+i, false, styles)
		x += utf8.RuneCountInString(" "+styles.IconDialogRadio(false)+" ") + utf8.RuneCountInString(r.Label) + 1
	}
	return y + 1
}

// PreviewSettingsDialogCapabilityMoveFocus handles arrow keys inside the three horizontal
// capability rows: Left/Right stay within the row, Up/Down keep the column across rows and
// Down from the last row enters the Active protocol group. ok is false outside those rows.
func PreviewSettingsDialogCapabilityMoveFocus(focus int, key tcell.Key) (int, bool) {
	n := len(PreviewSettingsDialogCapabilityRadios())
	if focus < previewSettingsDialogFocusSixelFirst || focus >= previewSettingsDialogFocusProtocolFirst {
		return focus, false
	}
	col := focus % n
	switch key {
	case tcell.KeyLeft:
		return focus - min(col, 1), true
	case tcell.KeyRight:
		return focus + min(n-1-col, 1), true
	case tcell.KeyUp:
		return max(focus-n, col), true
	case tcell.KeyDown:
		if focus+n >= previewSettingsDialogFocusProtocolFirst {
			return previewSettingsDialogFocusProtocolFirst, true
		}
		return focus + n, true
	}
	return focus, false
}
