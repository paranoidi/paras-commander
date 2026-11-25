package app

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/commands"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

type numbered struct{ n int }
type tickWake struct{}
type zeroPair struct{ A, B int }
type dismiss struct{ open bool }

func newLossless(t *testing.T) (*losslessScreen, tcell.SimulationScreen) {
	t.Helper()
	sim := uitest.Screen(t, 20, 5)
	return newLosslessScreen(sim), sim
}

// pollData polls n events within a timeout and returns their payloads.
func pollData(t *testing.T, s tcell.Screen, n int) []any {
	t.Helper()
	got := make(chan any, n)
	go func() {
		for i := 0; i < n; i++ {
			ev, ok := s.PollEvent().(*tcell.EventInterrupt)
			if !ok {
				got <- nil
				continue
			}
			got <- ev.Data()
		}
	}()
	var out []any
	for len(out) < n {
		select {
		case d := <-got:
			out = append(out, d)
		case <-time.After(3 * time.Second):
			t.Fatalf("polled %d of %d events", len(out), n)
		}
	}
	return out
}

func TestLosslessScreenDeliversEverythingPastTheQueue(t *testing.T) {
	s, _ := newLossless(t)
	const total = 50 // the simulation queue holds 10
	for i := 0; i < total; i++ {
		if err := s.PostEvent(tcell.NewEventInterrupt(numbered{i})); err != nil {
			t.Fatalf("PostEvent(%d) = %v", i, err)
		}
	}
	for i, d := range pollData(t, s, total) {
		if d != (numbered{i}) {
			t.Fatalf("event %d = %v, want %v", i, d, numbered{i})
		}
	}
	if s.HasPendingEvent() {
		t.Fatal("events left after polling everything")
	}
}

func TestLosslessScreenDedupesZeroValuePayloadsInTheOverflow(t *testing.T) {
	s, _ := newLossless(t)
	for i := 0; i < 10; i++ { // fill the queue
		_ = s.PostEvent(tcell.NewEventInterrupt(numbered{i}))
	}
	for i := 0; i < 1000; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(tickWake{}))
		_ = s.PostEvent(tcell.NewEventInterrupt(zeroPair{}))
	}
	if len(s.overflow) != 2 {
		t.Fatalf("overflow holds %d events, want one per zero-value type", len(s.overflow))
	}
	for _, p := range []dismiss{{true}, {true}, {true}} { // equal, not zero: all kept
		_ = s.PostEvent(tcell.NewEventInterrupt(p))
	}
	_ = s.PostEvent(tcell.NewEventInterrupt(nil))
	_ = s.PostEvent(tcell.NewEventInterrupt(nil))
	got := pollData(t, s, 10+2+3+2)
	want := []any{tickWake{}, zeroPair{}, dismiss{true}, dismiss{true}, dismiss{true}, nil, nil}
	for i, w := range want {
		if got[10+i] != w {
			t.Fatalf("event %d = %v, want %v", 10+i, got[10+i], w)
		}
	}
	if s.HasPendingEvent() {
		t.Fatal("events left after polling everything")
	}
}

type liveKey struct{ run string }
type liveSnap struct {
	run  string
	text int
}

func (p liveSnap) SupersedeKey() any { return liveKey{p.run} }

func TestLosslessScreenSupersedesSnapshotsOfTheSameKey(t *testing.T) {
	s, _ := newLossless(t)
	for i := 0; i < 10; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(numbered{i}))
	}
	for i := 1; i <= 1000; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(liveSnap{"A", i}))
		if i == 1 {
			_ = s.PostEvent(tcell.NewEventInterrupt(liveSnap{"B", 7}))
		}
	}
	if len(s.overflow) != 2 {
		t.Fatalf("overflow holds %d events, want 2", len(s.overflow))
	}
	got := pollData(t, s, 12)
	if got[10] != (liveSnap{"A", 1000}) || got[11] != (liveSnap{"B", 7}) {
		t.Fatalf("delivered %v and %v, want the last A snapshot and B's", got[10], got[11])
	}
}

