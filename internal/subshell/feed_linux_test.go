//go:build linux

package subshell

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestWatchWinchResizeStopTerminatesWatcher(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	waitStable := func() int {
		t.Helper()
		var n, last int
		for i := 0; i < 20; i++ {
			runtime.GC()
			n = runtime.NumGoroutine()
			if i > 0 && n == last {
				return n
			}
			last = n
			time.Sleep(10 * time.Millisecond)
		}
		return n
	}

	baseline := waitStable()
	const rounds = 25
	for i := 0; i < rounds; i++ {
		stop := watchWinchResize(w, r)
		stop()
	}
	got := waitStable()
	if got > baseline+2 {
		t.Fatalf("goroutines after %d watch/stop cycles = %d, baseline %d (watchers leaked)", rounds, got, baseline)
	}
}
