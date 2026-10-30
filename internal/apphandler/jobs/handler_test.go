package jobs

import (
	"fmt"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/oplog"
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
func (jobsHostStub) DiskUsagePainter() ui.DiskUsagePainter      { return nil }
func (jobsHostStub) PromptDanglingDirDelete([]string)           {}
func (jobsHostStub) ToggleLeaderMenu()                          {}
func (jobsHostStub) AdjustJobsThroughputChartColumnMS(int)      {}
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

func TestJobsViewMaximizedOperationsTypesIntoFilter(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	for _, r := range "quill" {
		h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if !model.JobsView.Maximized {
		t.Fatal("typing restored the maximized pane")
	}
	if got := model.JobsView.OpsFilter.Query; got != "quill" {
		t.Fatalf("query = %q, want quill", got)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if model.JobsView.OpsFilter.Query != "" || !model.JobsView.Maximized {
		t.Fatalf("first Esc must close the filter only: query %q, maximized %v", model.JobsView.OpsFilter.Query, model.JobsView.Maximized)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if model.JobsView.Maximized {
		t.Fatal("second Esc did not restore the layout")
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

// selectedOps returns the operation log of the job newMaximizeTestHandler selected.
func selectedOps(t *testing.T, h *Handler) *oplog.Log {
	t.Helper()
	log := h.selectedJobEntry().Ops
	if log == nil {
		t.Fatal("selected job has no operation log")
	}
	return log
}

// waitOpsWindow refreshes the Operations window until done reports true.
func waitOpsWindow(t *testing.T, h *Handler, model *ui.Model, done func(ui.JobsOpsWindow) bool) ui.JobsOpsWindow {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h.RefreshOpsWindow()
		if done(model.JobsOpsWindow) {
			return model.JobsOpsWindow
		}
		if time.Now().After(deadline) {
			t.Fatalf("Operations window never settled: %+v", model.JobsOpsWindow)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func opsLabels(win ui.JobsOpsWindow) string {
	var out []string
	for _, r := range win.Rows {
		out = append(out, r.Op.Label)
	}
	return strings.Join(out, ",")
}

func TestRefreshOpsWindowNewestFirstAndFilters(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	log := selectedOps(t, h)
	log.Append(oplog.Record{Result: oplog.Done, Label: "orchard/acorn.txt"})
	log.Append(oplog.Record{Result: oplog.Failed, Label: "orchard/bramble.txt", Reason: "permission denied"})
	log.Append(oplog.Record{Result: oplog.Skipped, Label: "valley/acorn.md", Reason: "exists"})
	h.RefreshOpsWindow()
	if got, want := opsLabels(model.JobsOpsWindow), "valley/acorn.md,orchard/bramble.txt,orchard/acorn.txt"; got != want {
		t.Fatalf("all = %s, want %s", got, want)
	}
	model.JobsView.OpsResultHidden = dialog.JobResultAll &^ dialog.JobResultSkipped
	h.RefreshOpsWindow()
	if got := opsLabels(model.JobsOpsWindow); got != "valley/acorn.md" || model.JobsOpsWindow.Total != 1 {
		t.Fatalf("skipped only = %s", got)
	}
	model.JobsView.OpsResultHidden = dialog.JobResultNone
	model.JobsView.OpsFilter.Query = "acrn"
	win := waitOpsWindow(t, h, model, func(w ui.JobsOpsWindow) bool { return !w.Scanning && w.Total > 0 })
	if got := opsLabels(win); got != "valley/acorn.md,orchard/acorn.txt" {
		t.Fatalf("fuzzy = %s", got)
	}
	if len(win.Rows[0].Ranges) == 0 {
		t.Fatal("fuzzy rows carry no match ranges")
	}
	log.Append(oplog.Record{Result: oplog.Done, Label: "thicket/acorn.bin"})
	win = waitOpsWindow(t, h, model, func(w ui.JobsOpsWindow) bool { return !w.Scanning && w.Total == 3 })
	if win.Rows[0].Op.Label != "thicket/acorn.bin" {
		t.Fatalf("rows appended after the scan were not matched: %s", opsLabels(win))
	}
}

func TestRefreshOpsWindowRateLimitsJobActivity(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	h.config.Jobs.OperationsRefreshIntervalMS = 60_000
	log := selectedOps(t, h)
	log.Append(oplog.Record{Result: oplog.Done, Label: "meadow/thistle.txt"})
	h.RefreshOpsWindow()
	log.Append(oplog.Record{Result: oplog.Done, Label: "meadow/clover.txt"})
	h.RefreshOpsWindow()
	if model.JobsOpsWindow.Total != 1 {
		t.Fatalf("job activity rebuilt the window inside the interval: Total %d", model.JobsOpsWindow.Total)
	}
	if !h.ops.trailing.Armed() {
		t.Fatal("held-back rows armed no trailing wake")
	}
	model.JobsView.OpsResultHidden = dialog.JobResultFailed
	h.RefreshOpsWindow()
	if model.JobsOpsWindow.Total != 2 {
		t.Fatalf("a filter change must rebuild at once: Total %d", model.JobsOpsWindow.Total)
	}
	h.config.Jobs.OperationsRefreshIntervalMS = 0
	log.Append(oplog.Record{Result: oplog.Done, Label: "meadow/sorrel.txt"})
	h.RefreshOpsWindow()
	if model.JobsOpsWindow.Total != 3 {
		t.Fatalf("interval 0 must rebuild every frame: Total %d", model.JobsOpsWindow.Total)
	}
}

func TestRefreshOpsWindowReadsSpilledRowsAsync(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	log := oplog.New("spill", oplog.Config{MemoryRows: 1000, Session: oplog.NewSession(t.TempDir())})
	t.Cleanup(log.Close)
	for i := range 5000 {
		log.Append(oplog.Record{Result: oplog.Done, Label: fmt.Sprintf("pebble-%04d.txt", i)})
	}
	model.JobsList[model.JobsView.Selected].Ops = log
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	h.RefreshOpsWindow()
	if !model.JobsOpsWindow.Rows[0].Pending {
		t.Fatal("rows only in the spill file must render pending first")
	}
	win := waitOpsWindow(t, h, model, func(w ui.JobsOpsWindow) bool { return len(w.Rows) > 0 && !w.Rows[0].Pending })
	if last := win.Rows[len(win.Rows)-1].Op.Label; last != "pebble-0000.txt" {
		t.Fatalf("bottom row = %q, want the oldest record", last)
	}
}

func fillOperations(t *testing.T, h *Handler, n int) {
	log := selectedOps(t, h)
	for i := range n {
		log.Append(oplog.Record{Result: oplog.Done, Label: fmt.Sprintf("pebble-%02d.txt", i)})
	}
}

func TestJobsViewOperationsHomeEndScroll(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	fillOperations(t, h, 60)
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

// TestOperationsScrolledViewportStaysWhileRowsArrive: scrolled away from the newest row, new
// records keep the same rows on screen; at the newest edge the pane follows them; leaving the
// pane returns it to the newest row.
func TestOperationsScrolledViewportStaysWhileRowsArrive(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	fillOperations(t, h, 60)
	h.refreshOpsWindow(true)
	if top := model.JobsOpsWindow.Rows[0].Op.Label; top != "pebble-59.txt" {
		t.Fatalf("top row = %q, want the newest record", top)
	}
	for range 3 {
		h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	top := model.JobsOpsWindow.Rows[0].Op.Label
	h.refreshOpsWindow(true)
	if got := model.JobsOpsWindow.Rows[0].Op.Label; got != "pebble-56.txt" {
		t.Fatalf("scrolled top row = %q (before refresh %q), want pebble-56.txt", got, top)
	}
	log := selectedOps(t, h)
	log.Append(oplog.Record{Result: oplog.Done, Label: "pebble-60.txt"})
	log.Append(oplog.Record{Result: oplog.Done, Label: "pebble-61.txt"})
	h.refreshOpsWindow(true)
	if got := model.JobsOpsWindow.Rows[0].Op.Label; got != "pebble-56.txt" || model.JobsView.OpsScroll != 5 {
		t.Fatalf("after appends: top row %q, scroll %d; want pebble-56.txt at 5", got, model.JobsView.OpsScroll)
	}

	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	log.Append(oplog.Record{Result: oplog.Done, Label: "pebble-62.txt"})
	h.refreshOpsWindow(true)
	if got := model.JobsOpsWindow.Rows[0].Op.Label; got != "pebble-62.txt" {
		t.Fatalf("at the newest edge: top row %q, want the new record", got)
	}

	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	model.JobsView.FocusPane = 0
	h.refreshOpsWindow(false)
	if model.JobsView.OpsScroll != 0 {
		t.Fatalf("unfocused scroll = %d, want 0", model.JobsView.OpsScroll)
	}
}

// TestOperationsNewestLastKeys: with the newest row at the bottom, Up moves away from it and End
// returns to it.
func TestOperationsNewestLastKeys(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	model.JobsOpsNewestLast = true
	fillOperations(t, h, 60)
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if model.JobsView.OpsScroll != 1 {
		t.Fatalf("Up: scroll = %d, want 1", model.JobsView.OpsScroll)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if model.JobsView.OpsScroll == 1 || model.JobsView.OpsScroll == 0 {
		t.Fatalf("Home: scroll = %d, want the oldest rows", model.JobsView.OpsScroll)
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	if model.JobsView.OpsScroll != 0 {
		t.Fatalf("End: scroll = %d, want 0", model.JobsView.OpsScroll)
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
	if model.JobsResultFilterDialog.Open || model.JobsView.OpsResultShown() != dialog.JobResultFailed|dialog.JobResultSkipped {
		t.Fatalf("after OK shown = %v, want Failed + Skipped", model.JobsView.OpsResultShown())
	}
	model.JobsView.OpsFilter.Query = "lantern"
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift))
	if model.JobsView.OpsResultShown() != dialog.JobResultAll || model.JobsView.OpsFilter.Query != "" {
		t.Fatalf("reset left shown=%v query=%q", model.JobsView.OpsResultShown(), model.JobsView.OpsFilter.Query)
	}
}

func TestJobsViewExpandSourcesFiltersOperations(t *testing.T) {
	t.Parallel()
	h, model, state := newMaximizeTestHandler(t)
	job := &jobs.Job{
		ID:          "multi",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusFailed,
		Sources:     pathloc.PathsForTest("/harbor/lantern", "/harbor/willow"),
		Destination: pathloc.MustParse("/meadow"),
	}
	state.AddJob(job)
	item := func(src string, source int32) ops.PlanItem {
		return ops.PlanItem{Src: pathloc.MustParse(src), Dst: pathloc.MustParse("/meadow" + src), Source: source}
	}
	job.RecordOperation(item("/harbor/lantern/acorn.txt", 0), ops.ItemDone, "")
	job.RecordOperation(item("/harbor/lantern/bramble.txt", 0), ops.ItemDone, "")
	job.RecordOperation(item("/harbor/willow/clover.txt", 1), ops.ItemFailed, "permission denied")
	h.SyncJobsList()
	idx := slices.IndexFunc(model.JobsList, func(e ui.JobEntry) bool { return e.ID == "multi" })
	model.JobsView.FocusPane = 0
	model.JobsView.Selected = idx

	key := func(k tcell.Key, mod tcell.ModMask) { h.HandleJobsViewKey(tcell.NewEventKey(k, 0, mod)) }
	key(tcell.KeyRight, tcell.ModAlt)
	if !model.JobsView.Expanded["multi"] {
		t.Fatal("jobs.expand did not expand the selected job")
	}
	key(tcell.KeyDown, tcell.ModNone)
	key(tcell.KeyDown, tcell.ModNone)
	if model.JobsView.Selected != idx || model.JobsView.SelectedSource(model.JobsList) != 1 {
		t.Fatalf("cursor = job %d sub %d, want job %d source 1", model.JobsView.Selected, model.JobsView.SelectedSub, idx)
	}
	h.RefreshOpsWindow()
	if got := opsLabels(model.JobsOpsWindow); got != "harbor/willow/clover.txt" {
		t.Fatalf("source 1 operations = %s", got)
	}
	key(tcell.KeyUp, tcell.ModNone)
	h.RefreshOpsWindow()
	if got := opsLabels(model.JobsOpsWindow); got != "harbor/lantern/bramble.txt,harbor/lantern/acorn.txt" {
		t.Fatalf("source 0 operations = %s", got)
	}
	key(tcell.KeyLeft, tcell.ModAlt)
	if model.JobsView.Expanded["multi"] || model.JobsView.SelectedSub != 0 || model.JobsView.Selected != idx {
		t.Fatalf("jobs.collapse left expanded=%v sub=%d job=%d", model.JobsView.Expanded["multi"], model.JobsView.SelectedSub, model.JobsView.Selected)
	}
	h.RefreshOpsWindow()
	if model.JobsOpsWindow.Total != 3 {
		t.Fatalf("job row operations = %d rows, want all 3", model.JobsOpsWindow.Total)
	}
	if model.ViewMode != ui.ViewJobs {
		t.Fatal("jobs.collapse must not close the view")
	}
}

// TestJobsListCopiesSourceDataOnlyWhenExpanded: list sync skips per-source data of collapsed
// jobs, and expanding fetches it at once.
func TestJobsListCopiesSourceDataOnlyWhenExpanded(t *testing.T) {
	t.Parallel()
	h, model, state := newMaximizeTestHandler(t)
	state.AddJob(&jobs.Job{
		ID:          "thicket",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusQueued,
		Sources:     pathloc.PathsForTest("/harbor/lantern", "/harbor/willow"),
		Destination: pathloc.MustParse("/meadow"),
	})
	h.SyncJobsList()
	idx := slices.IndexFunc(model.JobsList, func(e ui.JobEntry) bool { return e.ID == "thicket" })
	if e := model.JobsList[idx]; e.SourceStats != nil || e.SourcePhases != nil {
		t.Fatalf("collapsed entry carries source data: %+v %+v", e.SourceStats, e.SourcePhases)
	}
	model.JobsView.FocusPane, model.JobsView.Selected = 0, idx
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if e := model.JobsList[idx]; len(e.SourceStats) != 2 || len(e.SourcePhases) != 2 {
		t.Fatalf("expanded entry source data: %+v %+v", e.SourceStats, e.SourcePhases)
	}
}

func TestProgressMovesPathMarksToActiveSource(t *testing.T) {
	t.Parallel()
	h, model, state := newMaximizeTestHandler(t)
	h.closeJobsView()
	job := &jobs.Job{
		ID:          "walnut",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusRunning,
		Sources:     pathloc.PathsForTest("/harbor/lantern", "/harbor/willow"),
		Destination: pathloc.MustParse("/meadow"),
		DestIsDir:   true,
	}
	state.AddJob(job)
	job.StartSource(0)
	h.SyncJobPathMarks()
	i := slices.IndexFunc(model.JobPathMarks, func(m ui.JobPathMark) bool { return m.ID == "walnut" })
	before := model.JobPathMarks[i]
	if before.ActiveSource != 0 {
		t.Fatalf("synced ActiveSource = %d, want 0", before.ActiveSource)
	}
	if ok, _ := ui.PanelInsideJobWriteTree("/meadow/willow", model.JobPathMarks); ok {
		t.Fatal("a pending source's destination counts as written")
	}

	job.StartSource(1)
	h.applyJobEventBatch([]jobs.Event{{Type: jobs.EventProgress, JobID: "walnut", Status: jobs.StatusRunning, CurrentPath: "/harbor/willow/clover.txt"}})
	after := model.JobPathMarks[i]
	if after.ActiveSource != 1 || after.SourcePhases[0] != jobs.SourceFinished {
		t.Fatalf("mark after progress = active %d phases %v", after.ActiveSource, after.SourcePhases)
	}
	if ok, _ := ui.PanelInsideJobWriteTree("/meadow/willow", model.JobPathMarks); !ok {
		t.Fatal("the active source's destination is not a write tree after the move")
	}
	if ok, _ := ui.PanelInsideJobWriteTree("/meadow/lantern", model.JobPathMarks); ok {
		t.Fatal("the finished source's destination still counts as written")
	}
}

func TestJobsViewCtrlSpaceTogglesOperationsAtBottom(t *testing.T) {
	t.Parallel()
	h, model, _ := newMaximizeTestHandler(t)
	_, side := h.jobsPaneLayout()
	ctrlSpace := tcell.NewEventKey(tcell.KeyCtrlSpace, 0, tcell.ModCtrl)
	h.HandleJobsViewKey(ctrlSpace)
	if !model.JobsOpsBottom {
		t.Fatal("Ctrl-Space did not move Operations to the bottom")
	}
	order, rects := h.jobsPaneLayout()
	ops := rects[ui.JobsPaneOperations]
	if ops.Width != 100 || ops.X != 0 {
		t.Fatalf("bottom Operations = %+v, want full width", ops)
	}
	if detail := rects[ui.JobsPaneDetail]; ops.Y != detail.Y+detail.Height {
		t.Fatalf("Operations %+v not under Details %+v", ops, detail)
	}
	if order[model.JobsView.FocusPane] != ui.JobsPaneOperations {
		t.Fatal("toggle moved focus off Operations")
	}
	h.HandleJobsViewKey(ctrlSpace)
	if _, back := h.jobsPaneLayout(); model.JobsOpsBottom || back[ui.JobsPaneOperations] != side[ui.JobsPaneOperations] {
		t.Fatal("second Ctrl-Space did not restore the side layout")
	}
}

// indexOnlyStub stands in for the fileindex backend: only its scheme and IndexOnly marker matter.
type indexOnlyStub struct{ fsbackend.Backend }

func (indexOnlyStub) Scheme() pathloc.Scheme { return pathloc.SchemeFileIndex }
func (indexOnlyStub) IndexOnly()             {}

func TestIndexTransferRefusal(t *testing.T) {
	if err := fsbackend.RegisterDefault(indexOnlyStub{}); err != nil {
		t.Fatal(err)
	}
	idx := pathloc.MustParse("fileindex:///orchard/harbor.txt")
	idxDir := pathloc.MustParse("fileindex:///cedar")
	disk := pathloc.MustParse("/tmp/meadow")
	cases := []struct {
		name  string
		typ   jobs.Type
		src   pathloc.Path
		dest  pathloc.Path
		allow bool
	}{
		{"copy out", jobs.TypeCopy, idx, disk, true},
		{"move inside", jobs.TypeMove, idx, idxDir, true},
		{"move out", jobs.TypeMove, idx, disk, false},
		{"copy in", jobs.TypeCopy, disk, idxDir, false},
		{"plain copy", jobs.TypeCopy, disk, disk, true},
	}
	for _, c := range cases {
		if got := indexTransferRefusal(c.typ, []pathloc.Path{c.src}, c.dest) == ""; got != c.allow {
			t.Errorf("%s: allowed = %v, want %v", c.name, got, c.allow)
		}
	}
}
