package ops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
