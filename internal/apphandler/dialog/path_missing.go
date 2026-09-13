package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// PathsMissingPayload wakes PollEvent with the result of a background missing-path scan
// started by StartPathsMissingScan.
type PathsMissingPayload struct {
	// Target identifies which dialog the scan was for ("picker" | "history" | "pin") — Run()
	// routes on this to the right Apply* function.
	Target string
	Gen    uint64
	// Missing is keyed by the path string handed to StartPathsMissingScan, so a caller that
	// re-orders or trims its own path slice between Start and Apply can't misapply a result
	// to the wrong index.
	Missing map[string]bool
}

// StartPathsMissingScan stats paths off the UI goroutine and posts a PathsMissingPayload once
// done. sftp:// paths are skipped (left absent from Missing, i.e. not-missing) — dialing SSH
// on demand from a dialog-open scan is exactly the blocking behavior this exists to avoid.
func StartPathsMissingScan(screen tcell.Screen, target string, gen uint64, panelPath, home string, paths []string) {
	if screen == nil {
		return
	}
	go func() {
		missing := make(map[string]bool, len(paths))
		for _, p := range paths {
			if _, ok := missing[p]; ok {
				continue
			}
			// ponytail: sftp:// existence checks dial SSH (pool.withSFTP); never do that from
			// a background scan triggered just by opening a dialog. Left absent => not missing.
			if strings.HasPrefix(p, "sftp://") {
				continue
			}
			missing[p] = PathEntryMissing(panelPath, home, p)
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(PathsMissingPayload{Target: target, Gen: gen, Missing: missing}))
	}()
}
