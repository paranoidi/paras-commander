//go:build windows

package oplog

// processAlive reports every process as alive on Windows, so stale spill directories are
// never swept there (they are still removed by their own session on exit).
func processAlive(int) bool { return true }
