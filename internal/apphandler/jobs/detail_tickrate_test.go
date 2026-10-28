package jobs

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type tickrateHost struct {
	jobsHostStub
	deltas *[]int
}

func (h tickrateHost) AdjustJobsThroughputChartColumnMS(d int) { *h.deltas = append(*h.deltas, d) }

// ActionFromKeyEvent mirrors the jobs overlay, which binds + / - to the rate-limit actions.
func (tickrateHost) ActionFromKeyEvent(ev *tcell.EventKey) string {
	switch ev.Rune() {
	case '+':
		return keymap.ActionJobsRateLimitIncrease
	case '-':
		return keymap.ActionJobsRateLimitDecrease
	}
	return ""
}

func TestDetailPaneTickrateKeys(t *testing.T) {
	t.Parallel()
	var got []int
	h := New(Deps{Host: tickrateHost{deltas: &got}, Model: &ui.Model{}, State: jobs.NewState(), Config: config.Default()})
	for _, r := range []rune{'+', '=', '-', '_', 'x'} {
		h.handleJobsDetailPaneKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	step := config.ThroughputChartColumnMSLiveStep
	want := []int{-step, -step, step, step}
	if len(got) != len(want) {
		t.Fatalf("deltas = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("deltas = %v, want %v", got, want)
		}
	}
}

// The overlay's rate-limit actions must not swallow + / - while Details has focus.
func TestDetailPaneTickrateKeysThroughViewKey(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 30)
	var got []int
	model := &ui.Model{}
	h := New(Deps{Host: tickrateHost{deltas: &got}, Screen: screen, Model: model, State: jobs.NewState(), Config: config.Default()})
	h.OpenJobsView()
	model.JobsView.FocusPane = 1 // no blocker: list, Details, ...
	if !h.DetailFocused() {
		t.Fatal("Details not focused")
	}
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModNone))
	h.HandleJobsViewKey(tcell.NewEventKey(tcell.KeyRune, '-', tcell.ModNone))
	step := config.ThroughputChartColumnMSLiveStep
	if len(got) != 2 || got[0] != -step || got[1] != step {
		t.Fatalf("deltas = %v, want [%d %d]", got, -step, step)
	}
}
