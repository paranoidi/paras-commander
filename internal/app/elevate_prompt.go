package app

import (
	"context"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
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
	// Land on the password field, or on Elevate when there is none.
	a.model.ElevateDialog.Focus = 0
}

func (a *App) finishElevate(d ops.PermissionDecision) {
	ans := ops.PermissionAnswer{Decision: d, Password: a.model.ElevateDialog.Password.Value}
	a.model.ElevateDialog = dialog.ElevateDialogState{}
	if w := a.takeElevateWait(0); w != nil {
		w.reply <- ans
	}
}

// elevateDecisionAt maps a button index (0=Elevate, 1=Skip, 2=Cancel) to its decision.
func elevateDecisionAt(i int) ops.PermissionDecision {
	switch i {
	case 0:
		return ops.PermissionElevate
	case 1:
		return ops.PermissionSkip
	default:
		return ops.PermissionAbort
	}
}

func (a *App) handleElevateDialogKey(ev *tcell.EventKey) {
	d := &a.model.ElevateDialog
	defer a.render()
	if ev.Key() == tcell.KeyRune && keymap.AltLetterModifiers(ev.Modifiers()) {
		switch ev.Rune() {
		case 'e', 'E':
			a.finishElevate(ops.PermissionElevate)
		case 's', 'S':
			a.finishElevate(ops.PermissionSkip)
		case 'c', 'C':
			a.finishElevate(ops.PermissionAbort)
		}
		return
	}
	base := d.ButtonBase()
	last := base + dialog.ElevateButtons - 1
	switch ev.Key() {
	case tcell.KeyEsc:
		a.finishElevate(ops.PermissionAbort)
	case tcell.KeyEnter:
		if d.FocusOnInput() {
			a.finishElevate(ops.PermissionElevate)
			return
		}
		a.finishElevate(elevateDecisionAt(d.Focus - base))
	case tcell.KeyUp:
		if d.HasInput() && d.Focus >= base {
			d.Focus = 0 // up from the buttons goes back to the field
		}
	case tcell.KeyDown:
		if d.FocusOnInput() {
			d.Focus = base
		}
	case tcell.KeyTab, tcell.KeyBacktab:
		if d.HasInput() {
			if d.FocusOnInput() {
				d.Focus = base
			} else {
				d.Focus = 0
			}
		}
	case tcell.KeyLeft:
		if d.FocusOnInput() {
			d.Password.MoveCursor(-1)
		} else if d.Focus > base {
			d.Focus--
		}
	case tcell.KeyRight:
		if d.FocusOnInput() {
			d.Password.MoveCursor(1)
		} else if d.Focus < last {
			d.Focus++
		}
	case tcell.KeyHome:
		if d.FocusOnInput() {
			d.Password.MoveCursorStart()
		}
	case tcell.KeyEnd:
		if d.FocusOnInput() {
			d.Password.MoveCursorEnd()
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if d.FocusOnInput() {
			d.Password.Backspace()
		}
	case tcell.KeyDelete:
		if d.FocusOnInput() {
			d.Password.Delete()
		}
	case tcell.KeyRune:
		if d.FocusOnInput() && ev.Modifiers()&(tcell.ModAlt|tcell.ModCtrl) == 0 {
			d.Password.InsertRune(ev.Rune())
		}
	}
}
