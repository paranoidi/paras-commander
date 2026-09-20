//go:build linux

package commands

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pools"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/workpool"
)

// TestForegroundPTYBatchesDoNotReplaceEachOthersDrawer starts two blocking foreground PTY
// batches and checks the first drawer's pointer is not overwritten by the second.
func TestForegroundPTYBatchesDoNotReplaceEachOthersDrawer(t *testing.T) {
	h := newPTYHandler(t, true)
	dir := t.TempDir()
	first := ptyDirEntry("willow", dir)
	second := ptyDirEntry("cedar", dir)

	h.StartRunForEachBatch(blockingPTYSpec([]localfs.Entry{first}))
	waitCommandPhase(t, h, 0, ui.CommandRunRunning)
	waitOwnsPanel(t, h, true)
	firstDrawer := waitTerminalDrawer(t, h)
	if !h.OwnsTerminalPanel() {
		t.Fatal("first batch should own the terminal panel")
	}

	h.StartRunForEachBatch(blockingPTYSpec([]localfs.Entry{second}))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		drawer := h.model.TerminalPanel.Drawer
		n := len(h.model.CommandsList)
		secondPhase := ui.CommandRunPhase(-1)
		if n > 1 {
			secondPhase = h.model.CommandsList[1].Phase
		}
		h.mu.RUnlock()
		if drawer != firstDrawer {
			t.Fatalf("second batch replaced the first drawer")
		}
		if n > 1 && secondPhase == ui.CommandRunPending {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	h.mu.RLock()
	drawer := h.model.TerminalPanel.Drawer
	n := len(h.model.CommandsList)
	secondPhase := ui.CommandRunPhase(-1)
	if n > 1 {
		secondPhase = h.model.CommandsList[1].Phase
	}
	h.mu.RUnlock()
	if drawer != firstDrawer {
		t.Fatal("second batch replaced the first drawer")
	}
	if n < 2 {
		t.Fatal("expected a second Commands-view row")
	}
	if secondPhase != ui.CommandRunPending {
		t.Fatalf("second row phase = %v, want Pending while the first batch holds the panel", secondPhase)
	}
	if !h.OwnsTerminalPanel() {
		t.Fatal("panel ownership should stay with the first batch")
	}

	if !h.closeSelectedPTYRow(0) {
		t.Fatal("expected to close the first batch's PTY")
	}
	waitCommandPhase(t, h, 1, ui.CommandRunRunning)
	secondDrawer := waitTerminalDrawer(t, h)
	if secondDrawer == nil || secondDrawer == firstDrawer {
		t.Fatal("second batch should install its own drawer after the first releases the panel")
	}
	if !h.closeSelectedPTYRow(1) {
		t.Fatal("expected to close the second batch's PTY")
	}
	waitBatchesIdle(t, h)
}

// TestForegroundPTYOwnershipIsBatchLevel covers pool-wait, inter-entry, and background
// PTY sessions: only a foreground batch owns the panel, and it keeps ownership between entries.
func TestForegroundPTYOwnershipIsBatchLevel(t *testing.T) {
	t.Run("pool-wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		reg := workpool.NewRegistry([]pools.Def{{Name: "one", MaxParallel: 1}})
		release, err := reg.Acquire(context.Background(), "one")
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		defer release()

		h := newPTYHandlerCtx(t, ctx, true)
		h.workPools = reg
		dir := t.TempDir()
		spec := blockingPTYSpec([]localfs.Entry{ptyDirEntry("osprey", dir)})
		spec.PoolName = "one"
		h.StartRunForEachBatch(spec)

		waitOwnsPanel(t, h, true)
		if _, _, ok := h.ActivePTYSession(); ok {
			t.Fatal("pool-waiting batch must not expose a panel session before the entry starts")
		}
		waitCommandPhase(t, h, 0, ui.CommandRunPending)
		if !h.OwnsTerminalPanel() {
			t.Fatal("foreground PTY batch should own the panel while waiting for a pool slot")
		}

		cancel()
		waitBatchesIdle(t, h)
		if h.OwnsTerminalPanel() {
			t.Fatal("canceled pool-wait batch should release panel ownership")
		}
	})

	t.Run("inter-entry", func(t *testing.T) {
		h := newPTYHandler(t, true)
		dir := t.TempDir()
		releaseSecond := make(chan struct{})
		var hitSecond atomic.Bool
		t.Cleanup(func() {
			select {
			case <-releaseSecond:
			default:
				close(releaseSecond)
			}
		})

		spec := RunForEachBatchSpec{
			Entries: []localfs.Entry{
				ptyDirEntry("maple", dir),
				ptyDirEntry("aspen", dir),
			},
			AllowDirs: true,
			WorkDir:   dir,
			PTY:       true,
			BuildItem: func(ent localfs.Entry) (RunForEachBuiltItem, error) {
				if ent.Name == "aspen" {
					hitSecond.Store(true)
					<-releaseSecond
				}
				return RunForEachBuiltItem{Argv: []string{"true"}, UserLine: ent.Name}, nil
			},
		}
		h.StartRunForEachBatch(spec)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if hitSecond.Load() && h.OwnsTerminalPanel() {
				if _, _, ok := h.ActivePTYSession(); !ok {
					close(releaseSecond)
					waitBatchesIdle(t, h)
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("expected batch-level ownership with no active session between entries")
	})

	t.Run("background", func(t *testing.T) {
		h := newPTYHandler(t, true)
		h.model.ViewMode = ui.ViewCommands
		dir := t.TempDir()
		spec := blockingPTYSpec([]localfs.Entry{ptyDirEntry("linden", dir)})
		spec.Background = true
		h.StartRunForEachBatch(spec)
		waitCommandPhase(t, h, 0, ui.CommandRunRunning)

		if h.OwnsTerminalPanel() {
			t.Fatal("background PTY must not own the terminal panel")
		}
		if _, _, ok := h.ActivePTYSession(); ok {
			t.Fatal("background PTY must not be the active panel session")
		}
		h.mu.RLock()
		visible := h.model.TerminalPanel.Visible
		drawer := h.model.TerminalPanel.Drawer
		view := h.model.ViewMode
		h.mu.RUnlock()
		if visible || drawer != nil {
			t.Fatal("background PTY must not touch the terminal panel")
		}
		if view != ui.ViewCommands {
			t.Fatalf("ViewMode = %v, want ViewCommands", view)
		}
		if !h.closeSelectedPTYRow(0) {
			t.Fatal("background PTY should still be terminable by row")
		}
		waitBatchesIdle(t, h)
	})
}

// TestForegroundPTYViewAndPanelChangeOnlyOnEventLoop checks ViewMode/TerminalPanel stay
// untouched until ApplyWake runs on the (simulated) event loop.
func TestForegroundPTYViewAndPanelChangeOnlyOnEventLoop(t *testing.T) {
	h := newPTYHandler(t, false)
	h.model.ViewMode = ui.ViewCommands
	dir := t.TempDir()
	h.StartRunForEachBatch(blockingPTYSpec([]localfs.Entry{ptyDirEntry("hemlock", dir)}))

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		visible := h.model.TerminalPanel.Visible
		view := h.model.ViewMode
		h.mu.RUnlock()
		if visible || view != ui.ViewCommands {
			t.Fatalf("ViewMode/TerminalPanel changed before ApplyWake: visible=%v view=%v", visible, view)
		}
		time.Sleep(5 * time.Millisecond)
	}

	stopPump := startWakePump(t, h, h.screen)
	t.Cleanup(stopPump)
	waitCommandPhase(t, h, 0, ui.CommandRunRunning)
	h.mu.RLock()
	visible := h.model.TerminalPanel.Visible
	focused := h.model.TerminalPanel.Focused
	view := h.model.ViewMode
	h.mu.RUnlock()
	if !visible || !focused {
		t.Fatalf("after event-loop ApplyWake: visible=%v focused=%v, want both true", visible, focused)
	}
	if view != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v after ApplyWake, want ViewBrowser", view)
	}
	if !h.closeSelectedPTYRow(0) {
		t.Fatal("expected to close the PTY after the show wake was applied")
	}
	waitBatchesIdle(t, h)
}

