package compare

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func mustFilePath(t *testing.T, dir string) pathloc.Path {
	t.Helper()
	p, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func waitSessionClose(t *testing.T, sess *Session) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		sess.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Session.Close did not return")
	}
}

func gateSkip(started chan struct{}, released <-chan struct{}) func(string) bool {
	var once sync.Once
	return func(path string) bool {
		if filepath.Base(path) != "gate" {
			return false
		}
		once.Do(func() { close(started) })
		<-released
		return false
	}
}

func startGatedCompare(t *testing.T, ctx context.Context, blockPrimary, blockSecondary bool) (*Session, chan struct{}, chan struct{}) {
	t.Helper()
	primaryDir := t.TempDir()
	secondaryDir := t.TempDir()
	writeFile(t, primaryDir, "alpha.txt", "alpha")
	writeFile(t, secondaryDir, "beta.txt", "beta")
	if blockPrimary {
		writeFile(t, primaryDir, "gate/hold.txt", "hold-primary")
	}
	if blockSecondary {
		writeFile(t, secondaryDir, "gate/hold.txt", "hold-secondary")
	}

	started := make(chan struct{})
	released := make(chan struct{})
	sess := Start(ctx, mustFilePath(t, primaryDir), mustFilePath(t, secondaryDir), Options{
		Walk: WalkOptions{ShouldSkipDir: gateSkip(started, released)},
	})
	return sess, started, released
}

func cancelGatedWalk(t *testing.T, cancel context.CancelFunc, started, released chan struct{}, sess *Session) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("walk did not reach gate dir")
	}
	cancel()
	close(released)
	waitSessionClose(t, sess)
}

func TestSessionCancelDuringPrimaryWalkIsPhaseCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sess, started, released := startGatedCompare(t, ctx, true, false)
	cancelGatedWalk(t, cancel, started, released, sess)

	snap := sess.Snapshot()
	if snap.Phase != PhaseCanceled {
		t.Fatalf("phase = %v (%s), want PhaseCanceled", snap.Phase, snap.Err)
	}
}

func TestSessionPreCanceledContextEndsAsPhaseCanceled(t *testing.T) {
	primaryDir := t.TempDir()
	secondaryDir := t.TempDir()
	writeFile(t, primaryDir, "alpha.txt", "alpha")
	writeFile(t, secondaryDir, "beta.txt", "beta")
	primary := mustFilePath(t, primaryDir)
	secondary := mustFilePath(t, secondaryDir)

	// Pre-cancel so both walks return context.Canceled into the result
	// channels; run() may receive those errors instead of ctx.Done().
	for i := 0; i < 200; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		sess := Start(ctx, primary, secondary, Options{})
		deadline := time.Now().Add(time.Second)
		var snap Snapshot
		for time.Now().Before(deadline) {
			snap = sess.Snapshot()
			if snap.Phase == PhaseError || snap.Phase == PhaseCanceled || snap.Phase == PhaseDone {
				break
			}
			time.Sleep(time.Microsecond)
		}
		sess.Close()
		if snap.Phase != PhaseCanceled {
			t.Fatalf("iteration %d: phase = %v (%s), want PhaseCanceled", i, snap.Phase, snap.Err)
		}
	}
}

func TestSessionCancelDuringSecondaryWalkIsPhaseCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sess, started, released := startGatedCompare(t, ctx, false, true)
	cancelGatedWalk(t, cancel, started, released, sess)

	snap := sess.Snapshot()
	if snap.Phase != PhaseCanceled {
		t.Fatalf("phase = %v (%s), want PhaseCanceled", snap.Phase, snap.Err)
	}
}

func TestSessionCloseReturnsWhileWalking(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sess, started, released := startGatedCompare(t, ctx, true, true)
	cancelGatedWalk(t, cancel, started, released, sess)
}

func TestSessionWalkErrorIsPhaseError(t *testing.T) {
	secondaryDir := t.TempDir()
	writeFile(t, secondaryDir, "beta.txt", "beta")
	missing := filepath.Join(t.TempDir(), "missing-root")
	sess := Start(context.Background(), mustFilePath(t, missing), mustFilePath(t, secondaryDir), Options{})
	t.Cleanup(func() { sess.Close() })

	deadline := time.Now().Add(2 * time.Second)
	var snap Snapshot
	for time.Now().Before(deadline) {
		snap = sess.Snapshot()
		if snap.Phase == PhaseError || snap.Phase == PhaseCanceled || snap.Phase == PhaseDone {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if snap.Phase != PhaseError {
		t.Fatalf("phase = %v (%s), want PhaseError", snap.Phase, snap.Err)
	}
	if snap.Err == "" {
		t.Fatal("PhaseError snapshot missing Err")
	}
}

func TestSessionCloseOnIdleRootsReturns(t *testing.T) {
	primaryDir := t.TempDir()
	secondaryDir := t.TempDir()
	writeFile(t, primaryDir, "alpha.txt", "alpha")
	writeFile(t, secondaryDir, "beta.txt", "beta")
	sess := Start(context.Background(), mustFilePath(t, primaryDir), mustFilePath(t, secondaryDir), Options{})
	waitSessionClose(t, sess)
	if _, err := os.Stat(primaryDir); err != nil {
		t.Fatal(err)
	}
}
