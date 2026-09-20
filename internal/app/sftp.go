package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// sftpPromptDismissSentinel is posted through the existing host-key/password open
// payloads so a cancelled wait can close its dialog on the UI goroutine.
const sftpPromptDismissSentinel = "\x00dismiss"

var errSFTPPromptCanceled = errors.New("sftp prompt canceled")

type sftpHostKeyWait struct {
	id     uint64
	prompt sftpb.HostKeyPrompt
	reply  chan sftpHostKeyReply
}

type sftpHostKeyReply struct {
	decision sftpb.HostKeyDecision
	err      error
}

type sftpPasswordWait struct {
	id     uint64
	prompt sftpb.PasswordPrompt
	reply  chan sftpPasswordReply
}

type sftpPasswordReply struct {
	password string
	err      error
}

type sftpConnectPayload struct {
	panelID int
	uri     string
	err     error
	gen     uint64
}

type sftpAppExtra struct {
	promptSem  chan struct{}
	promptID   atomic.Uint64
	connectGen [2]atomic.Uint64
}

var (
	sftpAppExtraByApp sync.Map // *App -> *sftpAppExtra
	sftpTouchConn     = sftpb.TouchConn
)

func (a *App) sftpExtra() *sftpAppExtra {
	if v, ok := sftpAppExtraByApp.Load(a); ok {
		return v.(*sftpAppExtra)
	}
	extra := &sftpAppExtra{promptSem: make(chan struct{}, 1)}
	extra.promptSem <- struct{}{}
	actual, _ := sftpAppExtraByApp.LoadOrStore(a, extra)
	return actual.(*sftpAppExtra)
}

func (a *App) lockSFTPPrompt(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-a.sftpExtra().promptSem:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) unlockSFTPPrompt() {
	a.sftpExtra().promptSem <- struct{}{}
}

func (a *App) nextSFTPPromptID() uint64 {
	return a.sftpExtra().promptID.Add(1)
}

func sftpConnectPanelIndex(panelID int) (int, bool) {
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		return 0, false
	}
	return panelID, true
}

func (a *App) nextSFTPConnectGen(panelID int) uint64 {
	idx, ok := sftpConnectPanelIndex(panelID)
	if !ok {
		idx = ui.PrimaryPanel
	}
	return a.sftpExtra().connectGen[idx].Add(1)
}

func (a *App) currentSFTPConnectGen(panelID int) uint64 {
	idx, ok := sftpConnectPanelIndex(panelID)
	if !ok {
		return 0
	}
	return a.sftpExtra().connectGen[idx].Load()
}

func (a *App) configureSFTP() error {
	cfg := a.config.SFTP
	sshConfigPath, err := sshconfig.ResolvePath(cfg.SSHConfigFile)
	if err != nil {
		return fmt.Errorf("resolve ssh config path: %w", err)
	}
	return sftpb.Configure(sftpb.Settings{
		KnownHostsFile: cfg.KnownHostsFile,
		SSHConfigFile:  sshConfigPath,
		IdleTimeout:    time.Duration(cfg.IdleTimeoutSecs) * time.Second,
		DialTimeout:    time.Duration(cfg.DialTimeoutSecs) * time.Second,
	}, sftpb.Prompts{
		HostKey:  a.promptSFTPHostKey,
		Password: a.promptSFTPPassword,
	})
}

func (a *App) openSFTPConnectDialog() {
	a.openSFTPConnectDialogForPanel(a.model.ActivePanel)
}

func (a *App) executeSFTPConnectURI(panelID int, raw string) {
	loc, err := pathloc.Parse(raw)
	if err != nil {
		a.setErrorMessage("SFTP", err)
		return
	}
	if loc.Scheme() != pathloc.SchemeSFTP {
		a.setErrorMessage("SFTP", fmt.Errorf("expected sftp:// URI, got %s", loc.Scheme()))
		return
	}
	if panelID != ui.PrimaryPanel && panelID != ui.SecondaryPanel {
		panelID = a.model.ActivePanel
	}
	a.startSFTPConnect(panelID, loc)
}

func (a *App) startSFTPConnect(panelID int, loc pathloc.Path) {
	gen := a.nextSFTPConnectGen(panelID)
	a.setTransientMessage("Connecting to "+loc.Display(48)+"...", ui.MessageUrgencyInfo)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.SFTP.DialTimeoutSecs)*time.Second)
		defer cancel()
		err := sftpTouchConn(ctx, loc)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpConnectPayload{
			panelID: panelID,
			uri:     loc.String(),
			err:     err,
			gen:     gen,
		}))
	}()
}

