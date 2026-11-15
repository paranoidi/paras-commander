package jobs

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// A steady stream of progress events must keep waking the UI once per window instead of
// postponing the wake until the stream pauses.
func TestProgressWakeIsThrottledNotDebounced(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	cfg := config.Default()
	cfg.Jobs.ProgressUIWakeDebounceMS = 40
	h := New(Deps{Host: jobsHostStub{}, Screen: screen, Model: &ui.Model{}, State: jobs.NewState(), Config: cfg})

	var wakes atomic.Int32
	go func() {
		for {
			ev := screen.PollEvent()
			if ev == nil {
				return
			}
			if _, ok := ev.(*tcell.EventInterrupt); ok {
				wakes.Add(1)
			}
		}
	}()

	h.OnJobEmitted(jobs.Event{Type: jobs.EventProgress})
	h.wakeMu.Lock()
	first := h.wakeTimer
	h.wakeMu.Unlock()
	if first == nil {
		t.Fatal("first progress event did not arm the wake timer")
	}
	deadline := time.Now().Add(170 * time.Millisecond)
	for time.Now().Before(deadline) {
		h.OnJobEmitted(jobs.Event{Type: jobs.EventProgress})
		time.Sleep(time.Millisecond)
	}
	if n := wakes.Load(); n < 2 {
		t.Fatalf("wakes during a continuous progress stream = %d, want >= 2", n)
	}
	h.StopWakeTimer()
	h.wakeMu.Lock()
	defer h.wakeMu.Unlock()
	if h.wakeTimer != nil {
		t.Fatal("StopWakeTimer left the timer armed")
	}
}
