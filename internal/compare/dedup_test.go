package compare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
func startDedup(t *testing.T, dir string, confirmHashBytes int64) (*DedupSession, <-chan DedupSnapshot) {
	t.Helper()
	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}
	phases := make(chan DedupSnapshot, 8)
	sess := StartDedup(context.Background(), root, DedupOptions{
		HashWorkers:      2,
		ConfirmHashBytes: confirmHashBytes,
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

func TestDedupWalkingPublishesFileCount(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "alpha.txt", "a")
	writeFile(t, dir, "beta.txt", "b")

	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}

	var walking []DedupSnapshot
	sess := StartDedup(context.Background(), root, DedupOptions{
		OnUpdate: func(s DedupSnapshot) {
			if s.Phase == DedupWalking && s.Walked > 0 {
				walking = append(walking, s)
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

	if len(walking) == 0 {
		t.Fatal("expected walking snapshots with file count")
	}
	if walking[0].Walked < 1 {
		t.Fatalf("first walking update = %d, want >= 1", walking[0].Walked)
	}
	if got := sess.Snapshot().Walked; got != 2 {
		t.Fatalf("walked = %d, want 2", got)
	}
}

func TestDedupWalkingProgressRateLimited(t *testing.T) {
	dir := t.TempDir()
	const n = 40
	for i := range n {
		writeFile(t, dir, fmt.Sprintf("dir%d/file.txt", i), "x")
	}

	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}

	var updates int
	sess := StartDedup(context.Background(), root, DedupOptions{
		OnUpdate: func(s DedupSnapshot) {
			if s.Phase == DedupWalking && s.Walked > 0 {
				updates++
			}
		},
	})
	defer sess.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p := sess.Snapshot().Phase; p != DedupWalking {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if updates >= n {
		t.Fatalf("walking updates = %d, want < %d (rate-limited)", updates, n)
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
	if last.HashedBytes == 0 || last.HashTotal < 2 {
		t.Fatalf("hashing progress = %d bytes of %d files, want partial progress", last.HashedBytes, last.HashTotal)
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

	// n tiny files hash in well under one dedupProgressInterval; the throttle should coalesce
	// them into far fewer visible path changes than one-per-file.
	if pathChanges >= n {
		t.Fatalf("path changes = %d, want < %d (rate-limited)", pathChanges, n)
	}
}

func TestHashFileReportsProgress(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("orchard meadow lantern ", 8) // 184 bytes
	writeFile(t, dir, "voyage.txt", content)
	loc, err := pathloc.File(filepath.Join(dir, "voyage.txt"))
	if err != nil {
		t.Fatal(err)
	}

	var reads []int64
	buf := make([]byte, 32) // force multiple reads
	if _, err := HashFile(context.Background(), loc, buf, 0, func(read int64) {
		reads = append(reads, read)
	}); err != nil {
		t.Fatal(err)
	}

	if len(reads) < 2 {
		t.Fatalf("progress callbacks = %d, want several with a 32-byte buffer", len(reads))
	}
	for i := 1; i < len(reads); i++ {
		if reads[i] <= reads[i-1] {
			t.Fatalf("progress not strictly increasing: %v", reads)
		}
	}
	if got, want := reads[len(reads)-1], int64(len(content)); got != want {
		t.Fatalf("final progress = %d, want file size %d", got, want)
	}
}

func TestDedupHashedBytesMonotonic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/dup1.txt", "duplicate!")
	writeFile(t, dir, "b/dup2.txt", "duplicate!")
	writeFile(t, dir, "c/dup3.txt", "duplicate!")

	root, err := pathloc.File(dir)
	if err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var seen []DedupSnapshot
	sess := StartDedup(context.Background(), root, DedupOptions{
		HashWorkers: 2,
		ReadBuffer:  make([]byte, 4),
		OnUpdate: func(s DedupSnapshot) {
			if s.Phase == DedupHashing || s.Phase == DedupDone {
				mu.Lock()
				seen = append(seen, s)
				mu.Unlock()
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

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("expected hashing/done snapshots")
	}
	for i := 1; i < len(seen); i++ {
		if seen[i].HashedBytes < seen[i-1].HashedBytes {
			t.Fatalf("HashedBytes regressed at %d: %d -> %d", i, seen[i-1].HashedBytes, seen[i].HashedBytes)
		}
	}
	last := seen[len(seen)-1]
	if last.Phase != DedupDone {
		t.Fatalf("last phase = %v, want DedupDone", last.Phase)
	}
	if last.HashedBytes != last.HashBytesTotal {
		t.Fatalf("done HashedBytes = %d, want HashBytesTotal %d", last.HashedBytes, last.HashBytesTotal)
	}
}

func TestDedupCurrentFileAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("harbor lantern meadow ", 6) // 132 bytes
	writeFile(t, dir, "alpha/voyage.dat", content)
	writeFile(t, dir, "beta/voyage.dat", content)

	run := func(t *testing.T, fileProgressBytes int64) []DedupSnapshot {
		t.Helper()
		root, err := pathloc.File(dir)
		if err != nil {
			t.Fatal(err)
		}
		var mu sync.Mutex
		var hashing []DedupSnapshot
		sess := StartDedup(context.Background(), root, DedupOptions{
			HashWorkers:       1,
			ReadBuffer:        make([]byte, 8), // several callbacks per file before completion
			FileProgressBytes: fileProgressBytes,
			OnUpdate: func(s DedupSnapshot) {
				if s.Phase == DedupHashing {
					mu.Lock()
					hashing = append(hashing, s)
					mu.Unlock()
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
		mu.Lock()
		defer mu.Unlock()
		return append([]DedupSnapshot(nil), hashing...)
	}

	withFile := run(t, 1)
	found := false
	for _, s := range withFile {
		if s.CurrentFile == "" {
			continue
		}
		found = true
		if strings.Contains(s.CurrentFile, "/") {
			t.Fatalf("CurrentFile = %q, want filename without path", s.CurrentFile)
		}
		if s.CurrentFile != "voyage.dat" {
			t.Fatalf("CurrentFile = %q, want voyage.dat", s.CurrentFile)
		}
		if s.CurrentFileSize != int64(len(content)) {
			t.Fatalf("CurrentFileSize = %d, want %d", s.CurrentFileSize, len(content))
		}
		if s.CurrentFileDone <= 0 || s.CurrentFileDone > s.CurrentFileSize {
			t.Fatalf("CurrentFileDone = %d, want within (0, %d]", s.CurrentFileDone, s.CurrentFileSize)
		}
	}
	if !found {
		t.Fatal("expected a hashing snapshot with CurrentFile above threshold")
	}

	for _, s := range run(t, 0) {
		if s.CurrentFile != "" {
			t.Fatalf("CurrentFile = %q with threshold disabled, want empty", s.CurrentFile)
		}
	}
}

func TestDedupConfirmGate(t *testing.T) {
	dir := t.TempDir()
	// Three size-10 candidates (30 bytes total); threshold 1 byte forces the confirmation gate.
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
	if snap.HashBytesTotal != 30 {
		t.Fatalf("hashBytesTotal = %d, want 30", snap.HashBytesTotal)
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

func TestDedupConfirmGateSkippedUnderByteThreshold(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/dup1.txt", "duplicate!")
	writeFile(t, dir, "b/dup2.txt", "duplicate!")

	sess, phases := startDedup(t, dir, 30)
	defer sess.Close()

	snap := <-phases
	if snap.Phase != DedupDone {
		t.Fatalf("phase = %v, want DedupDone (20 bytes <= 30 byte threshold)", snap.Phase)
	}
}
