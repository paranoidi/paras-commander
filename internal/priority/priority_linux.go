//go:build linux

package priority

import "syscall"

const defaultNiceIncrement = 10

// ApplyBackgroundPriority lowers CPU priority for the current process (nice).
// Returns restore to undo the change.
func ApplyBackgroundPriority(niceIncrement int) (restore func()) {
	restore = func() {}
	if niceIncrement <= 0 {
		niceIncrement = defaultNiceIncrement
	}
	oldNice, err := syscall.Getpriority(syscall.PRIO_PROCESS, 0)
	if err != nil {
		return restore
	}
	newNice := oldNice + niceIncrement
	if newNice > 19 {
		newNice = 19
	}
	if newNice < -20 {
		newNice = -20
	}
	if setErr := syscall.Setpriority(syscall.PRIO_PROCESS, 0, newNice); setErr != nil {
		return restore
	}
	return func() {
		_ = syscall.Setpriority(syscall.PRIO_PROCESS, 0, oldNice)
	}
}
