package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
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

func TestHandleFileDialogKeyCancelsSFTPPassword(t *testing.T) {
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
			host, h := newSFTPPasswordCancelHandler(t)
			openSFTPPasswordFileDialog(h)
			h.HandleFileDialogKey(tt.ev)
			if host.cancelCalls != 1 {
				t.Fatalf("CancelSFTPPassword calls = %d, want 1", host.cancelCalls)
			}
			if h.model.FileDialog.Open {
				t.Fatal("password dialog still open after cancel")
			}
		})
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

func TestMassRenameRadioAndCheckboxColumnNav(t *testing.T) {
	h := &Handler{model: &ui.Model{}}
	d := &h.model.FileDialog
	d.DialogType = uidialog.FileDialogMassRename
	d.MassRenameMode = uidialog.MassRenameModeUISimple
	press := func(k tcell.Key) {
		h.massRenameMoveFocusKey(tcell.NewEventKey(k, 0, tcell.ModNone))
	}
	expect := func(want int) {
		t.Helper()
		if d.FocusedField != want {
			t.Fatalf("FocusedField = %d, want %d", d.FocusedField, want)
		}
	}

	d.FocusedField = 0
	press(tcell.KeyLeft)
	expect(0)
	press(tcell.KeyUp)
	expect(0)
	press(tcell.KeyRight)
	press(tcell.KeyRight)
	press(tcell.KeyRight)
	press(tcell.KeyRight)
	expect(3)
	// Column 3 is Ignore extension in Simple mode, and Up returns to the same column.
	press(tcell.KeyDown)
	expect(uidialog.MassRenameIgnoreExtFocusIdx(*d))
	press(tcell.KeyUp)
	expect(3)
	d.FocusedField = 1
	press(tcell.KeyDown)
	expect(uidialog.MassRenameStripFocusIdx(*d))

	// Capitalize: column 2 holds Ignore extension, column 3 has no checkbox -> last visible.
	d.MassRenameMode = uidialog.MassRenameModeUICapitalize
	d.FocusedField = 2
	press(tcell.KeyDown)
	expect(uidialog.MassRenameIgnoreExtFocusIdx(*d))
	d.FocusedField = 3
	press(tcell.KeyDown)
	expect(uidialog.MassRenameIgnoreExtFocusIdx(*d))
	press(tcell.KeyUp)
	expect(2)

	// External: only two checkboxes; radio 3 falls back to the last one (Trim whitespace).
	d.MassRenameMode = uidialog.MassRenameModeUIExternalEditor
	d.FocusedField = 3
	press(tcell.KeyDown)
	expect(uidialog.MassRenameStripFocusIdx(*d))
}
