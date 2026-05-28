package jobs

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

type jobsHostStub struct{}

func (jobsHostStub) LayoutForTerminalSize(w, h int) ui.Layout {
	return geom.CalculateLayout(w, h, true, geom.PanelWidthSplit{})
}

func (jobsHostStub) SetTransientMessage(string, ui.MessageUrgency) {}
func (jobsHostStub) SetUnsupportedMessage(string)                  {}
func (jobsHostStub) RefreshBothPanels()                            {}
func (jobsHostStub) RequestBothPanelsVolumeSpaceRefreshAsync()     {}
func (jobsHostStub) ActivePanel() *panel.State                     { return nil }
func (jobsHostStub) ActivePanelSources() []string                  { return nil }
func (jobsHostStub) InactivePanel() *panel.State                   { return nil }
func (jobsHostStub) OpenTransferDialogSelfCopyRename(ui.TransferKind, string, string) {
}
func (jobsHostStub) HandleQuit() bool                           { return false }
func (jobsHostStub) HandleQuitImmediate() bool                  { return false }
func (jobsHostStub) OpenMenu()                                  {}
func (jobsHostStub) OpenMenuByShortcut(rune) bool               { return false }
func (jobsHostStub) Dispatch(string)                            {}
func (jobsHostStub) TryDispatchAuxiliaryScreens(string) bool    { return false }
func (jobsHostStub) ActionFromKeyEvent(*tcell.EventKey) string  { return "" }
func (jobsHostStub) SetJobFailedTransientMessage(error, string) {}
func (jobsHostStub) DevMode() bool                              { return false }

func TestOpenJobsViewFocusesFirstPendingBlocker(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)

	state := jobs.NewState()
	state.AddJob(&jobs.Job{
		ID:          "queued",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/src"),
		Destination: pathloc.MustParse("/dst"),
	})
	waiting := &jobs.Job{
		ID:          "wait",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusRunning,
		Sources:     pathloc.PathsForTest("/src2"),
		Destination: pathloc.MustParse("/dst2"),
	}
	state.AddJob(waiting)
	state.ApplyEvent(jobs.Event{
		Type:   jobs.EventJobBlockerRequest,
		JobID:  "wait",
		Status: jobs.StatusWaitingDecision,
		Blocker: &jobs.BlockerDetails{
			Kind: jobs.BlockerKindConflict,
			Conflict: &jobs.ConflictEvent{
				Source:      "/src2/file",
				Destination: "/dst2/file",
			},
		},
	})

	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}

	model := &ui.Model{}
	h := New(Deps{
		Host:     jobsHostStub{},
		Screen:   screen,
		Model:    model,
		State:    state,
		Config:   config.Default(),
		Keys:     bundle.Global,
		KeysJobs: bundle.Jobs,
	})

	h.OpenJobsView()

	if model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", model.ViewMode)
	}
	wantSel := ui.FirstJobEntryWaitingDecisionIndex(model.JobsList)
	if wantSel < 0 {
		t.Fatal("expected a waiting-decision job in JobsList")
	}
	if model.JobsView.Selected != wantSel {
		t.Fatalf("Selected = %d, want %d", model.JobsView.Selected, wantSel)
	}
	if model.JobsView.FocusPane != 1 {
		t.Fatalf("FocusPane = %d, want 1 (conflict panel)", model.JobsView.FocusPane)
	}
	if model.JobsView.ConflictButtonFocus != 0 {
		t.Fatalf("ConflictButtonFocus = %d, want 0 (Overwrite)", model.JobsView.ConflictButtonFocus)
	}
}
