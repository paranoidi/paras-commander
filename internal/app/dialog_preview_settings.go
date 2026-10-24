package app

import (
	"fmt"
	"os"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Preview settings dialog (M-F3) handlers. Mutates a.config.Preview in place (settings-dialog
// pattern, see docs/agents/architecture.md Package layout), so it stays in internal/app rather than
// apphandler/dialog.

func (a *App) openPreviewSettingsDialog() {
	a.clearTransientMessage()
	p := a.config.Preview
	a.model.PreviewSettingsDialog = dialog.PreviewSettingsDialogState{
		Open:             true,
		Sixel:            effectiveTerminalCapability(p.TerminalSixel),
		Kitty:            effectiveTerminalCapability(p.TerminalKitty),
		KittyPlaceholder: effectiveTerminalCapability(p.TerminalKittyPlaceholder),
		Protocol:         effectiveImageProtocol(p.ImageProtocol),
		ImageMetadata:    effectiveImageMetadata(p.ImageMetadata),
		VideoMetadata:    p.VideoMetadata,
	}
}

func effectiveImageProtocol(v string) string {
	switch v {
	case config.PreviewImageProtocolSixel, config.PreviewImageProtocolKitty, config.PreviewImageProtocolUeberzug:
		return v
	default:
		return config.PreviewImageProtocolAuto
	}
}

func effectiveImageMetadata(v string) string {
	switch v {
	case config.PreviewImageMetadataOff, config.PreviewImageMetadataBasic, config.PreviewImageMetadataFull:
		return v
	default:
		return config.PreviewImageMetadataEssentials
	}
}

func effectiveTerminalCapability(v string) string {
	switch v {
	case config.PreviewTerminalCapabilityYes, config.PreviewTerminalCapabilityNo:
		return v
	default:
		return config.PreviewTerminalCapabilityAuto
	}
}

func cycleTerminalCapability(v string) string {
	switch effectiveTerminalCapability(v) {
	case config.PreviewTerminalCapabilityAuto:
		return config.PreviewTerminalCapabilityYes
	case config.PreviewTerminalCapabilityYes:
		return config.PreviewTerminalCapabilityNo
	default:
		return config.PreviewTerminalCapabilityAuto
	}
}

func (a *App) closePreviewSettingsDialog() {
	a.model.PreviewSettingsDialog.Open = false
}

// applyPreviewSettingsDialog writes the dialog's checkbox/radio state into a.config.Preview in
// memory (takes effect on the next preview request, same as every other settings dialog) and
// persists the same 6 keys to config.toml via persistPartial.
func (a *App) applyPreviewSettingsDialog() {
	st := a.model.PreviewSettingsDialog
	sixel := effectiveTerminalCapability(st.Sixel)
	kitty := effectiveTerminalCapability(st.Kitty)
	placeholder := effectiveTerminalCapability(st.KittyPlaceholder)
	protocol := effectiveImageProtocol(st.Protocol)
	imageMetadata := effectiveImageMetadata(st.ImageMetadata)

	a.config.Preview.TerminalSixel = sixel
	a.config.Preview.TerminalKitty = kitty
	a.config.Preview.TerminalKittyPlaceholder = placeholder
	a.config.Preview.ImageProtocol = protocol
	a.config.Preview.ImageMetadata = imageMetadata
	a.config.Preview.VideoMetadata = st.VideoMetadata

	a.closePreviewSettingsDialog()
	msg := "Preview settings saved"
	urgency := ui.MessageUrgencyInfo
	patch := map[string]any{
		"preview": map[string]any{
			"terminal_sixel":             sixel,
			"terminal_kitty":             kitty,
			"terminal_kitty_placeholder": placeholder,
			"image_protocol":             protocol,
			"image_metadata":             imageMetadata,
			"video_metadata":             st.VideoMetadata,
		},
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("%s (config save failed: %v)", msg, err)
		urgency = ui.MessageUrgencyWarn
	}
	a.setTransientMessage(msg, urgency)
}

func capabilityFromDetect(detected bool) string {
	if detected {
		return config.PreviewTerminalCapabilityYes
	}
	return config.PreviewTerminalCapabilityAuto
}

// autoDetectPreviewSettingsDialog seeds the dialog's capability radios from
// preview.DetectTerminalCapabilities (F5 "Auto detect"): a best-guess snapshot from the
// environment/tmux introspection alone, ignoring any existing tri-state confirmations in config.
// Detected capabilities become "yes"; undetected stay "auto" (detection never writes "no").
// Does not touch the Protocol radio, nor the image/video metadata controls — F5 only fills in
// what can be guessed about terminal capabilities.
func (a *App) autoDetectPreviewSettingsDialog() {
	st := &a.model.PreviewSettingsDialog
	sixel, kitty, placeholder := preview.DetectTerminalCapabilities(os.Getenv)
	st.Sixel = capabilityFromDetect(sixel)
	st.SetKitty(capabilityFromDetect(kitty))
	st.SetKittyPlaceholder(capabilityFromDetect(placeholder))
}

// previewSettingsCapabilityMnemonic maps an Alt letter to the capability row it cycles. The row
// labels carry no mnemonic widget, so the letters are handled before the engine.
var previewSettingsCapabilityMnemonic = map[rune]string{
	's': dialog.PreviewSixel, 'k': dialog.PreviewKitty, 'p': dialog.PreviewPlaceholder,
}

// cyclePreviewSettingsCapability advances the capability row to its next value and focuses that
// value's radio.
func cyclePreviewSettingsCapability(st *dialog.PreviewSettingsDialogState, row string) {
	switch row {
	case dialog.PreviewSixel:
		st.Sixel = cycleTerminalCapability(st.Sixel)
		st.Focus = dialog.PreviewCapabilityID(row, st.Sixel)
	case dialog.PreviewKitty:
		st.SetKitty(cycleTerminalCapability(st.Kitty))
		st.Focus = dialog.PreviewCapabilityID(row, st.Kitty)
	case dialog.PreviewPlaceholder:
		st.SetKittyPlaceholder(cycleTerminalCapability(st.KittyPlaceholder))
		st.Focus = dialog.PreviewCapabilityID(row, st.KittyPlaceholder)
	}
}

func (a *App) handlePreviewSettingsDialogKey(event *tcell.EventKey) {
	st := &a.model.PreviewSettingsDialog
	if event.Key() == tcell.KeyF5 {
		a.autoDetectPreviewSettingsDialog()
		return
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if row, ok := previewSettingsCapabilityMnemonic[unicode.ToLower(event.Rune())]; ok {
			cyclePreviewSettingsCapability(st, row)
			return
		}
	}
	ev, ok := dlg.HandleKey(dialog.PreviewSettingsDialogView(*st), &st.Focus, event)
	switch {
	case !ok:
	case ev.ID == dlg.OK:
		a.applyPreviewSettingsDialog()
	case ev.ID == dlg.Cancel:
		a.closePreviewSettingsDialog()
	default:
		st.Apply(ev)
	}
}
