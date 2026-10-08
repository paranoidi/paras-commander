package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func skipAllResolver() ConflictResolver {
	return func(_ context.Context, src, dst string, facts FileConflictFacts) (ConflictResolution, error) {
		_ = src
		_ = dst
		_ = facts
		return ow(false), nil
	}
}

func overwriteAllResolver() ConflictResolver {
	return func(_ context.Context, src, dst string, facts FileConflictFacts) (ConflictResolution, error) {
		_ = src
		_ = dst
		_ = facts
		return ow(true), nil
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
	done, doneBytes, err := moveViaCopyFallback(
		context.Background(),
		MustPaths(onlySrc, conflictSrc),
		MustPath(dstDir),
		opts,
		ProgressEmitThrottle{},
		nil,
		skipAllResolver(),
		nil,
	)
	if err != nil {
		t.Fatalf("moveViaCopyFallback error = %v", err)
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
	_, _, err := moveViaCopyFallback(
		context.Background(),
		MustPaths(srcFile),
		MustPath(dstDir),
		opts,
		ProgressEmitThrottle{},
		nil,
		overwriteAllResolver(),
		nil,
	)
	if err != nil {
		t.Fatalf("moveViaCopyFallback error = %v", err)
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
	_, _, err := moveViaCopyFallback(
		context.Background(),
		MustPaths(srcDir),
		MustPath(dstDir),
		opts,
		ProgressEmitThrottle{},
		nil,
		skipAllResolver(),
		nil,
	)
	if err != nil {
		t.Fatalf("moveViaCopyFallback error = %v", err)
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

// moveViaCopyFallback drives moveCopyFallback for each source as ExecuteMove does when a rename
// is not possible, so tests can exercise the copy+delete path on a single filesystem.
func moveViaCopyFallback(ctx context.Context, sources []pathloc.Path, destination pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	var files int
	var bytes int64
	for _, src := range sources {
		dst, err := resolveMoveDestination(ctx, destination, src.Base())
		if err != nil {
			return files, bytes, err
		}
		f, b, err := moveRun{opts: opts, throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait}.moveCopyFallback(ctx, src, dst, destination)
		files += f
		bytes += b
		if err != nil {
			return files, bytes, err
		}
	}
	return files, bytes, nil
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

// TestMoveCancelKeepsEarlierRenames: source one overwrites its destination, then source two
// cancels. Like mc, the finished move of source one stays; source two and its destination are
// untouched.
func TestMoveCancelKeepsEarlierRenames(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcAlpha := filepath.Join(srcDir, "alpha.txt")
	srcBeta := filepath.Join(srcDir, "beta.txt")
	dstAlpha := filepath.Join(dstDir, "alpha.txt")
	dstBeta := filepath.Join(dstDir, "beta.txt")
	for path, content := range map[string]string{srcAlpha: "new-alpha", srcBeta: "new-beta", dstAlpha: "old-alpha", dstBeta: "old-beta"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	calls := 0
	resolver := func(context.Context, string, string, FileConflictFacts) (ConflictResolution, error) {
		calls++
		if calls == 1 {
			return ow(true), nil
		}
		return ow(false), fmt.Errorf("canceled by user")
	}

	done, _, err := ExecuteMove(context.Background(), MustPaths(srcAlpha, srcBeta), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, resolver, nil)
	if err == nil {
		t.Fatal("ExecuteMove error = nil, want cancel error")
	}
	if done != 1 {
		t.Fatalf("done = %d, want 1", done)
	}
	if got := readFileContent(t, dstAlpha); got != "new-alpha" {
		t.Fatalf("dest alpha = %q, want new-alpha (earlier rename kept)", got)
	}
	if got := readFileContent(t, srcBeta); got != "new-beta" {
		t.Fatalf("source beta = %q, want new-beta", got)
	}
	if got := readFileContent(t, dstBeta); got != "old-beta" {
		t.Fatalf("dest beta = %q, want old-beta untouched", got)
	}
}

// TestMoveMixedDeviceRenamesOneCopiesOther: the same-device source moves by rename, the
// cross-device one is planned and copied on its own; nothing is rolled back.
func TestMoveMixedDeviceRenamesOneCopiesOther(t *testing.T) {
	sameParent := t.TempDir()
	srcSame := filepath.Join(sameParent, "thicket")
	if err := os.MkdirAll(filepath.Join(srcSame, "den"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcSame, "den", "fox.txt"), []byte("fox-content"), 0o644); err != nil {
		t.Fatalf("write fox: %v", err)
	}
	dst := t.TempDir()
	crossParent, ok := otherDeviceDir(t, dst)
	if !ok {
		t.Skip("no other-device directory available for mixed same-device/cross-device move")
	}
	srcCross := filepath.Join(crossParent, "harbor")
	if err := os.MkdirAll(srcCross, 0o755); err != nil {
		t.Fatalf("mkdir cross: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcCross, "otter.txt"), []byte("otter-content"), 0o644); err != nil {
		t.Fatalf("write otter: %v", err)
	}

	var last int
	progress := func(_, _ string, doneFiles int, _ int64) { last = doneFiles }
	done, _, err := ExecuteMove(context.Background(), MustPaths(srcSame, srcCross), MustPath(dst), Options{CopyBufferKiB: 4, FlatDestNames: true}, ProgressEmitThrottle{}, progress, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("ExecuteMove: %v", err)
	}
	if done < 2 || last != done {
		t.Fatalf("done = %d, last progress = %d; want done >= 2 and equal", done, last)
	}
	for _, p := range []string{srcSame, srcCross} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("source %s should be gone: %v", p, err)
		}
	}
	if got := readFileContent(t, filepath.Join(dst, "thicket", "den", "fox.txt")); got != "fox-content" {
		t.Fatalf("fox.txt = %q", got)
	}
	if got := readFileContent(t, filepath.Join(dst, "harbor", "otter.txt")); got != "otter-content" {
		t.Fatalf("otter.txt = %q", got)
	}
}

func TestMoveOverwriteCommitsAndRemovesStash(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcAlpha := filepath.Join(srcDir, "alpha.txt")
	if err := os.WriteFile(srcAlpha, []byte("new-alpha"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dstAlpha := filepath.Join(dstDir, "alpha.txt")
	if err := os.WriteFile(dstAlpha, []byte("old-alpha"), 0o644); err != nil {
		t.Fatalf("write dest: %v", err)
	}

	_, _, err := ExecuteMove(context.Background(), MustPaths(srcAlpha), MustPath(dstDir), Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, overwriteAllResolver(), nil)
	if err != nil {
		t.Fatalf("ExecuteMove error = %v", err)
	}
	if _, err := os.Stat(srcAlpha); !os.IsNotExist(err) {
		t.Fatalf("source should be gone after overwrite move: %v", err)
	}
	if got := readFileContent(t, dstAlpha); got != "new-alpha" {
		t.Fatalf("dest alpha = %q, want new-alpha", got)
	}
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatalf("ReadDir dest: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".paras-move-stash-") {
			t.Fatalf("leftover stash %q after committed overwrite", e.Name())
		}
	}
}

// ow maps the old overwrite/skip boolean to a resolution.
func ow(overwrite bool) ConflictResolution {
	if overwrite {
		return ConflictResolution{Action: ActionOverwrite}
	}
	return ConflictResolution{Action: ActionSkip}
}
