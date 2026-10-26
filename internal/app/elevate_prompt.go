package app

import (
	"context"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// elevateState holds the pending permission prompt (at most one; prompts are serialized with the
// SFTP prompt semaphore).
type elevateState struct {
	mu   sync.Mutex
	wait *elevateWait
}

type elevateWait struct {
	id    uint64
	reply chan ops.PermissionAnswer
}

// elevateOpenPayload opens (or, with dismiss, closes) the elevate dialog on the UI goroutine.
type elevateOpenPayload struct {
	req     ops.PermissionRequest
	dismiss bool
}

// promptElevate is the single entry point that asks the user whether a permission-denied file
// step may be retried as root. It blocks the calling (worker) goroutine until answered; a
// canceled ctx dismisses the dialog and answers Abort.
func (a *App) promptElevate(ctx context.Context, req ops.PermissionRequest) ops.PermissionAnswer {
	abort := ops.PermissionAnswer{Decision: ops.PermissionAbort}
	if ctx == nil {
		ctx = context.Background()
	}
	if a.lockSFTPPrompt(ctx) != nil {
		return abort
	}
	defer a.unlockSFTPPrompt()

	reply := make(chan ops.PermissionAnswer, 1)
	id := a.nextSFTPPromptID()
	a.elevate.mu.Lock()
	a.elevate.wait = &elevateWait{id: id, reply: reply}
	a.elevate.mu.Unlock()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(elevateOpenPayload{req: req}))
	select {
	case ans := <-reply:
		return ans
	case <-ctx.Done():
		if a.takeElevateWait(id) != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(elevateOpenPayload{dismiss: true}))
		}
		select {
		case ans := <-reply:
			return ans
		default:
			return abort
		}
	}
}

func (a *App) takeElevateWait(id uint64) *elevateWait {
	a.elevate.mu.Lock()
	defer a.elevate.mu.Unlock()
	w := a.elevate.wait
	if w == nil || (id != 0 && w.id != id) {
		return nil
	}
	a.elevate.wait = nil
	return w
}

func (a *App) applyElevateOpen(p elevateOpenPayload) {
	if p.dismiss {
		a.model.ElevateDialog = dialog.ElevateDialogState{}
		return
	}
	a.model.ElevateDialog = dialog.ElevateDialogState{
		Open:        true,
		Op:          p.req.Op,
		Path:        p.req.Path,
		NoPassword:  p.req.NoPassword,
		BadPassword: p.req.BadPassword,
	}
}

func (a *App) finishElevate(d ops.PermissionDecision) {
	ans := ops.PermissionAnswer{Decision: d, Password: a.model.ElevateDialog.Password.Value}
	a.model.ElevateDialog = dialog.ElevateDialogState{}
	if w := a.takeElevateWait(0); w != nil {
		w.reply <- ans
	}
}

// elevateDecision maps an activated widget to its decision: Enter in the password field elevates.
func elevateDecision(id dlg.ID) ops.PermissionDecision {
	switch id {
	case dialog.ElevatePassword, dialog.ElevateRetry:
		return ops.PermissionElevate
	case dialog.ElevateSkip:
		return ops.PermissionSkip
	default:
		return ops.PermissionAbort
	}
}

// applyElevateEvent acts on a widget event of the elevation prompt (key or click).
func (a *App) applyElevateEvent(e dlg.Event) {
	switch {
	case e.Kind == dlg.EventCaret:
		a.model.ElevateDialog.Password.SetCursor(e.Index)
	case e.ID != "":
		a.finishElevate(elevateDecision(e.ID))
	}
}

func (a *App) handleElevateDialogKey(ev *tcell.EventKey) {
	d := &a.model.ElevateDialog
	defer a.render()
	if e, ok := dlg.HandleKey(dialog.ElevateView(*d), &d.Focus, ev); ok {
		a.applyElevateEvent(e)
		return
	}
	if !d.FocusOnInput() {
		return
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		d.Password.MoveCursor(-1)
	case tcell.KeyRight:
		d.Password.MoveCursor(1)
	case tcell.KeyHome:
		d.Password.MoveCursorStart()
	case tcell.KeyEnd:
		d.Password.MoveCursorEnd()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		d.Password.Backspace()
	case tcell.KeyDelete:
		d.Password.Delete()
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) == 0 {
			d.Password.InsertRune(ev.Rune())
		}
	}
}
