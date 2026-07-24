package app

import (
	"context"
	"testing"
	"time"

	sftpb "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
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
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.sftp.mu.Lock()
		waiting := app.sftp.hostKeyWait != nil
		app.sftp.mu.Unlock()
		if waiting {
			app.finishHostKeyDialog(sftpb.HostKeyReject)
			break
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("promptSFTPHostKey blocked without finishing host key dialog")
	}
}
