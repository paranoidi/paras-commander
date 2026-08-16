package app

import (
	"os"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/dialogform"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Image terminal-capabilities dialog (M-F3) handlers. Mutates a.config.Preview in place
// (settings-dialog pattern, see CLAUDE.md App package layout), so it stays in internal/app
// rather than apphandler/dialog.

func (a *App) openImageCapabilityDialog() {
	a.clearTransientMessage()
	p := a.config.Preview
	a.model.ImageCapabilityDialog = dialog.ImageCapabilityDialogState{
		Open:                      true,
		SixelSupported:            p.TerminalSixel == config.PreviewTerminalCapabilityYes,
		KittySupported:            p.TerminalKitty == config.PreviewTerminalCapabilityYes,
		KittyPlaceholderSupported: p.TerminalKittyPlaceholder == config.PreviewTerminalCapabilityYes,
		Protocol:                  effectiveImageProtocol(p.ImageProtocol),
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

func (a *App) closeImageCapabilityDialog() {
	a.model.ImageCapabilityDialog.Open = false
}

// toggleKittySupported flips Kitty support and, since Unicode-placeholder display requires
// Kitty protocol support, clears the (now-inconsistent) placeholder checkbox whenever Kitty
// support is unchecked.
func toggleKittySupported(st *dialog.ImageCapabilityDialogState) {
	st.KittySupported = !st.KittySupported
	if !st.KittySupported {
		st.KittyPlaceholderSupported = false
	}
}

// toggleKittyPlaceholderSupported flips placeholder support and, since it implies Kitty
// protocol support, checks the Kitty checkbox whenever placeholder support is checked.
func toggleKittyPlaceholderSupported(st *dialog.ImageCapabilityDialogState) {
	st.KittyPlaceholderSupported = !st.KittyPlaceholderSupported
	if st.KittyPlaceholderSupported {
		st.KittySupported = true
	}
}

// applyImageCapabilityDialog writes the dialog's checkbox/radio state into a.config.Preview in
// memory (takes effect on the next preview request, same as every other settings dialog) and
// persists the same 4 keys to config.toml via config.PatchPreviewTerminalKeys. Unlike other
// settings dialogs (which use the whole-file WriteMergedPartial/persistPartial merge), this one
// uses the narrower key-scoped patcher so it doesn't strip comments/formatting from the rest of
// config.toml — see internal/config/patch.go.
func (a *App) applyImageCapabilityDialog() {
	st := a.model.ImageCapabilityDialog
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

	a.config.Preview.TerminalSixel = sixel
	a.config.Preview.TerminalKitty = kitty
	a.config.Preview.TerminalKittyPlaceholder = placeholder
	a.config.Preview.ImageProtocol = protocol

	a.closeImageCapabilityDialog()
	msg := "Image terminal capabilities saved"
	if a.paths.ConfigFile == "" {
		a.setTransientMessage(msg, ui.MessageUrgencyInfo)
		return
	}
	if err := config.PatchPreviewTerminalKeys(a.paths.ConfigFile, sixel, kitty, placeholder, protocol); err != nil {
		a.setErrorMessage("Save image terminal capabilities", err)
		return
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}

// autoDetectImageCapabilityDialog seeds the dialog's checkboxes from preview.DetectTerminalCapabilities
// (F5 "Auto detect"): a best-guess snapshot from the environment/tmux introspection alone,
// ignoring any existing tri-state confirmations in config. Does not touch the Protocol radio —
// F5 only fills in what can be guessed about capabilities, leaving the active-protocol choice to
// the user (or to OK, which re-resolves "Auto" from the freshly guessed checkboxes anyway).
func (a *App) autoDetectImageCapabilityDialog() {
	st := &a.model.ImageCapabilityDialog
	st.SixelSupported, st.KittySupported, st.KittyPlaceholderSupported = preview.DetectTerminalCapabilities(os.Getenv)
}

func (a *App) handleImageCapabilityDialogKey(event *tcell.EventKey) {
	st := &a.model.ImageCapabilityDialog
	if event.Key() == tcell.KeyF5 {
		a.autoDetectImageCapabilityDialog()
		return
	}
	form := dialog.ImageCapabilityDialogForm()
	radios := dialog.ImageCapabilityDialogRadios()
	a.handleLinearFormDialogKey(event, form, dialogform.Handlers{
		Focus:              &st.Focus,
		OnApply:            a.applyImageCapabilityDialog,
		OnCancel:           a.closeImageCapabilityDialog,
		AllowPlainOKCancel: true,
		OnMnemonic: func(r rune) bool {
			for i, radio := range radios {
				if unicode.ToLower(r) == unicode.ToLower(radio.Shortcut) {
					st.Protocol = radio.Protocol
					st.Focus = 3 + i
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
			default:
				return false
			}
			return true
		},
		OnSpace: func(focus int) bool {
			switch focus {
			case 0:
				st.SixelSupported = !st.SixelSupported
			case 1:
				toggleKittySupported(st)
			case 2:
				toggleKittyPlaceholderSupported(st)
			case 3, 4, 5:
				st.Protocol = radios[focus-3].Protocol
			case form.OKIndex():
				a.applyImageCapabilityDialog()
			case form.CancelIndex():
				a.closeImageCapabilityDialog()
			default:
				return false
			}
			return true
		},
	})
}
