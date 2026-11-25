package app

import (
	"reflect"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
)

// losslessScreen makes PostEvent lossless. tcell's PostEvent drops the event when its queue is full
// (256 slots on a terminal, 10 on a SimulationScreen), and PostEventWait drops it while the screen
// is suspended; every payload posted here is a result or wake nothing sends again, and a
// coalescer whose wake was dropped stays pending forever. Events that do not fit wait in overflow
// and move into the queue as the event loop polls. Neither method blocks, so both are safe from
// the event loop itself.
//
// The overflow is bounded for repeating payloads, which pile up while the event loop is not
// polling (a full-screen subshell can last hours): a zero-value interrupt payload (a pure wake
// such as a tick) is dropped when the overflow holds one of the same type, and a host.Superseder
// replaces the overflow event with an equal key. Other payloads are never merged.
type losslessScreen struct {
	tcell.Screen
	mu       sync.Mutex
	overflow []tcell.Event
}

func newLosslessScreen(inner tcell.Screen) *losslessScreen {
	return &losslessScreen{Screen: inner}
}

// refill moves overflow events, in order, into the inner queue until it is full. Caller holds mu.
func (s *losslessScreen) refill() {
	n := 0
	for n < len(s.overflow) && s.Screen.PostEvent(s.overflow[n]) == nil {
		n++
	}
	clear(s.overflow[:n])
	s.overflow = s.overflow[n:]
}

// absorbInOverflow reports whether ev needs no slot of its own in the overflow: it is a zero-value
// payload equal to a queued one, or a superseder that replaced the queued event with its key.
func (s *losslessScreen) absorbInOverflow(ev tcell.Event) bool {
	ie, ok := ev.(*tcell.EventInterrupt)
	if !ok || ie.Data() == nil {
		return false
	}
	d := ie.Data()
	var key any
	if sp, ok := d.(host.Superseder); ok {
		key = sp.SupersedeKey()
	}
	zero := reflect.ValueOf(d).IsZero()
	for i, o := range s.overflow {
		oi, ok := o.(*tcell.EventInterrupt)
		if !ok || oi.Data() == nil {
			continue
		}
		od := oi.Data()
		if key != nil {
			if sp, ok := od.(host.Superseder); ok && sp.SupersedeKey() == key {
				s.overflow[i] = ev
				return true
			}
		} else if zero && reflect.TypeOf(od) == reflect.TypeOf(d) && reflect.ValueOf(od).IsZero() {
			return true
		}
	}
	return false
}

// PostEvent always returns nil.
func (s *losslessScreen) PostEvent(ev tcell.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refill()
	if len(s.overflow) == 0 && s.Screen.PostEvent(ev) == nil {
		return nil
	}
	// ponytail: linear scan; the overflow stays short because repeating payloads are merged here.
	if !s.absorbInOverflow(ev) {
		s.overflow = append(s.overflow, ev)
	}
	return nil
}

// PostEventWait is PostEvent: the inner one drops the event while the screen is suspended.
func (s *losslessScreen) PostEventWait(ev tcell.Event) { _ = s.PostEvent(ev) }

// PollEvent returns the next event, then refills the queue from the overflow. An event is in the
// overflow only because the inner queue was full when it was posted, and a refill follows every
// poll, so an overflow event could stay stuck only if the inner queue were empty, which that
// refill prevents: the queue is empty only once the overflow is.
func (s *losslessScreen) PollEvent() tcell.Event {
	ev := s.Screen.PollEvent()
	s.mu.Lock()
	s.refill()
	s.mu.Unlock()
	return ev
}

func (s *losslessScreen) HasPendingEvent() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Screen.HasPendingEvent() || len(s.overflow) > 0
}
