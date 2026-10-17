package commands

import (
	"context"
	"time"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// liveStreamInterval is how often StreamLive publishes partial output.
// ponytail: fixed at 10 Hz; make it configurable only if a very chatty command needs less repainting.
const liveStreamInterval = 100 * time.Millisecond

// StreamLive publishes live's partial output to CommandsList row idx and the running output
// dialog for rowID until stop is called or ctx is done. stop waits for the goroutine, so the
// caller's final completion patch always wins over a late tick. Call stop before that patch.
func (h *Handler) StreamLive(ctx context.Context, idx int, rowID string, live *cmdrun.Live) (stop func()) {
	done := make(chan struct{})
	quit := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(liveStreamInterval)
		defer t.Stop()
		lastOut, lastErr := -1, -1
		for {
			select {
			case <-quit:
				return
			case <-ctx.Done():
				return
			case <-t.C:
			}
			nOut, nErr := live.Stdout.Written(), live.Stderr.Written()
			if nOut == lastOut && nErr == lastErr {
				continue
			}
			lastOut, lastErr = nOut, nErr
			stdout := string(live.Stdout.Bytes())
			stderr := string(cmdrun.StripNoise(live.Stderr.Bytes()))
			h.PatchEntry(idx, func(e *ui.CommandRunEntry) {
				e.Stdout, e.Stderr = stdout, stderr
			})
			h.PostWake(WakePayload{OutputDialogLive: &OutputDialogLive{RunID: rowID, Stdout: stdout, Stderr: stderr}})
		}
	}()
	return func() {
		close(quit)
		<-done
	}
}
