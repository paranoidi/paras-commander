package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSFTPConnectDialogFilterSelectsHost(t *testing.T) {
	root := t.TempDir()
	sshConfig := filepath.Join(root, "config")
	content := "Host alpha-server\n  HostName alpha.example.com\n\nHost beta-server\n  HostName beta.example.com\n"
	if err := os.WriteFile(sshConfig, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	app.config.SFTP.SSHConfigFile = sshConfig

	app.openSFTPConnectDialogForPanel(ui.PrimaryPanel)
	if !app.model.SFTPConnectDialog.Open {
		t.Fatal("expected SFTP connect dialog open")
	}
	if len(app.model.SFTPConnectDialog.Ranked) != 2 {
		t.Fatalf("ranked hosts = %d, want 2", len(app.model.SFTPConnectDialog.Ranked))
	}

	for _, r := range "alpha" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatal("unexpected quit")
		}
	}
	if len(app.model.SFTPConnectDialog.Ranked) == 0 {
		t.Fatal("expected fuzzy matches for alpha")
	}
	idx := app.model.SFTPConnectDialog.Ranked[0]
	if idx < 0 || idx >= len(app.sftpConnectHosts) {
		t.Fatalf("ranked index out of range: %d", idx)
	}
	if app.sftpConnectHosts[idx].Alias != "alpha-server" {
		t.Fatalf("selected host alias = %q, want alpha-server", app.sftpConnectHosts[idx].Alias)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.SFTPConnectDialog.Focus != 1 {
		t.Fatalf("focus = %d, want location input (1)", app.model.SFTPConnectDialog.Focus)
	}
	if !strings.Contains(app.model.SFTPConnectDialog.Location.Value, "alpha.example.com") {
		t.Fatalf("location = %q, want alpha host URI", app.model.SFTPConnectDialog.Location.Value)
	}
	if !strings.Contains(app.model.SFTPConnectDialog.Location.Value, "/~") {
		t.Fatalf("location = %q, want remote home (~)", app.model.SFTPConnectDialog.Location.Value)
	}
}
