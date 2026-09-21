package app

import (
	"context"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
)

func TestHandleFileDialogKeyEscAndAltCCancelSFTPPasswordWaiter(t *testing.T) {
	t.Parallel()
	keys := []struct {
		name string
		ev   *tcell.EventKey
	}{
		{name: "Esc", ev: tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)},
		{name: "Alt+C", ev: tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt)},
	}
	for _, tt := range keys {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := testAppMinimal(t)
			result := make(chan passwordPromptResult, 1)
			go func() {
				pw, err := app.promptSFTPPassword(context.Background(), sftpb.PasswordPrompt{
					User: "alice",
					Host: "example.com",
				})
				result <- passwordPromptResult{password: pw, err: err}
			}()
			waitSFTPPasswordWait(t, app)
			app.openSFTPPasswordDialog(sftpb.PasswordPrompt{User: "alice", Host: "example.com"})
			app.dialogCtrl.HandleFileDialogKey(tt.ev)
			assertSFTPPasswordCanceled(t, app, result)
		})
	}
}

func TestExecuteSFTPPasswordStillCompletesWaiter(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	result := make(chan passwordPromptResult, 1)
	go func() {
		pw, err := app.promptSFTPPassword(context.Background(), sftpb.PasswordPrompt{
			User: "alice",
			Host: "example.com",
		})
		result <- passwordPromptResult{password: pw, err: err}
	}()
	waitSFTPPasswordWait(t, app)
	app.openSFTPPasswordDialog(sftpb.PasswordPrompt{User: "alice", Host: "example.com"})
	app.model.FileDialog.Fields[0].Value = "secret"
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatalf("OK must complete the waiter: %v", got.err)
		}
		if got.password != "secret" {
			t.Fatalf("password = %q, want secret", got.password)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("password waiter stranded after OK")
	}
	if app.model.FileDialog.Open {
		t.Fatal("password dialog still open after OK")
	}
}

func TestCloseFileDialogCancelsSFTPPasswordWaiter(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	result := make(chan passwordPromptResult, 1)
	go func() {
		pw, err := app.promptSFTPPassword(context.Background(), sftpb.PasswordPrompt{
			User: "alice",
			Host: "example.com",
		})
		result <- passwordPromptResult{password: pw, err: err}
	}()
	waitSFTPPasswordWait(t, app)
	app.openSFTPPasswordDialog(sftpb.PasswordPrompt{User: "alice", Host: "example.com"})
	app.dialogCtrl.CloseFileDialog()
	assertSFTPPasswordCanceled(t, app, result)
}

func assertSFTPPasswordCanceled(t *testing.T, app *App, result <-chan passwordPromptResult) {
	t.Helper()
	select {
	case got := <-result:
		if got.password != "" {
			t.Fatalf("password retained after cancel: %q", got.password)
		}
		if got.err == nil {
			t.Fatal("cancel must complete the waiter with an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("password waiter stranded after file-dialog cancel")
	}
	app.sftp.mu.Lock()
	stranded := app.sftp.passwordWait != nil
	app.sftp.mu.Unlock()
	if stranded {
		t.Fatal("passwordWait still set after cancel")
	}
	if app.model.FileDialog.Open {
		t.Fatal("password dialog still open after cancel")
	}
}
