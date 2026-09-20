package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestPromptSFTPHostKeyWaitsForDialog(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		decision, err := app.promptSFTPHostKey(context.TODO(), sftpb.HostKeyPrompt{
			Host:        "example.com",
			KeyType:     "ssh-ed25519",
			Fingerprint: "SHA256:abc",
		})
		if err != nil {
			t.Errorf("promptSFTPHostKey: %v", err)
		}
		if decision != sftpb.HostKeyReject {
			t.Errorf("decision = %v, want reject", decision)
		}
	}()
	waitSFTPHostKeyWait(t, app)
	app.finishHostKeyDialog(sftpb.HostKeyReject)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("promptSFTPHostKey blocked without finishing host key dialog")
	}
}

func TestPromptSFTPPasswordEscAndAltCCompleteWaiter(t *testing.T) {
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
			app.handleSFTPPasswordCancelKey(tt.ev)
			select {
			case got := <-result:
				if got.password != "" {
					t.Fatalf("password retained after cancel: %q", got.password)
				}
				if got.err == nil {
					t.Fatal("cancel must complete the waiter with an error")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("password waiter stranded after Esc/Alt+C")
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
		})
	}
}

func TestOverlappingSFTPPasswordPromptsDoNotStealWaiter(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	first := make(chan passwordPromptResult, 1)
	second := make(chan passwordPromptResult, 1)
	go func() {
		pw, err := app.promptSFTPPassword(context.Background(), sftpb.PasswordPrompt{
			User: "alice", Host: "first.example",
		})
		first <- passwordPromptResult{password: pw, err: err}
	}()
	waitSFTPPasswordWait(t, app)

	go func() {
		pw, err := app.promptSFTPPassword(context.Background(), sftpb.PasswordPrompt{
			User: "bob", Host: "second.example",
		})
		second <- passwordPromptResult{password: pw, err: err}
	}()

	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		app.sftp.mu.Lock()
		w := app.sftp.passwordWait
		app.sftp.mu.Unlock()
		if w != nil && w.prompt.Host == "second.example" {
			t.Fatal("overlapping dial stole the password waiter")
		}
		time.Sleep(time.Millisecond)
	}

	app.finishSFTPPassword("one")
	select {
	case got := <-first:
		if got.err != nil {
			t.Fatalf("first prompt: %v", got.err)
		}
		if got.password != "one" {
			t.Fatalf("first password = %q, want one", got.password)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first password waiter stranded")
	}

	waitSFTPPasswordWait(t, app)
	app.sftp.mu.Lock()
	host := ""
	if app.sftp.passwordWait != nil {
		host = app.sftp.passwordWait.prompt.Host
	}
	app.sftp.mu.Unlock()
	if host != "second.example" {
		t.Fatalf("second waiter host = %q, want second.example", host)
	}
	app.finishSFTPPassword("two")
	select {
	case got := <-second:
		if got.err != nil {
			t.Fatalf("second prompt: %v", got.err)
		}
		if got.password != "two" {
			t.Fatalf("second password = %q, want two", got.password)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second password waiter stranded")
	}
}

func TestOverlappingSFTPHostKeyPromptsDoNotStealWaiter(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	first := make(chan hostKeyPromptResult, 1)
	second := make(chan hostKeyPromptResult, 1)
	go func() {
		d, err := app.promptSFTPHostKey(context.Background(), sftpb.HostKeyPrompt{
			Host: "first.example", KeyType: "ssh-ed25519", Fingerprint: "SHA256:one",
		})
		first <- hostKeyPromptResult{decision: d, err: err}
	}()
	waitSFTPHostKeyWait(t, app)

	go func() {
		d, err := app.promptSFTPHostKey(context.Background(), sftpb.HostKeyPrompt{
			Host: "second.example", KeyType: "ssh-ed25519", Fingerprint: "SHA256:two",
		})
		second <- hostKeyPromptResult{decision: d, err: err}
	}()

	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		app.sftp.mu.Lock()
		w := app.sftp.hostKeyWait
		app.sftp.mu.Unlock()
		if w != nil && w.prompt.Host == "second.example" {
			t.Fatal("overlapping dial stole the host-key waiter")
		}
		time.Sleep(time.Millisecond)
	}

	app.finishHostKeyDialog(sftpb.HostKeyTrustSession)
	select {
	case got := <-first:
		if got.err != nil {
			t.Fatalf("first prompt: %v", got.err)
		}
		if got.decision != sftpb.HostKeyTrustSession {
			t.Fatalf("first decision = %v, want session trust", got.decision)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first host-key waiter stranded")
	}

	waitSFTPHostKeyWait(t, app)
	app.finishHostKeyDialog(sftpb.HostKeyReject)
	select {
	case got := <-second:
		if got.err != nil {
			t.Fatalf("second prompt: %v", got.err)
		}
		if got.decision != sftpb.HostKeyReject {
			t.Fatalf("second decision = %v, want reject", got.decision)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second host-key waiter stranded")
	}
}

func TestPromptSFTPPasswordContextCancelCompletesWaiter(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan passwordPromptResult, 1)
	go func() {
		pw, err := app.promptSFTPPassword(ctx, sftpb.PasswordPrompt{User: "alice", Host: "example.com"})
		result <- passwordPromptResult{password: pw, err: err}
	}()
	waitSFTPPasswordWait(t, app)
	cancel()
	select {
	case got := <-result:
		if got.password != "" {
			t.Fatalf("password retained after context cancel: %q", got.password)
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("password waiter ignored originating context cancel")
	}
}

func TestPromptSFTPHostKeyContextCancelCompletesWaiter(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan hostKeyPromptResult, 1)
	go func() {
		d, err := app.promptSFTPHostKey(ctx, sftpb.HostKeyPrompt{
			Host: "example.com", KeyType: "ssh-ed25519", Fingerprint: "SHA256:abc",
		})
		result <- hostKeyPromptResult{decision: d, err: err}
	}()
	waitSFTPHostKeyWait(t, app)
	cancel()
	select {
	case got := <-result:
		if got.decision != sftpb.HostKeyReject {
			t.Fatalf("decision = %v, want reject", got.decision)
		}
		if !errors.Is(got.err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("host-key waiter ignored originating context cancel")
	}
}

func TestDismissSFTPPasswordOpenPayloadClosesDialog(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.openSFTPPasswordDialog(sftpb.PasswordPrompt{User: "alice", Host: "example.com"})
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogSFTPPassword {
		t.Fatal("expected SFTP password dialog open")
	}
	app.openSFTPPasswordDialog(sftpb.PasswordPrompt{Host: sftpPromptDismissSentinel})
	if app.model.FileDialog.Open {
		t.Fatal("dismiss payload should close the password dialog")
	}
}

type passwordPromptResult struct {
	password string
	err      error
}

type hostKeyPromptResult struct {
	decision sftpb.HostKeyDecision
	err      error
}

func waitSFTPPasswordWait(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.sftp.mu.Lock()
		waiting := app.sftp.passwordWait != nil
		app.sftp.mu.Unlock()
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout waiting for password waiter")
}

func waitSFTPHostKeyWait(t *testing.T, app *App) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.sftp.mu.Lock()
		waiting := app.sftp.hostKeyWait != nil
		app.sftp.mu.Unlock()
		if waiting {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout waiting for host-key waiter")
}
