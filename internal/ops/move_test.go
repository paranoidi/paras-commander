package ops

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func skipAllResolver() ConflictResolver {
	return func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = src
		_ = dst
		_ = facts
		return false, nil
	}
}

func overwriteAllResolver() ConflictResolver {
	return func(src, dst string, facts FileConflictFacts) (bool, error) {
		_ = src
		_ = dst
		_ = facts
		return true, nil
	}
}

func TestMoveCopyPhaseSkipAllPreservesSkippedSources(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	onlySrc := filepath.Join(srcDir, "only-from-a.txt")
	conflictSrc := filepath.Join(srcDir, "conflict-at-root.txt")
	if err := os.WriteFile(onlySrc, []byte("content-from-a-only"), 0o644); err != nil {
		t.Fatalf("write only source: %v", err)
	}
	if err := os.WriteFile(conflictSrc, []byte("content-from-a-conflict"), 0o644); err != nil {
		t.Fatalf("write conflict source: %v", err)
	}
	conflictDst := filepath.Join(dstDir, "conflict-at-root.txt")
	if err := os.WriteFile(conflictDst, []byte("content-from-b-conflict"), 0o644); err != nil {
		t.Fatalf("write conflict dest: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	done, doneBytes, err := executeMoveCopyPhase(
		context.Background(),
		nil,
		MustPaths(onlySrc, conflictSrc),
		MustPath(dstDir),
		opts,
		ProgressEmitThrottle{},
		nil,
		skipAllResolver(),
		nil,
	)
	if err != nil {
		t.Fatalf("executeMoveCopyPhase error = %v", err)
	}
	if done != 1 {
		t.Fatalf("done files = %d, want 1 (only non-conflict file)", done)
	}
	if doneBytes <= 0 {
		t.Fatalf("done bytes = %d, want > 0", doneBytes)
	}

	if _, err := os.Stat(onlySrc); !os.IsNotExist(err) {
		t.Fatalf("copied source should be removed: %v", err)
	}
	if _, err := os.Stat(conflictSrc); err != nil {
		t.Fatalf("skipped conflict source should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "only-from-a.txt")); err != nil {
		t.Fatalf("non-conflict dest missing: %v", err)
	}
	data, err := os.ReadFile(conflictDst)
	if err != nil {
		t.Fatalf("read conflict dest: %v", err)
	}
	if string(data) != "content-from-b-conflict" {
		t.Fatalf("conflict dest content = %q, want b-content preserved", string(data))
	}
}

func TestMoveCopyPhaseOverwriteAllRemovesSources(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "conflict-at-root.txt")
	if err := os.WriteFile(srcFile, []byte("content-from-a-conflict"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dstFile := filepath.Join(dstDir, "conflict-at-root.txt")
	if err := os.WriteFile(dstFile, []byte("content-from-b-conflict"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	_, _, err := executeMoveCopyPhase(
		context.Background(),
		nil,
		MustPaths(srcFile),
		MustPath(dstDir),
		opts,
		ProgressEmitThrottle{},
		nil,
		overwriteAllResolver(),
		nil,
	)
	if err != nil {
		t.Fatalf("executeMoveCopyPhase error = %v", err)
	}

	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Fatalf("source should be removed after overwrite move: %v", err)
	}
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "content-from-a-conflict" {
		t.Fatalf("dest content = %q, want a-content", string(data))
	}
}

func TestMoveCopyPhasePartialTreeSkipAll(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	onlyNested := filepath.Join(subDir, "only-from-a-nested.txt")
	conflictNested := filepath.Join(subDir, "conflict-nested.txt")
	if err := os.WriteFile(onlyNested, []byte("nested-from-a"), 0o644); err != nil {
		t.Fatalf("write only nested: %v", err)
	}
	if err := os.WriteFile(conflictNested, []byte("nested-conflict-from-a"), 0o644); err != nil {
		t.Fatalf("write conflict nested: %v", err)
	}

	dstSub := filepath.Join(dstDir, filepath.Base(srcDir), "subdir")
	if err := os.MkdirAll(dstSub, 0o755); err != nil {
		t.Fatalf("mkdir dst subdir: %v", err)
	}
	conflictDst := filepath.Join(dstSub, "conflict-nested.txt")
	if err := os.WriteFile(conflictDst, []byte("nested-conflict-from-b"), 0o644); err != nil {
		t.Fatalf("write conflict dest: %v", err)
	}

	opts := Options{CopyBufferKiB: 4}
	_, _, err := executeMoveCopyPhase(
		context.Background(),
		nil,
		MustPaths(srcDir),
		MustPath(dstDir),
		opts,
		ProgressEmitThrottle{},
		nil,
		skipAllResolver(),
		nil,
	)
	if err != nil {
		t.Fatalf("executeMoveCopyPhase error = %v", err)
	}

	if _, err := os.Stat(onlyNested); !os.IsNotExist(err) {
		t.Fatalf("copied nested source should be removed: %v", err)
	}
	if _, err := os.Stat(conflictNested); err != nil {
		t.Fatalf("skipped nested source should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, filepath.Base(srcDir), "subdir", "only-from-a-nested.txt")); err != nil {
		t.Fatalf("copied nested dest missing: %v", err)
	}
	data, err := os.ReadFile(conflictDst)
	if err != nil {
		t.Fatalf("read conflict dest: %v", err)
	}
	if string(data) != "nested-conflict-from-b" {
		t.Fatalf("conflict dest content = %q, want b-content", string(data))
	}
	if _, err := os.Stat(srcDir); err != nil {
		t.Fatalf("source root should remain with skipped child: %v", err)
	}
}

func writeMoveWalkTree(t *testing.T, root string) {
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

func pauseWalkAfterRoot(t *testing.T, root string) (entered <-chan struct{}, release func()) {
	t.Helper()
	enteredCh := make(chan struct{})
	block := make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	restore := localfs.SetWalkAfterDirHook(func(path string) error {
		if path != root {
			return nil
		}
		enteredOnce.Do(func() { close(enteredCh) })
		<-block
		return nil
	})
	release = func() { releaseOnce.Do(func() { close(block) }) }
	t.Cleanup(release)
	t.Cleanup(restore)
	return enteredCh, release
}

// TestExecuteMoveWithPlanChanWaitsForWalkBeforeRename is the R04-003 ops case: the rename
// fast path must not relocate a directory while the delivery walk is still between emitting
// that root and ReadDir of its children.
func TestExecuteMoveWithPlanChanWaitsForWalkBeforeRename(t *testing.T) {
	srcParent := t.TempDir()
	src := filepath.Join(srcParent, "thicket")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	writeMoveWalkTree(t, src)
	dst := t.TempDir()

	entered, release := pauseWalkAfterRoot(t, src)

	planCh := make(chan PlanItem, 8)
	walkErrCh := make(chan error, 1)
	go func() {
		walkErrCh <- BuildPlanStreamCtx(context.Background(), MustPaths(src), MustPath(dst), true, PlanBuildOptions{}, planCh)
	}()

	moveDone := make(chan struct{})
	var moveErr error
	var doneFiles int
	go func() {
		doneFiles, _, moveErr = ExecuteMoveWithPlanChan(context.Background(), planCh, func() error {
			return <-walkErrCh
		}, MustPaths(src), MustPath(dst), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
		close(moveDone)
	}()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for walk to pause after emitting root")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source must still exist while the walk is paused after emitting root: %v", err)
	}
	stillThere := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(stillThere) {
		if _, err := os.Stat(src); err != nil {
			t.Fatalf("source disappeared while the walk was paused after emitting root: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	release()

	select {
	case <-moveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for ExecuteMoveWithPlanChan")
	}
	if moveErr != nil {
		t.Fatalf("ExecuteMoveWithPlanChan error = %v (old-path traversal after a premature rename)", moveErr)
	}
	if doneFiles != 4 {
		t.Fatalf("doneFiles = %d, want 4 (stable tree: dir + otter.txt + den + fox.txt)", doneFiles)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source should be gone after rename: %v", err)
	}
	if got := readFileContent(t, filepath.Join(dst, "thicket", "otter.txt")); got != "otter-content" {
		t.Fatalf("otter.txt = %q", got)
	}
	if got := readFileContent(t, filepath.Join(dst, "thicket", "den", "fox.txt")); got != "fox-content" {
		t.Fatalf("fox.txt = %q", got)
	}
}

// TestExecuteMoveWithPlanChanMixedDeviceWaitsForWalk covers the mixed same-device / cross-device
// case: a same-device directory must not be renamed while its walk is live, and a later
// cross-device source must still be able to fall back to copy using a complete plan.
func TestExecuteMoveWithPlanChanMixedDeviceWaitsForWalk(t *testing.T) {
	sameParent := t.TempDir()
	srcSame := filepath.Join(sameParent, "thicket")
	if err := os.Mkdir(srcSame, 0o755); err != nil {
		t.Fatalf("mkdir srcSame: %v", err)
	}
	writeMoveWalkTree(t, srcSame)
	dst := t.TempDir()

	crossParent, ok := otherDeviceDir(t, dst)
	if !ok {
		t.Skip("no other-device directory available for mixed same-device/cross-device move")
	}
	srcCross := filepath.Join(crossParent, "harbor.txt")
	if err := os.WriteFile(srcCross, []byte("harbor-content"), 0o644); err != nil {
		t.Fatalf("write cross source: %v", err)
	}

	entered, release := pauseWalkAfterRoot(t, srcSame)

	planCh := make(chan PlanItem, 16)
	walkErrCh := make(chan error, 1)
	go func() {
		walkErrCh <- BuildPlanStreamCtx(context.Background(), MustPaths(srcSame, srcCross), MustPath(dst), true, PlanBuildOptions{FlatDestNames: true}, planCh)
	}()

	moveDone := make(chan struct{})
	var moveErr error
	go func() {
		_, _, moveErr = ExecuteMoveWithPlanChan(context.Background(), planCh, func() error {
			return <-walkErrCh
		}, MustPaths(srcSame, srcCross), MustPath(dst), Options{CopyBufferKiB: 4, FlatDestNames: true}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
		close(moveDone)
	}()

	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for walk to pause after emitting same-device root")
	}
	if _, err := os.Stat(srcSame); err != nil {
		t.Fatalf("same-device source must still exist while its walk is paused: %v", err)
	}
	stillThere := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(stillThere) {
		if _, err := os.Stat(srcSame); err != nil {
			t.Fatalf("same-device source disappeared while its walk was paused: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	release()

	select {
	case <-moveDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for mixed-device ExecuteMoveWithPlanChan")
	}
	if moveErr != nil {
		t.Fatalf("ExecuteMoveWithPlanChan error = %v", moveErr)
	}
	if _, err := os.Stat(srcSame); !os.IsNotExist(err) {
		t.Fatalf("same-device source should be gone: %v", err)
	}
	if _, err := os.Stat(srcCross); !os.IsNotExist(err) {
		t.Fatalf("cross-device source should be gone: %v", err)
	}
	if got := readFileContent(t, filepath.Join(dst, "thicket", "otter.txt")); got != "otter-content" {
		t.Fatalf("otter.txt = %q", got)
	}
	if got := readFileContent(t, filepath.Join(dst, "harbor.txt")); got != "harbor-content" {
		t.Fatalf("harbor.txt = %q", got)
	}
}

func otherDeviceDir(t *testing.T, reference string) (string, bool) {
	t.Helper()
	refDev, ok := pathDev(reference)
	if !ok {
		return "", false
	}
	candidates := []string{"/dev/shm"}
	if uid := os.Getuid(); uid >= 0 {
		candidates = append(candidates, "/run/user/"+strconv.Itoa(uid))
	}
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		d, ok := pathDev(cand)
		if !ok || d == refDev {
			continue
		}
		dir, err := os.MkdirTemp(cand, "pc-move-*")
		if err != nil {
			continue
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		return dir, true
	}
	return "", false
}

func pathDev(path string) (uint64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}
