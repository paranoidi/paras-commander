package jobs

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
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
func (jobsHostStub) PromptDanglingDirDelete([]string)           {}
func (jobsHostStub) ToggleLeaderMenu()                          {}
func (jobsHostStub) DispatchLeaderLetter(*tcell.EventKey) bool  { return false }

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
	// Match Size -> Skip across button rows.
	model.JobsView.ConflictButtonFocus = 2
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if model.JobsView.ConflictButtonFocus != 3 {
		t.Fatalf("ConflictButtonFocus = %d, want 3 after Right from Match Size", model.JobsView.ConflictButtonFocus)
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
		t.Fatalf("ConflictButtonFocus = %d, want 2 after Left from Skip", model.JobsView.ConflictButtonFocus)
	}
}

type jobsHostDispatchLetterStub struct {
	jobsHostStub
	called bool
}

func (s *jobsHostDispatchLetterStub) DispatchLeaderLetter(*tcell.EventKey) bool {
	s.called = true
	return true
}

// TestJobsViewBlockerPaneFocusedSkipsViMotion verifies that while the blocker/conflict pane has
// focus (FocusPane 1), vi-motion mode's hjkl-remap and direct-fire leader letters are both
// skipped, so a bare letter never fires a queue action or moves blocker-button focus while the
// user is mid-decision on a blocker.
func TestJobsViewBlockerPaneFocusedSkipsViMotion(t *testing.T) {
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
	model := &ui.Model{ViMotionMode: true}
	host := &jobsHostDispatchLetterStub{}
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

	before := model.JobsView.ConflictButtonFocus
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	if model.JobsView.ConflictButtonFocus != before {
		t.Fatalf("'h' with blocker pane focused must not move ConflictButtonFocus: got %d, want %d", model.JobsView.ConflictButtonFocus, before)
	}
	if host.called {
		t.Fatal("DispatchLeaderLetter must not be called while the blocker pane has focus")
	}
}

type jobsHostRefreshStub struct {
	jobsHostStub
	refreshed bool
}

func (h *jobsHostRefreshStub) RefreshBothPanels() { h.refreshed = true }

type jobsHostToggleLeaderMenuStub struct {
	jobsHostStub
	lookup  func(*tcell.EventKey) string
	toggled bool
}

func (s jobsHostToggleLeaderMenuStub) ActionFromKeyEvent(ev *tcell.EventKey) string {
	if s.lookup != nil {
		return s.lookup(ev)
	}
	return ""
}

func (s *jobsHostToggleLeaderMenuStub) ToggleLeaderMenu() { s.toggled = true }

func TestJobsViewColonKeyTogglesLeaderMenu(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)

	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}

	model := &ui.Model{}
	host := &jobsHostToggleLeaderMenuStub{
		lookup: func(ev *tcell.EventKey) string {
			id, ok := bundle.Jobs.Lookup(ev)
			if ok {
				return id
			}
			id, _ = bundle.Global.Lookup(ev)
			return id
		},
	}
	h := New(Deps{
		Host:     host,
		Screen:   screen,
		Model:    model,
		State:    jobs.NewState(),
		Config:   config.Default(),
		Keys:     bundle.Global,
		KeysJobs: bundle.Jobs,
	})
	h.OpenJobsView()

	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
	if !host.toggled {
		t.Fatal("expected ToggleLeaderMenu to be called for :")
	}
}

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
	marked, _, _ := ui.EntryPathJobMarkStatus("/src/a", model.JobPathMarks)
	if marked {
		t.Fatal("completed flatten job should not mark source paths")
	}
}

type jobsHostPanelsStub struct {
	jobsHostStub
	primary   *panel.State
	secondary *panel.State
}

func (h jobsHostPanelsStub) PrimaryPanel() *panel.State   { return h.primary }
func (h jobsHostPanelsStub) SecondaryPanel() *panel.State { return h.secondary }

func TestApplyRefreshesReloadsJobWriteDestinationPanels(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	srcPanel := &panel.State{Path: pathloc.MustParse("/src")}
	dstPanel := &panel.State{Path: pathloc.MustParse("/dst")}
	var srcLoads, dstLoads int
	srcPanel.ScheduleAsyncLoad = func(panel.AsyncLoadRequest) bool {
		srcLoads++
		return true
	}
	dstPanel.ScheduleAsyncLoad = func(panel.AsyncLoadRequest) bool {
		dstLoads++
		return true
	}

	model := &ui.Model{
		JobPathMarks: []ui.JobPathMark{{
			Type:        string(jobs.TypeCopy),
			Status:      string(jobs.StatusRunning),
			Sources:     []string{"/src/willow.txt"},
			Destination: "/dst",
			DestIsDir:   true,
		}},
	}
	h := New(Deps{
		Host:   jobsHostPanelsStub{primary: srcPanel, secondary: dstPanel},
		Screen: screen,
		Model:  model,
		State:  jobs.NewState(),
		Config: config.Default(),
	})
	h.refreshDestListing = true
	if h.ApplyRefreshes() {
		t.Fatal("non-terminal ApplyRefreshes should return false")
	}
	if dstLoads != 1 {
		t.Fatalf("destination panel loads = %d, want 1", dstLoads)
	}
	if srcLoads != 0 {
		t.Fatalf("source panel loads = %d, want 0", srcLoads)
	}

	// Unrelated panel path: no reload.
	dstLoads = 0
	model.JobPathMarks[0].Destination = "/other"
	h.refreshDestListing = true
	_ = h.ApplyRefreshes()
	if dstLoads != 0 {
		t.Fatalf("unrelated dest: destination panel loads = %d, want 0", dstLoads)
	}
}

