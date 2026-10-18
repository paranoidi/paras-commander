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
	NotifyLog           string
	NotifyBanner        string
	NotifyUrg           ui.MessageUrgency
	RefreshBrowserPanel bool
	OpenOutputDialog    *dialog.CommandOutputDialogState
	// OutputDialogLive refreshes the running output dialog's body with partial output.
	OutputDialogLive *OutputDialogLive
	// OutputDialogStartedRunID clears the output dialog's Queued state when it shows this run.
	OutputDialogStartedRunID string

	// TerminalPanelShow / TerminalPanelHide / TerminalPanelDrawer / TerminalPanelClearDrawer
	// are applied on the event loop so ViewMode and TerminalPanel are never written from a
	// batch goroutine.
	TerminalPanelShow        bool
	TerminalPanelHide        bool
	TerminalPanelDrawer      ui.TerminalDrawer
	TerminalPanelClearDrawer bool

	applied chan struct{}
}

// OutputDialogLive is a snapshot of a run's partial output for the open running dialog.
type OutputDialogLive struct {
	RunID          string
	Stdout, Stderr string
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
	cur := &h.model.CommandOutputDialog
	if p.OutputDialogStartedRunID != "" && cur.Open && cur.RunID == p.OutputDialogStartedRunID {
		cur.Queued = false
	}
	if l := p.OutputDialogLive; l != nil && cur.Open && cur.Running && cur.RunID == l.RunID {
		cur.Lines = outputDialogLines(l.Stdout, l.Stderr)
	}
	if p.OpenOutputDialog != nil {
		if cur.Open && cur.Running && cur.RunID == p.OpenOutputDialog.RunID {
			// The user is watching this run: show the output instead of a banner/refresh.
			if cur.Cancel != nil {
				cur.Cancel() // release the run context
			}
			scroll, follow := cur.Scroll, cur.Follow
			*cur = *p.OpenOutputDialog
			cur.Scroll, cur.Follow = scroll, follow
			if len(cur.Paths) > 0 {
				if follow {
					cur.Scroll = 0
				}
				cur.Follow = false
				h.ensurePathVisible(cur)
			}
			p.NotifyLog, p.NotifyBanner, p.RefreshBrowserPanel = "", "", false
		}
	}
	if p.RefreshBrowserPanel {
		h.host.RefreshAfterBackgroundCommand()
	}
	if strings.TrimSpace(p.NotifyLog) != "" {
		h.host.SetTransientMessageBanner(p.NotifyLog, p.NotifyBanner, p.NotifyUrg)
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