// TestForegroundPTYStartInputTerminateRace exercises start, PTY input, and terminate
// concurrently so the race detector can see unsynchronized panel/session access.
func TestForegroundPTYStartInputTerminateRace(t *testing.T) {
	h := newPTYHandler(t, true)
	dir := t.TempDir()
	h.StartRunForEachBatch(blockingPTYSpec([]localfs.Entry{ptyDirEntry("sumac", dir)}))
	waitCommandPhase(t, h, 0, ui.CommandRunRunning)
	waitActivePTY(t, h)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if sub, _, ok := h.ActivePTYSession(); ok && sub != nil {
				_, _ = sub.WritePTY([]byte("x"))
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = h.OwnsTerminalPanel()
			_, _, _ = h.ActivePTYSession()
		}
	}()
	time.Sleep(20 * time.Millisecond)
	if !h.closeSelectedPTYRow(0) {
		t.Fatal("expected to terminate the running PTY")
	}
	close(stop)
	wg.Wait()
	waitBatchesIdle(t, h)
}

func blockingPTYSpec(entries []localfs.Entry) RunForEachBatchSpec {
	return RunForEachBatchSpec{
		Entries:   entries,
		AllowDirs: true,
		WorkDir:   entries[0].Path,
		PTY:       true,
		BuildItem: func(localfs.Entry) (RunForEachBuiltItem, error) {
			return RunForEachBuiltItem{Argv: []string{"sleep", "60"}, UserLine: "sleep 60"}, nil
		},
	}
}

