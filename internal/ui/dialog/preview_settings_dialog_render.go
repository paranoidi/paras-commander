package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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

// DrawPreviewSettingsDialog renders the M-F3 preview settings modal from its widget spec (see
// PreviewSettingsDialogView).
func DrawPreviewSettingsDialog(screen tcell.Screen, layout Layout, state PreviewSettingsDialogState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, PreviewSettingsDialogView(state, dlg.MarkerWidth(styles)), state.Focus, styles)
}
