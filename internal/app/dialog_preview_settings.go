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
		Open:                      true,
		SixelSupported:            p.TerminalSixel == config.PreviewTerminalCapabilityYes,
		KittySupported:            p.TerminalKitty == config.PreviewTerminalCapabilityYes,
		KittyPlaceholderSupported: p.TerminalKittyPlaceholder == config.PreviewTerminalCapabilityYes,
		Protocol:                  effectiveImageProtocol(p.ImageProtocol),
		ImageMetadata:             effectiveImageMetadata(p.ImageMetadata),
		VideoMetadata:             p.VideoMetadata,
		Focus:                     0,
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

func (a *App) closePreviewSettingsDialog() {
	a.model.PreviewSettingsDialog.Open = false
}

// toggleKittySupported flips Kitty support and, since Unicode-placeholder display requires
// Kitty protocol support, clears the (now-inconsistent) placeholder checkbox whenever Kitty
// support is unchecked.
func toggleKittySupported(st *dialog.PreviewSettingsDialogState) {
	st.KittySupported = !st.KittySupported
	if !st.KittySupported {
		st.KittyPlaceholderSupported = false
	}
}

// toggleKittyPlaceholderSupported flips placeholder support and, since it implies Kitty
// protocol support, checks the Kitty checkbox whenever placeholder support is checked.
func toggleKittyPlaceholderSupported(st *dialog.PreviewSettingsDialogState) {
	st.KittyPlaceholderSupported = !st.KittyPlaceholderSupported
	if st.KittyPlaceholderSupported {
		st.KittySupported = true
	}
}

// applyPreviewSettingsDialog writes the dialog's checkbox/radio state into a.config.Preview in
// memory (takes effect on the next preview request, same as every other settings dialog) and
// persists the same 6 keys to config.toml via config.PatchPreviewKeys. Unlike other settings
// dialogs (which use the whole-file WriteMergedPartial/persistPartial merge), this one uses the
// narrower key-scoped patcher so it doesn't strip comments/formatting from the rest of
// config.toml — see internal/config/patch.go.
func (a *App) applyPreviewSettingsDialog() {
	st := a.model.PreviewSettingsDialog
	tri := func(checked bool) string {
		if checked {
			return config.PreviewTerminalCapabilityYes
		}
		return config.PreviewTerminalCapabilityAuto
	}
	sixel := tri(st.SixelSupported)
	kitty := tri(st.KittySupported)
	placeholder := tri(st.KittyPlaceholderSupported)
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

// autoDetectPreviewSettingsDialog seeds the dialog's checkboxes from
// preview.DetectTerminalCapabilities (F5 "Auto detect"): a best-guess snapshot from the
// environment/tmux introspection alone, ignoring any existing tri-state confirmations in config.
// Does not touch the Protocol radio, nor the image/video metadata controls — F5 only fills in
// what can be guessed about terminal capabilities.
func (a *App) autoDetectPreviewSettingsDialog() {
	st := &a.model.PreviewSettingsDialog
	st.SixelSupported, st.KittySupported, st.KittyPlaceholderSupported = preview.DetectTerminalCapabilities(os.Getenv)
}

func (a *App) handlePreviewSettingsDialogKey(event *tcell.EventKey) {
	st := &a.model.PreviewSettingsDialog
	if event.Key() == tcell.KeyF5 {
		a.autoDetectPreviewSettingsDialog()
		return
	}
	form := dialog.PreviewSettingsDialogForm()
	protocolRadios := dialog.PreviewSettingsDialogProtocolRadios()
	metadataRadios := dialog.PreviewSettingsDialogImageMetadataRadios()
	const (
		protocolFocusFirst = 3
		metadataFocusFirst = 6
		videoFocus         = 10
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
				st.SixelSupported = !st.SixelSupported
				st.Focus = 0
			case 'k', 'K':
				toggleKittySupported(st)
				st.Focus = 1
			case 'p', 'P':
				toggleKittyPlaceholderSupported(st)
				st.Focus = 2
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
			case focus == 0:
				st.SixelSupported = !st.SixelSupported
			case focus == 1:
				toggleKittySupported(st)
			case focus == 2:
				toggleKittyPlaceholderSupported(st)
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
