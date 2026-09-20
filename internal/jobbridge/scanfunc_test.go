package jobbridge

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
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

	scanFn := ScanFunc(config.JobsConfig{})
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

func writeScanTree(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "den"), 0o755); err != nil {
		t.Fatalf("mkdir den: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "otter.txt"), []byte("otter-content"), 0o644); err != nil {
		t.Fatalf("write otter: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "den", "fox.txt"), []byte("fox-content"), 0o644); err != nil {
		t.Fatalf("write fox: %v", err)
	}
}

// TestScanFuncBufferDeliveryPlanCompletesWithoutConsumer is the move-scan deadlock guard:
// BufferDeliveryPlan must let the delivery walk finish (and Done close) even when nobody
// is reading Items, so a move job can wait for enumeration before becoming runnable.
func TestScanFuncBufferDeliveryPlanCompletesWithoutConsumer(t *testing.T) {
	srcDir := t.TempDir()
	writeScanTree(t, srcDir)
	dstDir := t.TempDir()

	sources, err := pathloc.ParseAll([]string{srcDir})
	if err != nil {
		t.Fatalf("ParseAll sources: %v", err)
	}
	destination, err := pathloc.Parse(dstDir)
	if err != nil {
		t.Fatalf("Parse destination: %v", err)
	}

	scanFn := ScanFunc(config.JobsConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producer := scanFn(ctx, sources, destination, jobs.ScanWalkHooks{BufferDeliveryPlan: true})

	select {
	case <-producer.Done:
	case <-time.After(3 * time.Second):
		t.Fatal("Done must close without an Items consumer when BufferDeliveryPlan is set")
	}
	if err := producer.Err(); err != nil {
		t.Fatalf("delivery walk error = %v", err)
	}
	files, _, bytes := producer.Totals()
	if files != 4 {
		t.Fatalf("Totals files = %d, want 4 (stable)", files)
	}
	if bytes != int64(len("otter-content")+len("fox-content")) {
		t.Fatalf("Totals bytes = %d, want %d", bytes, len("otter-content")+len("fox-content"))
	}

	n := 0
	for range producer.Items {
		n++
	}
	if n != 4 {
		t.Fatalf("buffered Items = %d, want 4", n)
	}
}

// TestMoveJobDoesNotRenameWhileWalkPausedAfterRoot is the R04-003 end-to-end case: a
// TypeMove job must stay scanning (source path unchanged) while the walker is paused
// after emitting the root, then complete with stable totals once the walk is released.
func TestMoveJobDoesNotRenameWhileWalkPausedAfterRoot(t *testing.T) {
	srcParent := t.TempDir()
	src := filepath.Join(srcParent, "thicket")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	writeScanTree(t, src)
	dst := t.TempDir()

	entered := make(chan struct{})
	block := make(chan struct{})
	var once, releaseOnce sync.Once
	restore := localfs.SetWalkAfterDirHook(func(path string) error {
		if path != src {
			return nil
		}
		once.Do(func() { close(entered) })
		<-block
		return nil
	})
	defer restore()
	release := func() { releaseOnce.Do(func() { close(block) }) }
	defer release()

	s := jobs.NewState()
	cfg := config.Default()
	s.SetScanFunc(ScanFunc(cfg.Jobs))
	s.SetTransferFunc(TransferFunc(cfg.Operations, cfg.Jobs, nil))
	stop := make(chan struct{})
	defer close(stop)
	s.StartWorker(stop)

	srcLoc, err := pathloc.File(src)
	if err != nil {
		t.Fatalf("pathloc.File src: %v", err)
	}
	dstLoc, err := pathloc.File(dst)
	if err != nil {
		t.Fatalf("pathloc.File dst: %v", err)
	}

	done := make(chan string, 1)
	go func() {
		deadline := time.After(8 * time.Second)
		for {
			select {
			case ev := <-s.Events():
				s.ApplyEvent(ev)
				if ev.Type == jobs.EventCompleted || ev.Type == jobs.EventFailed {
					done <- string(ev.Type) + " " + ev.Error
					return
				}
			case <-deadline:
				done <- "timeout"
				return
			}
		}
	}()

	s.AddJob(&jobs.Job{
		ID:          "move-walk-race",
		Type:        jobs.TypeMove,
		Status:      jobs.StatusScanning,
		Sources:     []pathloc.Path{srcLoc},
		Destination: dstLoc,
	})

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for walk to pause after emitting root")
	}

	all := s.AllJobs()
	if len(all) != 1 || all[0].Status != jobs.StatusScanning {
		t.Fatalf("job status while walk paused = %+v, want StatusScanning", all)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must still exist while the walk is paused: %v", err)
	}
	still := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(still) {
		all = s.AllJobs()
		if len(all) != 1 || all[0].Status != jobs.StatusScanning {
			st := "none"
			if len(all) == 1 {
				st = string(all[0].Status)
			}
			t.Fatalf("job left StatusScanning while walk paused: status=%q", st)
		}
		if _, err := os.Stat(src); err != nil {
			t.Fatalf("source disappeared while walk paused: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	release()

	got := <-done
	if got != string(jobs.EventCompleted)+" " {
		t.Fatalf("terminal event = %q, want completed", got)
	}
	snap := s.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(snap))
	}
	if snap[0].TotalFiles != 4 {
		t.Fatalf("TotalFiles = %d, want 4 (stable)", snap[0].TotalFiles)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be gone after move: %v", err)
	}
	gotOtter, err := os.ReadFile(filepath.Join(dst, "thicket", "otter.txt"))
	if err != nil {
		t.Fatalf("read dest otter: %v", err)
	}
	if string(gotOtter) != "otter-content" {
		t.Fatalf("otter.txt = %q", gotOtter)
	}
}
