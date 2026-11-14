package dlg

import (
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TooltipWakePayload is the EventInterrupt payload that asks the app for a plain redraw once a
// pending tooltip's delay has elapsed.
type TooltipWakePayload struct{}

// tooltipKey identifies one tooltip: the dialog frame, the widget it explains and its text.
// Moving focus to another widget, or opening another dialog, changes it.
type tooltipKey struct {
	frame, widget Rect
	text          string
}

// tooltipGate debounces tooltips: a tooltip is painted only once the same one has been asked for
// across redraws for the delay. Only the UI goroutine calls it; the wake-up timer callback just
// posts an event, so the mutex guards nothing but parallel tests.
type tooltipGate struct {
	mu    sync.Mutex
	key   tooltipKey
	since time.Time
	set   bool
	timer *time.Timer
}

// theTooltipGate is the one gate every tooltip paint goes through (PaintTooltipFor).
var theTooltipGate tooltipGate

// due reports whether the tooltip for key may be painted at now. Until then it arms one wake-up
// (wake, called after the remaining time) and replaces any wake-up armed for another key.
func (g *tooltipGate) due(key tooltipKey, now time.Time, delay time.Duration, wake func()) bool {
	if delay <= 0 {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.set || g.key != key {
		g.stopLocked()
		g.key, g.since, g.set = key, now, true
	}
	left := delay - now.Sub(g.since)
	if left <= 0 {
		return true
	}
	if g.timer == nil {
		g.timer = time.AfterFunc(left, wake)
	}
	return false
}

// clear forgets the pending or shown tooltip of the dialog frame, so focusing a tooltip widget
// again waits again. A tooltip of another frame is left alone.
func (g *tooltipGate) clear(frame Rect) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.set && g.key.frame == frame {
		g.stopLocked()
		g.set = false
	}
}

func (g *tooltipGate) stopLocked() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}

// wakeScreen posts the redraw request that the gate's timer fires.
func wakeScreen(screen tcell.Screen) func() {
	return func() { _ = screen.PostEvent(tcell.NewEventInterrupt(TooltipWakePayload{})) }
}
