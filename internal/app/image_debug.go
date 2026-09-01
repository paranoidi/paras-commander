package app

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Image-overlay tracing, off unless PC_IMAGE_LOG names a file. It records what this process does
// to the terminal image — every plan decision, lock/unlock, blank and payload write — with wall
// clock timestamps, so a log can be lined up against a byte capture of what tmux sent the terminal
// (graphics-testing/record-live.sh). Without it, a capture shows only the merged result of tcell's
// frames, our direct tty writes and tmux's own redraws, and there is no way to tell which side
// wrote what.
var imageLog = func() *os.File {
	path := os.Getenv("PC_IMAGE_LOG")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil
	}
	return f
}()

var imageLogMu sync.Mutex

// imageTrace writes one timestamped line. The timestamp is the same clock record-live.sh notes as
// its start epoch, so entries can be placed on the capture's timeline directly.
func imageTrace(format string, args ...any) {
	if imageLog == nil {
		return
	}
	imageLogMu.Lock()
	defer imageLogMu.Unlock()
	_, _ = fmt.Fprintf(imageLog, "%d.%06d %s\n", time.Now().Unix(), time.Now().Nanosecond()/1000,
		fmt.Sprintf(format, args...))
}