func TestLosslessScreenMergesRealRepeatingPayloads(t *testing.T) {
	s, _ := newLossless(t)
	for i := 0; i < 10; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(numbered{i}))
	}
	live := func(run, out string) commands.WakePayload {
		return commands.WakePayload{OutputDialogLive: &commands.OutputDialogLive{RunID: run, Stdout: out}}
	}
	for i := 0; i < 100; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(live("A", strconv.Itoa(i))))
		_ = s.PostEvent(tcell.NewEventInterrupt(commands.WakePayload{}))
		_ = s.PostEvent(tcell.NewEventInterrupt(statusCommandResultPayload{Gen: 1, Index: 2, Text: strconv.Itoa(i)}))
		_ = s.PostEvent(tcell.NewEventInterrupt(statusCommandResultPayload{Gen: 1, Index: 3, Text: "x"}))
		_ = s.PostEvent(tcell.NewEventInterrupt(volumeSpaceRefreshPayload{PanelID: 1, Avail: uint64(i)}))
	}
	if len(s.overflow) != 5 { // live A, empty wake, two status runners, one panel's volume
		t.Fatalf("overflow holds %d events, want 5", len(s.overflow))
	}
	got := pollData(t, s, 15)
	if w, ok := got[10].(commands.WakePayload); !ok || w.OutputDialogLive.Stdout != "99" {
		t.Fatalf("delivered %+v, want the last live snapshot", got[10])
	}
	if got[12] != (statusCommandResultPayload{Gen: 1, Index: 2, Text: "99"}) {
		t.Fatalf("delivered %+v, want the last status result", got[12])
	}
}

func TestLosslessScreenConcurrentPosters(t *testing.T) {
	s, _ := newLossless(t)
	const posters, each = 8, 100
	var wg sync.WaitGroup
	for p := 0; p < posters; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				_ = s.PostEvent(tcell.NewEventInterrupt(numbered{p*each + i}))
			}
		}()
	}
	seen := map[int]bool{}
	for _, d := range pollData(t, s, posters*each) {
		seen[d.(numbered).n] = true
	}
	wg.Wait()
	if len(seen) != posters*each {
		t.Fatalf("got %d distinct events, want %d", len(seen), posters*each)
	}
}

func TestWakeCoalescerSurvivesAFullQueue(t *testing.T) {
	s, _ := newLossless(t)
	for i := 0; i < 10; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(numbered{i}))
	}
	var w host.WakeCoalescer
	w.Post(s, tickWake{}) // hits the full queue
	w.Post(s, tickWake{}) // coalesced
	got := pollData(t, s, 11)
	if got[10] != (tickWake{}) {
		t.Fatalf("last event = %v, want the wake", got[10])
	}
	if !w.Take() {
		t.Fatal("wake should be pending until taken")
	}
	w.Post(s, tickWake{})
	if pollData(t, s, 1)[0] != (tickWake{}) {
		t.Fatal("wake after Take was not delivered")
	}
}

func TestLosslessScreenHasPendingEvent(t *testing.T) {
	s, sim := newLossless(t)
	if s.HasPendingEvent() {
		t.Fatal("pending on an empty screen")
	}
	for i := 0; i < 11; i++ {
		_ = s.PostEvent(tcell.NewEventInterrupt(numbered{i}))
	}
	for i := 0; i < 10; i++ { // empty the raw queue without the refill a poll through s does
		sim.PollEvent()
	}
	if len(s.overflow) != 1 || sim.HasPendingEvent() {
		t.Fatalf("setup: overflow %d, raw pending %v, want 1 and false", len(s.overflow), sim.HasPendingEvent())
	}
	if !s.HasPendingEvent() {
		t.Fatal("overflow event not reported pending")
	}
}
