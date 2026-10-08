package dialog

import (
	"context"
	"io"
	"io/fs"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

const remoteTestRoot = "sftp://alice@example.com/var"

// passwordRequestPayload is posted by passwordBackend when Stat/Mkdir/Rename/List need a
// password. The event-loop test accepts it the same way Run() accepts sftpPasswordOpenPayload.
type passwordRequestPayload struct {
	reply chan struct{}
}

type startOpPayload struct {
	fn func()
}

// passwordBackend is a fake SFTP backend whose Stat/Mkdir/Rename/List request a password
// (post an interrupt and wait) so the test can assert the UI loop still delivers it.
type passwordBackend struct {
	screen tcell.Screen
	once   sync.Once
}

func (b *passwordBackend) Scheme() pathloc.Scheme { return pathloc.SchemeSFTP }

func (b *passwordBackend) requestPassword() {
	b.once.Do(func() {
		reply := make(chan struct{})
		_ = b.screen.PostEvent(tcell.NewEventInterrupt(passwordRequestPayload{reply: reply}))
		<-reply
	})
}

func (b *passwordBackend) Stat(context.Context, pathloc.Path) (fsbackend.Entry, error) {
	b.requestPassword()
	return fsbackend.Entry{}, os.ErrNotExist
}

func (b *passwordBackend) Mkdir(context.Context, pathloc.Path, fs.FileMode) error {
	b.requestPassword()
	return nil
}

func (b *passwordBackend) Rename(context.Context, pathloc.Path, pathloc.Path) error {
	b.requestPassword()
	return nil
}

func (b *passwordBackend) List(context.Context, pathloc.Path) ([]fsbackend.Entry, error) {
	b.requestPassword()
	return nil, nil
}

func (b *passwordBackend) OpenRead(context.Context, pathloc.Path) (io.ReadCloser, error) {
	return nil, fs.ErrInvalid
}
func (b *passwordBackend) OpenWrite(context.Context, pathloc.Path, int64, fsbackend.CreateOpts) (io.WriteCloser, error) {
	return nil, fs.ErrInvalid
}
func (b *passwordBackend) Remove(context.Context, pathloc.Path) error { return fs.ErrInvalid }
func (b *passwordBackend) ReadSymlink(context.Context, pathloc.Path) (string, error) {
	return "", fs.ErrInvalid
}
func (b *passwordBackend) Symlink(context.Context, pathloc.Path, string) error {
	return fs.ErrInvalid
}

func seedRemotePanel(m *ui.Model) {
	root := pathloc.MustParse(remoteTestRoot)
	file := localfs.Entry{
		Name: "cedar.txt",
		Path: remoteTestRoot + "/cedar.txt",
		Type: localfs.EntryFile,
	}
	dir := localfs.Entry{
		Name: "harbor",
		Path: remoteTestRoot + "/harbor",
		Type: localfs.EntryDirectory,
	}
	m.Primary.Path = root
	m.Primary.Entries = []localfs.Entry{file, dir}
	m.Secondary.Path = root
	m.Secondary.Entries = []localfs.Entry{file, dir}
}

func newRemotePasswordHarness(t *testing.T) (*Handler, tcell.SimulationScreen, *passwordBackend) {
	t.Helper()
	screen := uitest.Screen(t, 80, 24)
	model := &ui.Model{}
	seedRemotePanel(model)
	cfg := config.Default()
	cfg.Preview.Prefetch = false
	fh := &identityTestHost{model: model, cfg: cfg}
	jobState := jobs.NewState()
	jobsH := jobsctrl.New(jobsctrl.Deps{
		Host: fh, Screen: screen, Model: model, State: jobState, Config: cfg,
	})
	previewH := previewctrl.New(previewctrl.Deps{
		Host: fh, Screen: screen, Model: model, Ctx: context.Background(),
	})
	be := &passwordBackend{screen: screen}
	h := New(Deps{
		Host: fh, Screen: screen, Model: model, Jobs: jobsH, Preview: previewH,
	})
	h.testRemote = be
	return h, screen, be
}

func runRemoteOpUILoop(h *Handler, screen tcell.SimulationScreen) (accepted bool) {
	for {
		ev := screen.PollEvent()
		interruptEv, ok := ev.(*tcell.EventInterrupt)
		if !ok {
			continue
		}
		switch d := interruptEv.Data().(type) {
		case startOpPayload:
			d.fn()
		case passwordRequestPayload:
			h.model.FileDialog = uidialog.FileDialogState{
				Open:       true,
				DialogType: uidialog.FileDialogSFTPPassword,
				Fields:     []uidialog.FileDialogField{{Label: "Password"}},
			}
			h.host.ExecuteSFTPPassword()
			accepted = true
			close(d.reply)
		case RemoteFileOpPayload:
			h.ApplyRemoteFileOp(d)
			return accepted
		}
	}
}

func assertPasswordAcceptedOnLoop(t *testing.T, h *Handler, screen tcell.SimulationScreen, start func()) {
	t.Helper()
	done := make(chan bool, 1)
	go func() {
		done <- runRemoteOpUILoop(h, screen)
	}()
	if err := screen.PostEvent(tcell.NewEventInterrupt(startOpPayload{fn: start})); err != nil {
		t.Fatal(err)
	}
	select {
	case accepted := <-done:
		if !accepted {
			t.Fatal("password dialog was not accepted on the event loop")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event loop blocked waiting for an SFTP password the blocked loop must deliver")
	}
}

func TestRemotePlanningDoesNotBlockPasswordDialog(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		start func(h *Handler)
	}{
		{
			name: "mkdir",
			start: func(h *Handler) {
				h.OpenMkdirDialog(false)
				h.model.FileDialog.Fields[0].Value = "meadow"
				h.model.FileDialog.Fields[0].PrefillPending = false
				h.executeMkdir()
			},
		},
		{
			name: "rename",
			start: func(h *Handler) {
				h.model.FileDialog = uidialog.FileDialogState{
					Open:         true,
					DialogType:   uidialog.FileDialogRename,
					Fields:       []uidialog.FileDialogField{{Label: "Name", Value: "pine.txt"}},
					RenameSource: remoteTestRoot + "/cedar.txt",
				}
				h.executeRename()
			},
		},
		{
			name: "transfer dest probe",
			start: func(h *Handler) {
				h.model.TransferDialog = uidialog.TransferDialogState{
					Open:        true,
					Kind:        uidialog.TransferKindCopy,
					Destination: uidialog.FileDialogField{Value: remoteTestRoot + "/dest"},
				}
				h.confirmTransfer()
			},
		},
		{
			name: "extract dest probe",
			start: func(h *Handler) {
				h.model.FileDialog = uidialog.FileDialogState{
					Open:           true,
					DialogType:     uidialog.FileDialogExtract,
					Fields:         []uidialog.FileDialogField{{Value: remoteTestRoot + "/dest"}},
					ExtractSources: []string{remoteTestRoot + "/cedar.tar.gz"},
				}
				h.ExecuteExtract()
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, screen, _ := newRemotePasswordHarness(t)
			assertPasswordAcceptedOnLoop(t, h, screen, func() { tc.start(h) })
		})
	}
}