func ptyDirEntry(name, dir string) localfs.Entry {
	return localfs.Entry{Name: name, Path: dir, Type: localfs.EntryDirectory}
}

func newPTYHandler(t *testing.T, pump bool) *Handler {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := newPTYHandlerCtx(t, ctx, pump)
	// Cancel first on cleanup so a stuck batch unblocks before we wait for idle.
	t.Cleanup(cancel)
	return h
}

func newPTYHandlerCtx(t *testing.T, ctx context.Context, pump bool) *Handler {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 40)

	model := &ui.Model{ViewMode: ui.ViewBrowser}
	h := New(Deps{
		Host:   ptyHostStub{panel: &panel.State{}},
		Screen: screen,
		Model:  model,
		Mu:     &sync.RWMutex{},
		Ctx:    ctx,
	})
	stopPump := func() {}
	if pump {
		stopPump = startWakePump(t, h, screen)
	}
	t.Cleanup(func() {
		waitBatchesIdle(t, h)
		stopPump()
	})
	return h
}

func startWakePump(t *testing.T, h *Handler, screen tcell.Screen) func() {
	t.Helper()
	stop := make(chan struct{})
	var once sync.Once
	stopPump := func() {
		once.Do(func() {
			close(stop)
			_ = screen.PostEvent(tcell.NewEventInterrupt(nil))
		})
	}
	go func() {
		for {
			ev := screen.PollEvent()
			select {
			case <-stop:
				return
			default:
			}
			interrupt, ok := ev.(*tcell.EventInterrupt)
			if !ok {
				continue
			}
			p, ok := interrupt.Data().(WakePayload)
			if !ok {
				continue
			}
			h.ApplyWake(p)
		}
	}()
	return stopPump
}

func waitOwnsPanel(t *testing.T, h *Handler, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.OwnsTerminalPanel() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for OwnsTerminalPanel = %v", want)
}

func waitActivePTY(t *testing.T, h *Handler) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, ok := h.ActivePTYSession(); ok {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for ActivePTYSession")
}

func waitTerminalDrawer(t *testing.T, h *Handler) ui.TerminalDrawer {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.RLock()
		d := h.model.TerminalPanel.Drawer
		h.mu.RUnlock()
		if d != nil {
			return d
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for TerminalPanel.Drawer")
	return nil
}

type ptyHostStub struct {
	panel *panel.State
}

func (ptyHostStub) LayoutForTerminalSize(w, h int) ui.Layout {
	return ui.Layout{
		Width:    w,
		Height:   h,
		Terminal: ui.Rect{X: 0, Y: h - 12, Width: w, Height: 10},
	}
}

func (ptyHostStub) SetTransientMessage(string, ui.MessageUrgency)               {}
func (ptyHostStub) SetErrorMessage(string, error)                               {}
func (p ptyHostStub) ActivePanel() *panel.State                                 { return p.panel }
func (ptyHostStub) InactivePanel() *panel.State                                 { return &panel.State{} }
func (ptyHostStub) Styles() theme.Theme                                         { return theme.Theme{} }
func (ptyHostStub) BrowserMenuDefinitions() []menu.Definition                   { return nil }
func (ptyHostStub) SetTransientMessageBanner(string, string, ui.MessageUrgency) {}
func (ptyHostStub) ClearTransientMessage()                                      {}
func (ptyHostStub) CloseFileDialog()                                            {}
func (ptyHostStub) FocusedFileDialogField() *dialog.FileDialogField             { return nil }
func (ptyHostStub) RefreshAfterBackgroundCommand()                              {}
func (ptyHostStub) HandleQuit() bool                                            { return false }
func (ptyHostStub) HandleQuitImmediate() bool                                   { return false }
func (ptyHostStub) OpenMenu()                                                   {}
func (ptyHostStub) OpenMenuByShortcut(rune) bool                                { return false }
func (ptyHostStub) Dispatch(string)                                             {}
func (ptyHostStub) TryDispatchAuxiliaryScreens(string) bool                     { return false }
func (ptyHostStub) ActionFromKeyEvent(*tcell.EventKey) string                   { return "" }
func (ptyHostStub) ToggleLeaderMenu()                                           {}
func (ptyHostStub) DispatchLeaderLetter(*tcell.EventKey) bool                   { return false }

var _ Host = ptyHostStub{}
