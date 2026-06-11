package app

import (
	"context"
	"errors"
	"testing"
)

func TestRunMetaCommand_expandsF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/my file.txt"
	out, err := runMetaCommand(context.Background(), "echo %f", path, dir)
	if err != nil {
		t.Fatalf("runMetaCommand: %v", err)
	}
	if out != path {
		t.Fatalf("out = %q, want %q", out, path)
	}
}

func TestRunMetaCommand_success(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	out, err := runMetaCommand(context.Background(), "echo hello", dir+"/file", dir)
	if err != nil {
		t.Fatalf("runMetaCommand: %v", err)
	}
	if out != "hello" {
		t.Fatalf("out = %q, want hello", out)
	}
}

func TestRunMetaCommand_failure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := runMetaCommand(context.Background(), "exit 1", dir+"/file", dir)
	if err == nil {
		t.Fatal("expected error for failing command")
	}
}

func TestRunMetaCommand_cancelled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runMetaCommand(ctx, "echo hello", dir+"/file", dir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
