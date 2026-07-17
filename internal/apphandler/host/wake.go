package host

import (
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
)

// WakeCoalescer posts at most one interrupt event until Take clears the pending flag.
type WakeCoalescer struct {
	pending atomic.Bool
}

// Post queues a single EventInterrupt with payload if none is already pending.
func (w *WakeCoalescer) Post(screen tcell.Screen, payload any) {
	if screen == nil {
		return
	}
	if w.pending.Swap(true) {
		return
	}
	_ = screen.PostEvent(tcell.NewEventInterrupt(payload))
}

// Take reports whether a wake was pending and clears the flag (call from the event loop).
func (w *WakeCoalescer) Take() bool {
	return w.pending.Swap(false)
}
