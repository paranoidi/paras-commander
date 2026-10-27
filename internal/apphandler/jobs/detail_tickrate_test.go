package jobs

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type tickrateHost struct {
	jobsHostStub
	deltas *[]int
}

func (h tickrateHost) AdjustJobsThroughputChartColumnMS(d int) { *h.deltas = append(*h.deltas, d) }

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
