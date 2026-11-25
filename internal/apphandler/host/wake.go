package host

import (
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
)

// Superseder is an interrupt payload that makes an earlier one with the same key worthless (a
// newer snapshot of the same thing). A nil key supersedes nothing; keys must be comparable and
// should be of a type private to the payload, so payloads of different types cannot collide.
type Superseder interface{ SupersedeKey() any }

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
