package app

import (
	"context"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// archivePasswordState holds the pending archive password prompt (at most one; prompts are
// serialized with the SFTP prompt semaphore, shared with the elevate prompt).
type archivePasswordState struct {
	mu   sync.Mutex
	wait *archivePasswordWait
}

type archivePasswordWait struct {
	id    uint64
	reply chan archivePasswordAnswer
}

// archivePasswordAnswer is the reply to the prompt: ok is false when the user skipped the archive.
type archivePasswordAnswer struct {
	password string
	ok       bool
}

// archivePasswordOpenPayload opens (or, with dismiss, closes) the archive password dialog on the
// UI goroutine.
type archivePasswordOpenPayload struct {
	path    string
	retry   bool
	dismiss bool
}

// promptArchivePassword asks the user for an encrypted archive's password (ops.ArchivePasswordFunc);
// retry says the previous answer was rejected. It blocks the calling (worker) goroutine until
// answered; Cancel or a canceled ctx answers ok=false.
func (a *App) promptArchivePassword(ctx context.Context, archivePath string, retry bool) (string, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.lockSFTPPrompt(ctx) != nil {
		return "", false
	}
	defer a.unlockSFTPPrompt()

	reply := make(chan archivePasswordAnswer, 1)
	id := a.nextSFTPPromptID()
	a.archivePw.mu.Lock()
	a.archivePw.wait = &archivePasswordWait{id: id, reply: reply}
	a.archivePw.mu.Unlock()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(archivePasswordOpenPayload{path: archivePath, retry: retry}))
	select {
	case ans := <-reply:
		return ans.password, ans.ok
	case <-ctx.Done():
		if a.takeArchivePasswordWait(id) != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(archivePasswordOpenPayload{dismiss: true}))
		}
		select {
		case ans := <-reply:
			return ans.password, ans.ok
		default:
			return "", false
		}
	}
}

func (a *App) takeArchivePasswordWait(id uint64) *archivePasswordWait {
	a.archivePw.mu.Lock()
	defer a.archivePw.mu.Unlock()
	w := a.archivePw.wait
	if w == nil || (id != 0 && w.id != id) {
		return nil
	}
	a.archivePw.wait = nil
	return w
}

func (a *App) applyArchivePasswordOpen(p archivePasswordOpenPayload) {
	if p.dismiss {
		a.model.ArchivePasswordDialog = dialog.ArchivePasswordDialogState{}
		return
	}
	a.model.ArchivePasswordDialog = dialog.ArchivePasswordDialogState{Open: true, Path: p.path, BadPassword: p.retry}
}

// finishArchivePassword closes the dialog and answers the waiting extract; ok=false skips the
// archive.
func (a *App) finishArchivePassword(ok bool) {
	ans := archivePasswordAnswer{password: a.model.ArchivePasswordDialog.Password.Value, ok: ok}
	a.model.ArchivePasswordDialog = dialog.ArchivePasswordDialogState{}
	if w := a.takeArchivePasswordWait(0); w != nil {
		w.reply <- ans
	}
}

// applyArchivePasswordEvent acts on a widget event of the prompt (key or click): Enter in the
// input and OK answer with the password, Cancel skips the archive.
func (a *App) applyArchivePasswordEvent(e dlg.Event) {
	switch {
	case e.Kind == dlg.EventCaret:
		a.model.ArchivePasswordDialog.Password.SetCursor(e.Index)
	case e.ID != "":
		a.finishArchivePassword(e.ID == dialog.ArchivePasswordInput || e.ID == dlg.OK)
	}
}

func (a *App) handleArchivePasswordDialogKey(ev *tcell.EventKey) {
	d := &a.model.ArchivePasswordDialog
	defer a.render()
	if e, ok := dlg.HandleKey(dialog.ArchivePasswordView(*d), &d.Focus, ev); ok {
		a.applyArchivePasswordEvent(e)
		return
	}
	if d.FocusOnInput() {
		dialog.HandleFileDialogFieldKey(ev, &d.Password, a.keys.DialogInput, nil)
	}
}
