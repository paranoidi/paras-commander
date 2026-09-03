package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TestScanFuncTotalsKeepGrowingWithStalledConsumer proves the counting-walk fix: ScanFunc's
// Totals() must keep climbing even while the delivery side (Items) is stalled on a single
// unread item, simulating a slow/blocked transfer consumer (e.g. a large file mid-copy). Before
// the fix, the relay goroutine counted an item and then blocked trying to hand it to Items in
// lockstep, so Totals() froze the moment the consumer stopped reading — exactly the bug this
// test guards against.
func TestScanFuncTotalsKeepGrowingWithStalledConsumer(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// A handful of small files, named with random English words per repo test convention
	// (never real project filenames). All comfortably fit inside
	// config.DefaultPlanStreamBufferItems so the whole tree is enumerable well within the
	// buffer, making a stall on the delivery side purely about consumer behavior, not the
	// buffer's capacity.
	names := []string{"lantern", "harbor", "meadow", "compass", "ember"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(srcDir, n+".txt"), []byte("content-"+n), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}

	sources, err := pathloc.ParseAll([]string{srcDir})
	if err != nil {
		t.Fatalf("ParseAll sources: %v", err)
	}
	destination, err := pathloc.Parse(dstDir)
	if err != nil {
		t.Fatalf("Parse destination: %v", err)
	}

	scanFn := ScanFunc()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producer := scanFn(ctx, sources, destination, jobs.ScanWalkHooks{})

	// Read exactly one item and stop reading entirely — this is the stalled-consumer
	// simulation. A real transfer executor would be mid-copy of this item for a long time.
	select {
	case _, ok := <-producer.Items:
		if !ok {
			t.Fatal("Items closed before yielding a single item")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for first item")
	}

	// Do not read producer.Items again. With the old single-relay implementation, the relay
	// counts an item and only then tries to hand it off — so with the consumer stopped after
	// item 1, the relay pops+counts exactly one more item (the one it is now stuck trying to
	// deliver) and then freezes forever at files==2. The fix's independent counting walk has no
	// such handoff to block on, so it keeps enumerating the rest of the small tree regardless
	// and Totals() climbs well past that freeze point.
	deadline := time.After(3 * time.Second)
	for {
		files, _, _ := producer.Totals()
		if files > 2 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for Totals() to grow past the stalled-relay freeze point (files=2) with a stalled Items consumer; last files=%d", files)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
