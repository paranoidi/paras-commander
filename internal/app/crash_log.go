package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// crashLogPath returns $XDG_CACHE_HOME/pc/crash.log (via os.UserCacheDir), mirroring the
// cache directory convention used by preview/prefetch's video-thumbs cache.
func crashLogPath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pc", "crash.log"), nil
}

// reportCrash appends a recovered panic and its stack trace to the crash log and returns an
// error describing where the details were written (or the raw panic if the log itself could
// not be written).
func reportCrash(r any, stack []byte) error {
	path, err := crashLogPath()
	if err != nil {
		return fmt.Errorf("pc crashed: %v (could not resolve crash log path: %w)", r, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("pc crashed: %v (could not create crash log dir: %w)", r, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("pc crashed: %v (could not open crash log: %w)", r, err)
	}
	defer func() { _ = f.Close() }()
	if _, writeErr := fmt.Fprintf(f, "=== %s ===\npanic: %v\n\n%s\n", time.Now().Format(time.RFC3339), r, stack); writeErr != nil {
		return fmt.Errorf("pc crashed: %v (could not write crash log: %w)", r, writeErr)
	}
	return fmt.Errorf("pc crashed: %v (details written to %s)", r, path)
}