// TestAddTransferJobMoveNeverSetsDereferenceSymlinks guards the enforcement choke point:
// a Move request must never carry DereferenceSymlinks through onto the resulting Job, even
// when the request (transfer dialog / TransferPreserve) asked for it — the option only makes
// sense for Copy.
func TestAddTransferJobMoveNeverSetsDereferenceSymlinks(t *testing.T) {
	t.Parallel()
	state := jobs.NewState()
	h := New(Deps{
		Host:   jobsHostStub{},
		Model:  &ui.Model{},
		State:  state,
		Config: config.Default(),
	})

	h.AddTransferJob(TransferJobRequest{
		Type:    jobs.TypeMove,
		Sources: []string{"/src/thistlewood.txt"},
		Dest:    "/dst",
		Preserve: jobs.TransferPreserve{
			DereferenceSymlinks: true,
		},
	})

	all := state.AllJobs()
	if len(all) != 1 {
		t.Fatalf("len(AllJobs()) = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeMove {
		t.Fatalf("job type = %v, want Move", all[0].Type)
	}
	if all[0].DereferenceSymlinks {
		t.Fatal("Move job must never have DereferenceSymlinks set, even when requested")
	}
}

// TestAddTransferJobCopyKeepsDereferenceSymlinks is the Copy-side counterpart: the same
// request field on a Copy job is passed through unchanged.
func TestAddTransferJobCopyKeepsDereferenceSymlinks(t *testing.T) {
	t.Parallel()
	state := jobs.NewState()
	h := New(Deps{
		Host:   jobsHostStub{},
		Model:  &ui.Model{},
		State:  state,
		Config: config.Default(),
	})

	h.AddTransferJob(TransferJobRequest{
		Type:    jobs.TypeCopy,
		Sources: []string{"/src/thistlewood.txt"},
		Dest:    "/dst",
		Preserve: jobs.TransferPreserve{
			DereferenceSymlinks: true,
		},
	})

	all := state.AllJobs()
	if len(all) != 1 {
		t.Fatalf("len(AllJobs()) = %d, want 1", len(all))
	}
	if !all[0].DereferenceSymlinks {
		t.Fatal("Copy job should keep DereferenceSymlinks as requested")
	}
}

// newMaximizeTestHandler builds a jobs-view handler with a failed job selected and the
// Operations pane focused (FocusPane 2: list, Details, Operations).
func newMaximizeTestHandler(t *testing.T) (*Handler, *ui.Model, *jobs.State) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)

	state := jobs.NewState()
	state.AddJob(&jobs.Job{
		ID:          "failing",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusFailed,
		Error:       "orchard thistle meadow lantern",
		Sources:     pathloc.PathsForTest("/orchard"),
		Destination: pathloc.MustParse("/meadow"),
	})
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	model := &ui.Model{}
	host := jobsHostKeymapStub{lookup: func(ev *tcell.EventKey) string {
		if id, ok := bundle.Jobs.Lookup(ev); ok {
			return id
		}
		return ""
	}}
	h := New(Deps{Host: host, Screen: screen, Model: model, State: state, Config: config.Default(), Keys: bundle.Global, KeysJobs: bundle.Jobs})
	h.OpenJobsView()
	model.JobsView.FocusPane = 2
	return h, model, state
}

func TestJobsViewEnterMaximizesOperationsAndKeysRestore(t *testing.T) {
	t.Parallel()
	keys := map[string]*tcell.EventKey{
		"esc":   tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone),
		"q":     tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone),
		"enter": tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
		"left":  tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone),
	}
	for name, ev := range keys {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, model, _ := newMaximizeTestHandler(t)
			h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			if !model.JobsView.Maximized {
				t.Fatal("Enter on Operations pane did not maximize")
			}
			h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
			if !model.JobsView.Maximized || model.JobsView.FocusPane != 2 {
				t.Fatal("Tab must be swallowed while maximized")
			}
			h.HandleJobsViewKey(ev)
			if model.JobsView.Maximized {
				t.Fatalf("%s did not restore layout", name)
			}
			if model.ViewMode != ui.ViewJobs {
				t.Fatalf("%s closed the jobs view", name)
			}
		})
	}
}

func TestJobsViewEnterOnListPaneDoesNotMaximize(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	model.JobsView.FocusPane = 0
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.JobsView.Maximized {
		t.Fatal("Enter on list pane maximized")
	}
}

