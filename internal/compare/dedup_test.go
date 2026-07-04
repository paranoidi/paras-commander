package compare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/paranoidi/paras-commander/internal/fsbackend/file" // register default backend for HashFile
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// startDedup starts a session and returns a channel of its terminal/await snapshots
// (Done, Error, AwaitConfirm) in order. At most a couple are ever sent per run.
func startDedup(t *testing.T, dir string, threshold int) (*DedupSession, <-chan DedupSnapshot) {
	t.Helper()
	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}
	phases := make(chan DedupSnapshot, 8)
	sess := StartDedup(context.Background(), root, DedupOptions{
		HashWorkers:      2,
		ConfirmThreshold: threshold,
		OnUpdate: func(s DedupSnapshot) {
			switch s.Phase {
			case DedupDone, DedupError, DedupAwaitConfirm:
				phases <- s
			}
		},
	})
	return sess, phases
}

func TestDedupGroupsAndSizePrefilter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "unique.txt", "unique-content-xyz") // size 18, unique -> never hashed
	writeFile(t, dir, "a/dup1.txt", "duplicate!")         // size 10
	writeFile(t, dir, "b/dup2.txt", "duplicate!")         // size 10, identical -> duplicate pair
	writeFile(t, dir, "collide.txt", "0123456789")        // size 10, same size, different content

	sess, phases := startDedup(t, dir, 0)
	defer sess.Close()
	snap := <-phases

	if snap.Phase != DedupDone {
		t.Fatalf("phase = %v, want DedupDone", snap.Phase)
	}
	if snap.Walked != 4 {
		t.Fatalf("walked = %d, want 4", snap.Walked)
	}
	// Size prefilter: only the three size-10 files are candidates; unique.txt is skipped.
	if snap.HashTotal != 3 {
		t.Fatalf("hashTotal = %d, want 3 (unique-sized file must not be hashed)", snap.HashTotal)
	}
	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(snap.Groups))
	}
	g := snap.Groups[0]
	if len(g.Files) != 2 {
		t.Fatalf("group files = %d, want 2", len(g.Files))
	}
	if g.Files[0].Rel != "a/dup1.txt" || g.Files[1].Rel != "b/dup2.txt" {
		t.Fatalf("group files = %v, want sorted [a/dup1.txt b/dup2.txt]", g.Files)
	}
}

func TestDedupHashingPublishesCurrentPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "alpha/dup1.txt", "duplicate!")
	writeFile(t, dir, "beta/dup2.txt", "duplicate!")

	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}

	var hashing []DedupSnapshot
	sess := StartDedup(context.Background(), root, DedupOptions{
		HashWorkers: 1,
		OnUpdate: func(s DedupSnapshot) {
			if s.Phase == DedupHashing && s.Current != "" {
				hashing = append(hashing, s)
			}
		},
	})
	defer sess.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap := sess.Snapshot()
		if snap.Phase == DedupDone || snap.Phase == DedupError {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(hashing) == 0 {
		t.Fatal("expected hashing snapshots with a current path")
	}
	last := hashing[len(hashing)-1]
	if last.Hashed == 0 || last.HashTotal < 2 {
		t.Fatalf("hashing progress = %d/%d, want partial progress", last.Hashed, last.HashTotal)
	}
	if last.Current == "" {
		t.Fatal("hashing snapshot missing current path")
	}
	if last.Current != "alpha" && last.Current != "beta" {
		t.Fatalf("current = %q, want directory alpha or beta", last.Current)
	}
	if strings.Contains(last.Current, ".txt") {
		t.Fatalf("current = %q, want directory not filename", last.Current)
	}
}

func TestDedupHashingProgressRateLimited(t *testing.T) {
	dir := t.TempDir()
	const n = 12
	for i := range n {
		writeFile(t, dir, fmt.Sprintf("dir%d/dup.txt", i), "duplicate!")
	}

	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}

	var pathChanges int
	var prev string
	sess := StartDedup(context.Background(), root, DedupOptions{
		HashWorkers: 1,
		OnUpdate: func(s DedupSnapshot) {
			if s.Phase != DedupHashing || s.Current == "" {
				return
			}
			if s.Current != prev {
				pathChanges++
				prev = s.Current
			}
		},
	})
	defer sess.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p := sess.Snapshot().Phase; p == DedupDone || p == DedupError {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// n tiny files hash in well under one hashProgressInterval; the throttle should coalesce
	// them into far fewer visible path changes than one-per-file.
	if pathChanges >= n {
		t.Fatalf("path changes = %d, want < %d (rate-limited)", pathChanges, n)
	}
}

func TestDedupConfirmGate(t *testing.T) {
	dir := t.TempDir()
	// Three size-10 candidates; threshold 1 forces the confirmation gate.
	writeFile(t, dir, "a/dup1.txt", "duplicate!")
	writeFile(t, dir, "b/dup2.txt", "duplicate!")
	writeFile(t, dir, "collide.txt", "0123456789")

	sess, phases := startDedup(t, dir, 1)
	defer sess.Close()

	snap := <-phases
	if snap.Phase != DedupAwaitConfirm {
		t.Fatalf("phase = %v, want DedupAwaitConfirm", snap.Phase)
	}
	if snap.HashTotal != 3 {
		t.Fatalf("hashTotal = %d, want 3", snap.HashTotal)
	}

	sess.Confirm()
	got := <-phases
	if got.Phase != DedupDone {
		t.Fatalf("phase after confirm = %v, want DedupDone", got.Phase)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("groups after confirm = %d, want 1", len(got.Groups))
	}
}
