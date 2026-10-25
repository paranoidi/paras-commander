package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
)

type sftpPasswordCancelHost struct {
	identityTestHost
	cancelCalls int
}

func (f *sftpPasswordCancelHost) CancelSFTPPassword() { f.cancelCalls++ }

func newSFTPPasswordCancelHandler(t *testing.T) (*sftpPasswordCancelHost, *Handler) {
	t.Helper()
	model := &ui.Model{}
	host := &sftpPasswordCancelHost{identityTestHost: identityTestHost{model: model, cfg: config.Default()}}
	return host, New(Deps{Host: host, Model: model})
}

func openSFTPPasswordFileDialog(h *Handler) {
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogSFTPPassword,
		Fields:     []uidialog.FileDialogField{{Label: "Password"}},
	}
}

func TestFileDialogCancelCancelsSFTPPassword(t *testing.T) {
	t.Parallel()
	host, h := newSFTPPasswordCancelHandler(t)
	openSFTPPasswordFileDialog(h)
	h.FileDialogCancel()
	if host.cancelCalls != 1 {
		t.Fatalf("CancelSFTPPassword calls = %d, want 1", host.cancelCalls)
	}
	if h.model.FileDialog.Open {
		t.Fatal("password dialog still open after cancel")
	}
}

func TestCloseFileDialogCancelsSFTPPassword(t *testing.T) {
	t.Parallel()
	host, h := newSFTPPasswordCancelHandler(t)
	openSFTPPasswordFileDialog(h)
	h.CloseFileDialog()
	if host.cancelCalls != 1 {
		t.Fatalf("CancelSFTPPassword calls = %d, want 1", host.cancelCalls)
	}
	if h.model.FileDialog.Open {
		t.Fatal("password dialog still open after CloseFileDialog")
	}
}

func TestCloseFileDialogSkipsCancelForOtherFileDialogs(t *testing.T) {
	t.Parallel()
	host, h := newSFTPPasswordCancelHandler(t)
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogRename,
		Fields:     []uidialog.FileDialogField{{Label: "Name", Value: "cedar.txt"}},
	}
	h.CloseFileDialog()
	if host.cancelCalls != 0 {
		t.Fatalf("CancelSFTPPassword calls = %d, want 0 for a non-password dialog", host.cancelCalls)
	}
}
