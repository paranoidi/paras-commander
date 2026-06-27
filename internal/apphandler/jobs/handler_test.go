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
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

type jobsHostStub struct{}

func (jobsHostStub) LayoutForTerminalSize(w, h int) ui.Layout {
	return geom.CalculateLayout(w, h, true, geom.PanelWidthSplit{})
}

func (jobsHostStub) SetTransientMessage(string, ui.MessageUrgency) {}
func (jobsHostStub) SetErrorMessage(string, error)                 {}
func (jobsHostStub) SetUnsupportedMessage(string)                  {}
func (jobsHostStub) RefreshBothPanels()                            {}
func (jobsHostStub) RequestBothPanelsVolumeSpaceRefreshAsync()     {}
func (jobsHostStub) ActivePanel() *panel.State                     { return nil }
func (jobsHostStub) ActivePanelSources() []string                  { return nil }
func (jobsHostStub) InactivePanel() *panel.State                   { return nil }
func (jobsHostStub) PrimaryPanel() *panel.State                    { return &panel.State{} }
func (jobsHostStub) SecondaryPanel() *panel.State                  { return &panel.State{} }
func (jobsHostStub) OpenTransferDialogSelfCopyRename(dialog.TransferKind, string, string) {
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

type jobsHostKeymapStub struct {
	jobsHostStub
	lookup func(*tcell.EventKey) string
}

func (s jobsHostKeymapStub) ActionFromKeyEvent(ev *tcell.EventKey) string {
	if s.lookup != nil {
		return s.lookup(ev)
	}
	return ""
}

func TestJobsViewLeftInConflictPanelNavigatesButtonsNotClose(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)

	state := jobs.NewState()
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
	host := jobsHostKeymapStub{
		lookup: func(ev *tcell.EventKey) string {
			id, ok := bundle.Jobs.Lookup(ev)
			if ok {
				return id
			}
			return ""
		},
	}
	h := New(Deps{
		Host:     host,
		Screen:   screen,
		Model:    model,
		State:    state,
		Config:   config.Default(),
		Keys:     bundle.Global,
		KeysJobs: bundle.Jobs,
	})
	h.OpenJobsView()
	if model.JobsView.FocusPane != 1 {
		t.Fatalf("FocusPane = %d, want 1 (conflict panel)", model.JobsView.FocusPane)
	}

	// Move focus right among conflict buttons.
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if model.JobsView.ConflictButtonFocus != 1 {
		t.Fatalf("ConflictButtonFocus = %d, want 1 after Right", model.JobsView.ConflictButtonFocus)
	}
	// Overwrite All -> Skip All across button rows.
	model.JobsView.ConflictButtonFocus = 2
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if model.JobsView.ConflictButtonFocus != 3 {
		t.Fatalf("ConflictButtonFocus = %d, want 3 after Right from Overwrite All", model.JobsView.ConflictButtonFocus)
	}
	if model.ViewMode != ui.ViewJobs {
		t.Fatal("jobs view closed after Right in conflict panel")
	}

	// Left navigates buttons; it must not close the jobs view while conflict UI is focused.
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if model.ViewMode != ui.ViewJobs {
		t.Fatal("jobs view closed after Left in conflict panel")
	}
	if model.JobsView.ConflictButtonFocus != 2 {
		t.Fatalf("ConflictButtonFocus = %d, want 2 after Left from Skip All", model.JobsView.ConflictButtonFocus)
	}
}

type jobsHostRefreshStub struct {
	jobsHostStub
	refreshed bool
}

func (h *jobsHostRefreshStub) RefreshBothPanels() { h.refreshed = true }

func TestApplyRefreshesReloadsPanelsAndSyncsJobPathMarks(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	state := jobs.NewState()
	state.AddJob(&jobs.Job{
		ID:          "flat",
		Type:        jobs.TypeFlatten,
		Status:      jobs.StatusRunning,
		Sources:     pathloc.PathsForTest("/src/a", "/src/b"),
		Destination: pathloc.MustParse("/dst"),
	})
	state.ApplyEvent(jobs.Event{Type: jobs.EventCompleted, JobID: "flat", Status: jobs.StatusCompleted})

	model := &ui.Model{}
	host := &jobsHostRefreshStub{}
	h := New(Deps{
		Host:   host,
		Screen: screen,
		Model:  model,
		State:  state,
		Config: config.Default(),
	})
	h.refreshTerminal = true

	if !h.ApplyRefreshes() {
		t.Fatal("ApplyRefreshes should report terminal refresh")
	}
	if !host.refreshed {
		t.Fatal("RefreshBothPanels was not called")
	}
	if len(model.JobPathMarks) != 1 {
		t.Fatalf("JobPathMarks len = %d, want 1", len(model.JobPathMarks))
	}
	if model.JobPathMarks[0].Status != string(jobs.StatusCompleted) {
		t.Fatalf("JobPathMarks status = %q, want completed", model.JobPathMarks[0].Status)
	}
	marked, _ := ui.EntryPathJobMarkStatus("/src/a", model.JobPathMarks)
	if marked {
		t.Fatal("completed flatten job should not mark source paths")
	}
}