func (a *App) applySFTPConnect(payload sftpConnectPayload) {
	if payload.gen != a.currentSFTPConnectGen(payload.panelID) {
		return
	}
	if payload.err != nil {
		a.setErrorMessage("SFTP connect", payload.err)
		a.render()
		return
	}
	if err := a.navigatePanelToDirectory(payload.panelID, payload.uri, ""); err != nil {
		a.setErrorMessage("SFTP browse", err)
	} else {
		a.setTransientMessage("Connected to "+payload.uri, ui.MessageUrgencyInfo)
	}
	a.render()
}

func (a *App) promptSFTPHostKey(ctx context.Context, p sftpb.HostKeyPrompt) (sftpb.HostKeyDecision, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := a.lockSFTPPrompt(ctx); err != nil {
		return sftpb.HostKeyReject, err
	}
	defer a.unlockSFTPPrompt()

	reply := make(chan sftpHostKeyReply, 1)
	id := a.nextSFTPPromptID()
	a.sftp.mu.Lock()
	if old := a.sftp.hostKeyWait; old != nil {
		a.sftp.hostKeyWait = nil
		old.reply <- sftpHostKeyReply{decision: sftpb.HostKeyReject, err: errSFTPPromptCanceled}
	}
	a.sftp.hostKeyWait = &sftpHostKeyWait{id: id, prompt: p, reply: reply}
	a.sftp.mu.Unlock()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpHostKeyOpenPayload{prompt: p}))
	select {
	case r := <-reply:
		return r.decision, r.err
	case <-ctx.Done():
		if a.takeHostKeyWait(id) != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpHostKeyOpenPayload{
				prompt: sftpb.HostKeyPrompt{Host: sftpPromptDismissSentinel},
			}))
		}
		select {
		case r := <-reply:
			return r.decision, r.err
		default:
			return sftpb.HostKeyReject, ctx.Err()
		}
	}
}

func (a *App) promptSFTPPassword(ctx context.Context, p sftpb.PasswordPrompt) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := a.lockSFTPPrompt(ctx); err != nil {
		return "", err
	}
	defer a.unlockSFTPPrompt()

	reply := make(chan sftpPasswordReply, 1)
	id := a.nextSFTPPromptID()
	a.sftp.mu.Lock()
	if old := a.sftp.passwordWait; old != nil {
		a.sftp.passwordWait = nil
		old.reply <- sftpPasswordReply{err: errSFTPPromptCanceled}
	}
	a.sftp.passwordWait = &sftpPasswordWait{id: id, prompt: p, reply: reply}
	a.sftp.mu.Unlock()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpPasswordOpenPayload{prompt: p}))
	select {
	case r := <-reply:
		return r.password, r.err
	case <-ctx.Done():
		if a.takePasswordWait(id) != nil {
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpPasswordOpenPayload{
				prompt: sftpb.PasswordPrompt{Host: sftpPromptDismissSentinel},
			}))
		}
		select {
		case r := <-reply:
			return r.password, r.err
		default:
			return "", ctx.Err()
		}
	}
}

type sftpHostKeyOpenPayload struct {
	prompt sftpb.HostKeyPrompt
}

type sftpPasswordOpenPayload struct {
	prompt sftpb.PasswordPrompt
}

func (a *App) openHostKeyDialog(p sftpb.HostKeyPrompt) {
	if p.Host == sftpPromptDismissSentinel {
		a.closeHostKeyDialog()
		return
	}
	a.model.HostKeyDialog = dialog.HostKeyDialogState{
		Open:        true,
		Host:        p.Host,
		KeyType:     p.KeyType,
		Fingerprint: p.Fingerprint,
		Focus:       0,
	}
}

func (a *App) closeHostKeyDialog() {
	a.model.HostKeyDialog = dialog.HostKeyDialogState{}
}

func (a *App) takeHostKeyWait(id uint64) *sftpHostKeyWait {
	a.sftp.mu.Lock()
	defer a.sftp.mu.Unlock()
	wait := a.sftp.hostKeyWait
	if wait == nil || (id != 0 && wait.id != id) {
		return nil
	}
	a.sftp.hostKeyWait = nil
	return wait
}

