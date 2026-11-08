package app

import (
	"slices"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fschange"
)

// metaFSWakePayload is posted (throttled) after filesystem changes were reported; the batches wait
// in asyncWakeQueues.metaFS.
type metaFSWakePayload struct{}

// metaFSSink carries filesystem changes (ADR 0023) to the meta columns on the event loop. app is
// set before the job worker starts; until then changes are ignored.
type metaFSSink struct {
	app *App
}

// Apply queues a batch and, for the first one of a window, schedules the wake. Dialog operations
// report from the UI goroutine, so the post happens on a timer goroutine: posting here could block
// on the event queue that only this goroutine drains.
func (s *metaFSSink) Apply(batch []fschange.Change) {
	a := s.app
	if a == nil || !asyncWakes(a).metaFS.push(slices.Clone(batch)) {
		return
	}
	time.AfterFunc(config.MetaFSChangeDebounce, func() {
		select {
		case <-a.jobStopCh:
			return
		default:
		}
		if a.screen != nil {
			postGuaranteedWake(a.screen, metaFSWakePayload{})
		}
	})
}

// handleMetaFSWake invalidates the meta values of the paths that changed.
func (a *App) handleMetaFSWake() {
	a.metaCtrl.ApplyFSChanges(slices.Concat(asyncWakes(a).metaFS.drain()...))
}
