package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportCrashWritesLog(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	path, err := crashLogPath()
	if err != nil {
		t.Fatalf("crashLogPath: %v", err)
	}

	reportErr := reportCrash("boom", []byte("goroutine 1 [running]:\nfake.stack()"))
	if reportErr == nil {
		t.Fatal("reportCrash returned nil error")
	}
	if !strings.Contains(reportErr.Error(), path) {
		t.Errorf("error %q does not mention log path %q", reportErr.Error(), path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read crash log: %v", err)
	}
	if !strings.Contains(string(data), "panic: boom") {
		t.Errorf("crash log missing panic message: %s", data)
	}
	if !strings.Contains(string(data), "fake.stack()") {
		t.Errorf("crash log missing stack trace: %s", data)
	}

	// A second crash appends rather than overwriting the first.
	if reportErr := reportCrash("again", []byte("stack 2")); reportErr == nil {
		t.Fatal("reportCrash returned nil error")
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read crash log: %v", err)
	}
	if !strings.Contains(string(data), "panic: boom") || !strings.Contains(string(data), "panic: again") {
		t.Errorf("crash log did not accumulate both entries: %s", data)
	}
}

func TestCrashLogPathUnderCacheDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	path, err := crashLogPath()
	if err != nil {
		t.Fatalf("crashLogPath: %v", err)
	}
	want := filepath.Join(dir, "pc", "crash.log")
	if path != want {
		t.Errorf("crashLogPath() = %q, want %q", path, want)
	}
}
