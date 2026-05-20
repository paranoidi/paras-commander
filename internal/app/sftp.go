package app

import (
	"context"
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type sftpHostKeyWait struct {
	prompt sftpb.HostKeyPrompt
	reply  chan sftpb.HostKeyDecision
}

type sftpPasswordWait struct {
	prompt sftpb.PasswordPrompt
	reply  chan string
}

type sftpConnectPayload struct {
	panelID int
	uri     string
	err     error
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
		a.setErrorMessage("SFTP link", err)
		return
	}
	if loc.Scheme() != pathloc.SchemeSFTP {
		a.setErrorMessage("SFTP link", fmt.Errorf("expected sftp:// URI, got %s", loc.Scheme()))
		return
	}
	if panelID != ui.LeftPanel && panelID != ui.RightPanel {
		panelID = a.model.ActivePanel
	}
	a.startSFTPConnect(panelID, loc)
}

func (a *App) startSFTPConnect(panelID int, loc pathloc.Path) {
	a.setTransientMessage("Connecting to "+loc.Display(48)+"...", ui.MessageUrgencyInfo)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.config.SFTP.DialTimeoutSecs)*time.Second)
		defer cancel()
		err := sftpb.TouchConn(ctx, loc)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpConnectPayload{
			panelID: panelID,
			uri:     loc.String(),
			err:     err,
		}))
	}()
}

func (a *App) applySFTPConnect(payload sftpConnectPayload) {
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
	_ = ctx // interactive approval is not cancelled by SSH dial context (may be nil from host key callback)
	reply := make(chan sftpb.HostKeyDecision, 1)
	a.sftpMu.Lock()
	a.sftpHostKeyWait = &sftpHostKeyWait{prompt: p, reply: reply}
	a.sftpMu.Unlock()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpHostKeyOpenPayload{prompt: p}))
	d := <-reply
	return d, nil
}

func (a *App) promptSFTPPassword(ctx context.Context, p sftpb.PasswordPrompt) (string, error) {
	_ = ctx // same as host key: wait for user input on the main thread
	reply := make(chan string, 1)
	a.sftpMu.Lock()
	a.sftpPasswordWait = &sftpPasswordWait{prompt: p, reply: reply}
	a.sftpMu.Unlock()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(sftpPasswordOpenPayload{prompt: p}))
	return <-reply, nil
}

type sftpHostKeyOpenPayload struct {
	prompt sftpb.HostKeyPrompt
}

type sftpPasswordOpenPayload struct {
	prompt sftpb.PasswordPrompt
}

func (a *App) openHostKeyDialog(p sftpb.HostKeyPrompt) {
	a.model.HostKeyDialog = ui.HostKeyDialogState{
		Open:        true,
		Host:        p.Host,
		KeyType:     p.KeyType,
		Fingerprint: p.Fingerprint,
		Focus:       0,
	}
}

func (a *App) closeHostKeyDialog() {
	a.model.HostKeyDialog = ui.HostKeyDialogState{}
}

func (a *App) finishHostKeyDialog(decision sftpb.HostKeyDecision) {
	a.sftpMu.Lock()
	wait := a.sftpHostKeyWait
	a.sftpHostKeyWait = nil
	a.sftpMu.Unlock()
	a.closeHostKeyDialog()
	if wait != nil {
		wait.reply <- decision
	}
}

func (a *App) openSFTPPasswordDialog(p sftpb.PasswordPrompt) {
	label := "Password for " + p.User + "@" + p.Host
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogSFTPPassword,
		Fields: []ui.FileDialogField{{
			Label:  label,
			Value:  "",
			Cursor: 0,
		}},
		FocusedField: 0,
	}
}

func (a *App) finishSFTPPassword(password string) {
	a.sftpMu.Lock()
	wait := a.sftpPasswordWait
	a.sftpPasswordWait = nil
	a.sftpMu.Unlock()
	if wait != nil {
		wait.reply <- password
	}
}

func (a *App) executeSFTPPassword() {
	if len(a.model.FileDialog.Fields) == 0 {
		a.closeFileDialog()
		a.finishSFTPPassword("")
		return
	}
	pw := a.model.FileDialog.Fields[0].Value
	a.closeFileDialog()
	a.finishSFTPPassword(pw)
}

func (a *App) handleHostKeyDialogKey(ev *tcell.EventKey) bool {
	if !a.model.HostKeyDialog.Open {
		return false
	}
	d := &a.model.HostKeyDialog
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