func (a *App) takePasswordWait(id uint64) *sftpPasswordWait {
	a.sftp.mu.Lock()
	defer a.sftp.mu.Unlock()
	wait := a.sftp.passwordWait
	if wait == nil || (id != 0 && wait.id != id) {
		return nil
	}
	a.sftp.passwordWait = nil
	return wait
}

func (a *App) finishHostKeyDialog(decision sftpb.HostKeyDecision) {
	wait := a.takeHostKeyWait(0)
	a.closeHostKeyDialog()
	if wait != nil {
		wait.reply <- sftpHostKeyReply{decision: decision}
	}
}

func (a *App) openSFTPPasswordDialog(p sftpb.PasswordPrompt) {
	if p.Host == sftpPromptDismissSentinel {
		if a.model.FileDialog.DialogType == dialog.FileDialogSFTPPassword {
			a.dialogCtrl.CloseFileDialog()
		}
		return
	}
	label := "Password for " + p.User + "@" + p.Host
	a.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogSFTPPassword,
		Fields: []dialog.FileDialogField{{
			Label:  label,
			Value:  "",
			Cursor: 0,
		}},
		FocusedField: 0,
	}
}

func (a *App) finishSFTPPassword(password string) {
	wait := a.takePasswordWait(0)
	if wait != nil {
		wait.reply <- sftpPasswordReply{password: password}
	}
}

func (a *App) cancelSFTPPassword() {
	wait := a.takePasswordWait(0)
	if a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogSFTPPassword {
		a.dialogCtrl.CloseFileDialog()
	}
	if wait != nil {
		wait.reply <- sftpPasswordReply{err: errSFTPPromptCanceled}
	}
}

func (a *App) handleSFTPPasswordCancelKey(ev *tcell.EventKey) {
	if ev == nil {
		return
	}
	if ev.Key() == tcell.KeyEsc || dialog.AltDialogCancel(ev) {
		a.cancelSFTPPassword()
	}
}

func (a *App) executeSFTPPassword() {
	if len(a.model.FileDialog.Fields) == 0 {
		a.dialogCtrl.CloseFileDialog()
		a.finishSFTPPassword("")
		return
	}
	pw := a.model.FileDialog.Fields[0].Value
	a.dialogCtrl.CloseFileDialog()
	a.finishSFTPPassword(pw)
}

func (a *App) handleHostKeyDialogKey(ev *tcell.EventKey) bool {
	if !a.model.HostKeyDialog.Open {
		return false
	}
	d := &a.model.HostKeyDialog
	if ev.Key() == tcell.KeyRune && keymap.AltLetterModifiers(ev.Modifiers()) {
		switch ev.Rune() {
		case 'a', 'A':
			a.finishHostKeyDialog(sftpb.HostKeyTrustSession)
			a.render()
			return true
		case 's', 'S':
			a.finishHostKeyDialog(sftpb.HostKeyTrustPersist)
			a.render()
			return true
		case 'r', 'R':
			a.finishHostKeyDialog(sftpb.HostKeyReject)
			a.render()
			return true
		}
	}
	switch ev.Key() {
	case tcell.KeyEsc:
		a.finishHostKeyDialog(sftpb.HostKeyReject)
		a.render()
		return true
	case tcell.KeyLeft:
		if d.Focus > 0 {
			d.Focus--
		}
		a.render()
		return true
	case tcell.KeyRight:
		if d.Focus < 2 {
			d.Focus++
		}
		a.render()
		return true
	case tcell.KeyEnter:
		switch d.Focus {
		case 0:
			a.finishHostKeyDialog(sftpb.HostKeyTrustSession)
		case 1:
			a.finishHostKeyDialog(sftpb.HostKeyTrustPersist)
		default:
			a.finishHostKeyDialog(sftpb.HostKeyReject)
		}
		a.render()
		return true
	}
	if ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case 'a', 'A':
			a.finishHostKeyDialog(sftpb.HostKeyTrustSession)
			a.render()
			return true
		case 's', 'S':
			a.finishHostKeyDialog(sftpb.HostKeyTrustPersist)
			a.render()
			return true
		case 'r', 'R':
			a.finishHostKeyDialog(sftpb.HostKeyReject)
			a.render()
			return true
		}
	}
	return false
}
