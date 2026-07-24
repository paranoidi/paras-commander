package commands

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// WakePayload wakes PollEvent after asynchronous command-run mutations. Run() forwards it to
// ApplyWake.
type WakePayload struct {
	NotifyLog            string
	NotifyBanner         string
	NotifyUrg            ui.MessageUrgency
	RefreshBrowserPanel  bool
	ClearActiveSelection bool
	OpenOutputDialog     *dialog.CommandOutputDialogState
}

// PostWake posts p through the screen's event queue so Run's interrupt switch delivers it to
// ApplyWake on the main goroutine.
func (h *Handler) PostWake(p WakePayload) {
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(p))
}

// PostRenderWake posts a zero-value WakePayload purely to wake the event loop and trigger a
// repaint (e.g. after a CommandsList entry mutates in place).
func (h *Handler) PostRenderWake() {
	h.PostWake(WakePayload{})
}

// ApplyWake applies a delivered WakePayload's side effects on the main goroutine.
func (h *Handler) ApplyWake(p WakePayload) {
	if p.ClearActiveSelection {
		h.host.ActivePanel().ClearSelection()
	}
	if p.RefreshBrowserPanel {
		h.host.RefreshAfterUserMenuCommand()
	}
	if strings.TrimSpace(p.NotifyLog) != "" {
		h.host.SetTransientMessageBanner(p.NotifyLog, p.NotifyBanner, p.NotifyUrg)
	}
	if p.OpenOutputDialog != nil {
		h.model.CommandOutputDialog = *p.OpenOutputDialog
	}
}