func TestJobsViewEnterOnBlockerPaneStillSubmits(t *testing.T) {
	t.Parallel()
	h, model, state := newMaximizeTestHandler(t)
	state.AddJob(&jobs.Job{ID: "wait", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: pathloc.PathsForTest("/lantern"), Destination: pathloc.MustParse("/harbor")})
	state.ApplyEvent(jobs.Event{
		Type: jobs.EventJobBlockerRequest, JobID: "wait", Status: jobs.StatusWaitingDecision,
		Blocker: &jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Source: "/lantern/f", Destination: "/harbor/f"}},
	})
	h.SyncJobsList()
	h.OpenJobsView()
	if model.JobsView.FocusPane != 1 {
		t.Fatalf("FocusPane = %d, want blocker pane", model.JobsView.FocusPane)
	}
	model.JobsView.ConflictButtonFocus = 1
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if model.JobsView.Maximized {
		t.Fatal("Enter on blocker pane maximized")
	}
	// submitJobsConflictDecision resets the button focus, so 0 proves Enter reached the buttons.
	if model.JobsView.ConflictButtonFocus != 0 {
		t.Fatalf("ConflictButtonFocus = %d, want 0 after Enter submitted", model.JobsView.ConflictButtonFocus)
	}
}

func TestAppendJobOperationRecordsSuccessAndFailure(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	h.appendJobOperation(jobs.Event{Type: jobs.EventProgress, JobID: "failing", CurrentPath: "/orchard/acorn.txt"})
	h.appendJobOperation(jobs.Event{Type: jobs.EventProgress, JobID: "failing", CurrentPath: "/orchard/bramble.txt"})
	h.appendJobOperation(jobs.Event{Type: jobs.EventItemFailed, JobID: "failing", FailedItem: &jobs.FailedItem{
		Item:  ops.PlanItem{Src: pathloc.MustParse("/orchard/bramble.txt")},
		Error: "permission denied",
	}})
	got := model.JobOperations["failing"]
	if len(got) != 2 || got[0].Failed || !got[1].Failed || got[1].Error != "permission denied" {
		t.Fatalf("operations = %+v, want one success then the failure", got)
	}
	rows := h.visibleOperations()
	if len(rows) != 2 || !rows[0].Op.Failed {
		t.Fatalf("visible rows = %+v, want newest (the failure) first", rows)
	}
}

func fillOperations(model *ui.Model, n int) {
	ops := make([]ui.JobOperation, n)
	for i := range ops {
		ops[i] = ui.JobOperation{Label: fmt.Sprintf("pebble-%02d.txt", i)}
	}
	model.JobOperations = map[string][]ui.JobOperation{"failing": ops}
}

func TestJobsViewOperationsHomeEndScroll(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	fillOperations(model, 60)
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if model.JobsView.OpsScroll == 0 {
		t.Fatal("End did not scroll Operations to the bottom")
	}
	end := model.JobsView.OpsScroll
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if model.JobsView.OpsScroll != end {
		t.Fatalf("Down past the bottom moved scroll %d -> %d", end, model.JobsView.OpsScroll)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if model.JobsView.OpsScroll != 0 {
		t.Fatalf("Home left scroll at %d", model.JobsView.OpsScroll)
	}
}

func TestJobsViewOperationsFuzzyFilterKeys(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	rn := func(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }
	h.HandleJobsViewKey(rn('q')) // q types while the filter is closed too (unbound in [jobs])
	h.HandleJobsViewKey(rn('u'))
	if q := model.JobsView.OpsFilter.Query; q != "qu" {
		t.Fatalf("query = %q, want qu", q)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if q := model.JobsView.OpsFilter.Query; q != "q" {
		t.Fatalf("query after Backspace = %q, want q", q)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if model.JobsView.OpsFilter.Query != "" || model.ViewMode != ui.ViewJobs {
		t.Fatalf("first Esc: query %q, view %v; want the filter cleared and the view open", model.JobsView.OpsFilter.Query, model.ViewMode)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if model.ViewMode == ui.ViewJobs {
		t.Fatal("second Esc did not close the jobs view")
	}
}

func TestJobsViewTypingOnListPaneDoesNotFilter(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	model.JobsView.FocusPane = 0
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModNone))
	if model.JobsView.OpsFilter.Query != "" {
		t.Fatalf("query = %q, want typing ignored on the list pane", model.JobsView.OpsFilter.Query)
	}
}

func TestJobsViewResultFilterDialogAndReset(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if !model.JobsResultFilterDialog.Open || model.JobsResultFilterDialog.Filter != dialog.JobResultAll {
		t.Fatalf("F3 dialog = %+v, want open on All", model.JobsResultFilterDialog)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt)) // untick Success
	h.ResultFilterDialogOK()
	if model.JobsResultFilterDialog.Open || model.JobsView.OpsResultShown() != dialog.JobResultFailed {
		t.Fatalf("after OK shown = %v, want Failed only", model.JobsView.OpsResultShown())
	}
	model.JobsView.OpsFilter.Query = "lantern"
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift))
	if model.JobsView.OpsResultShown() != dialog.JobResultAll || model.JobsView.OpsFilter.Query != "" {
		t.Fatalf("reset left shown=%v query=%q", model.JobsView.OpsResultShown(), model.JobsView.OpsFilter.Query)
	}
}
