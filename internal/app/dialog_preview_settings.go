package app

import (
	"fmt"
	"os"
	"strconv"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/dialogform"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Preview settings dialog (M-F3) handlers. Mutates a.config.Preview in place (settings-dialog
// pattern, see CLAUDE.md App package layout), so it stays in internal/app rather than
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
		Focus:            0,
	}
}

func effectiveImageProtocol(v string) string {
	switch v {
	case config.PreviewImageProtocolSixel, config.PreviewImageProtocolKitty:
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

func capabilityRadioFocus(first int, v string) int {
	switch effectiveTerminalCapability(v) {
	case config.PreviewTerminalCapabilityAuto:
		return first
	case config.PreviewTerminalCapabilityYes:
		return first + 1
	default:
		return first + 2
	}
}

func (a *App) closePreviewSettingsDialog() {
	a.model.PreviewSettingsDialog.Open = false
}

// setKittyCapability writes the Kitty radio and, since Unicode-placeholder display requires
// Kitty protocol support, drops a "yes" placeholder whenever Kitty is not "yes".
func setKittyCapability(st *dialog.PreviewSettingsDialogState, v string) {
	st.Kitty = effectiveTerminalCapability(v)
	if st.Kitty != config.PreviewTerminalCapabilityYes && st.KittyPlaceholder == config.PreviewTerminalCapabilityYes {
		st.KittyPlaceholder = st.Kitty
	}
}

// setKittyPlaceholderCapability writes the placeholder radio and, since it implies Kitty
// protocol support, forces Kitty to "yes" whenever placeholder is "yes".
func setKittyPlaceholderCapability(st *dialog.PreviewSettingsDialogState, v string) {
	st.KittyPlaceholder = effectiveTerminalCapability(v)
	if st.KittyPlaceholder == config.PreviewTerminalCapabilityYes {
		st.Kitty = config.PreviewTerminalCapabilityYes
	}
}

func cycleKittyCapability(st *dialog.PreviewSettingsDialogState) {
	setKittyCapability(st, cycleTerminalCapability(st.Kitty))
}

func cycleKittyPlaceholderCapability(st *dialog.PreviewSettingsDialogState) {
	setKittyPlaceholderCapability(st, cycleTerminalCapability(st.KittyPlaceholder))
}

// applyPreviewSettingsDialog writes the dialog's checkbox/radio state into a.config.Preview in
// memory (takes effect on the next preview request, same as every other settings dialog) and
// persists the same 6 keys to config.toml via config.PatchPreviewKeys. Unlike other settings
// dialogs (which use the whole-file WriteMergedPartial/persistPartial merge), this one uses the
// narrower key-scoped patcher so it doesn't strip comments/formatting from the rest of
// config.toml — see internal/config/patch.go.
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
	if !a.paths.CanPersist() {
		msg = fmt.Sprintf("%s (config save failed: no config path)", msg)
		urgency = ui.MessageUrgencyWarn
		a.setTransientMessage(msg, urgency)
		return
	}
	values := map[string]string{
		"terminal_sixel":             strconv.Quote(sixel),
		"terminal_kitty":             strconv.Quote(kitty),
		"terminal_kitty_placeholder": strconv.Quote(placeholder),
		"image_protocol":             strconv.Quote(protocol),
		"image_metadata":             strconv.Quote(imageMetadata),
		"video_metadata":             strconv.FormatBool(st.VideoMetadata),
	}
	if err := config.PatchPreviewKeysForPaths(a.paths, values); err != nil {
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
	setKittyCapability(st, capabilityFromDetect(kitty))
	setKittyPlaceholderCapability(st, capabilityFromDetect(placeholder))
}

func (a *App) handlePreviewSettingsDialogKey(event *tcell.EventKey) {
	st := &a.model.PreviewSettingsDialog
	if event.Key() == tcell.KeyF5 {
		a.autoDetectPreviewSettingsDialog()
		return
	}
	form := dialog.PreviewSettingsDialogForm()
	capabilityRadios := dialog.PreviewSettingsDialogCapabilityRadios()
	protocolRadios := dialog.PreviewSettingsDialogProtocolRadios()
	metadataRadios := dialog.PreviewSettingsDialogImageMetadataRadios()
	const (
		sixelFocusFirst       = 0
		kittyFocusFirst       = 3
		placeholderFocusFirst = 6
		protocolFocusFirst    = 9
		metadataFocusFirst    = 12
		videoFocus            = 16
	)
	a.handleLinearFormDialogKey(event, form, dialogform.Handlers{
		Focus:              &st.Focus,
		OnApply:            a.applyPreviewSettingsDialog,
		OnCancel:           a.closePreviewSettingsDialog,
		AllowPlainOKCancel: true,
		OnMnemonic: func(r rune) bool {
			for i, radio := range protocolRadios {
				if unicode.ToLower(r) == unicode.ToLower(radio.Shortcut) {
					st.Protocol = radio.Value
					st.Focus = protocolFocusFirst + i
					return true
				}
			}
			for i, radio := range metadataRadios {
				if unicode.ToLower(r) == unicode.ToLower(radio.Shortcut) {
					st.ImageMetadata = radio.Value
					st.Focus = metadataFocusFirst + i
					return true
				}
			}
			switch r {
			case 's', 'S':
				st.Sixel = cycleTerminalCapability(st.Sixel)
				st.Focus = capabilityRadioFocus(sixelFocusFirst, st.Sixel)
			case 'k', 'K':
				cycleKittyCapability(st)
				st.Focus = capabilityRadioFocus(kittyFocusFirst, st.Kitty)
			case 'p', 'P':
				cycleKittyPlaceholderCapability(st)
				st.Focus = capabilityRadioFocus(placeholderFocusFirst, st.KittyPlaceholder)
			case 'v', 'V':
				st.VideoMetadata = !st.VideoMetadata
				st.Focus = videoFocus
			default:
				return false
			}
			return true
		},
		OnSpace: func(focus int) bool {
			switch {
			case focus >= sixelFocusFirst && focus < kittyFocusFirst:
				st.Sixel = capabilityRadios[focus-sixelFocusFirst].Value
			case focus >= kittyFocusFirst && focus < placeholderFocusFirst:
				setKittyCapability(st, capabilityRadios[focus-kittyFocusFirst].Value)
			case focus >= placeholderFocusFirst && focus < protocolFocusFirst:
				setKittyPlaceholderCapability(st, capabilityRadios[focus-placeholderFocusFirst].Value)
			case focus >= protocolFocusFirst && focus < protocolFocusFirst+len(protocolRadios):
				st.Protocol = protocolRadios[focus-protocolFocusFirst].Value
			case focus >= metadataFocusFirst && focus < metadataFocusFirst+len(metadataRadios):
				st.ImageMetadata = metadataRadios[focus-metadataFocusFirst].Value
			case focus == videoFocus:
				st.VideoMetadata = !st.VideoMetadata
			case focus == form.OKIndex():
				a.applyPreviewSettingsDialog()
			case focus == form.CancelIndex():
				a.closePreviewSettingsDialog()
			default:
				return false
			}
			return true
		},
	})
}
