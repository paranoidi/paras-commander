package commands

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
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

	// TerminalPanelShow / TerminalPanelHide / TerminalPanelDrawer / TerminalPanelClearDrawer
	// are applied on the event loop so ViewMode and TerminalPanel are never written from a
	// batch goroutine.
	TerminalPanelShow        bool
	TerminalPanelHide        bool
	TerminalPanelDrawer      ui.TerminalDrawer
	TerminalPanelClearDrawer bool

	applied chan struct{}
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
		h.host.RefreshAfterBackgroundCommand()
	}
	if strings.TrimSpace(p.NotifyLog) != "" {
		h.host.SetTransientMessageBanner(p.NotifyLog, p.NotifyBanner, p.NotifyUrg)
	}
	if p.OpenOutputDialog != nil {
		h.model.CommandOutputDialog = *p.OpenOutputDialog
	}
	h.applyTerminalPanelWake(p)
	if p.applied != nil {
		close(p.applied)
	}
}

func (h *Handler) applyTerminalPanelWake(p WakePayload) {
	needLock := p.TerminalPanelShow || p.TerminalPanelHide || p.TerminalPanelClearDrawer || p.TerminalPanelDrawer != nil
	if !needLock {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	tp := &h.model.TerminalPanel
	if p.TerminalPanelShow {
		h.model.ViewMode = ui.ViewBrowser
		if tp.Rows < config.MinShellTerminalPanelHeight {
			tp.Rows = config.DefaultShellTerminalPanelHeight
		}
		tp.Visible = true
		tp.Focused = true
	}
	if p.TerminalPanelHide {
		tp.Visible = false
		tp.Focused = false
		tp.Drawer = nil
	}
	if p.TerminalPanelClearDrawer {
		tp.Drawer = nil
	}
	if p.TerminalPanelDrawer != nil {
		tp.Drawer = p.TerminalPanelDrawer
	}
}

// postPanelWakeAndWait posts p and blocks until ApplyWake runs (or ctx is done). Used so a
// batch goroutine can request ViewMode/TerminalPanel changes without writing those fields
// itself. When ctx is already canceled the wake is still posted (the event loop applies it)
// but this returns immediately so shutdown cannot hang.
func (h *Handler) postPanelWakeAndWait(ctx context.Context, p WakePayload) {
	applied := make(chan struct{})
	p.applied = applied
	if err := h.screen.PostEvent(tcell.NewEventInterrupt(p)); err != nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return
	}
	select {
	case <-applied:
	case <-ctx.Done():
	}
}
