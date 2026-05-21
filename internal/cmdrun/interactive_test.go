package cmdrun

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunInteractiveEmptyArgv(t *testing.T) {
	if err := RunInteractive(context.Background(), nil, ""); err == nil {
		t.Fatal("expected error for empty argv")
	}
}

func TestStartDetachedEmptyArgv(t *testing.T) {
	if err := StartDetached(nil, ""); err == nil {
		t.Fatal("expected error for empty argv")
	}
}

func TestStartDetachedTrue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("true not on PATH on Windows")
	}
	dir := t.TempDir()
	if err := StartDetached([]string{"true"}, dir); err != nil {
		t.Fatal(err)
	}
}

func TestRunInteractiveTrue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("true not on PATH on Windows")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	script := filepath.Join(dir, "touch.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunInteractive(context.Background(), []string{script}, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker not created: %v", err)
	}
}
