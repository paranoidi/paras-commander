//go:build windows

package fsvol

import (
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func volumeBytes(path string) (avail uint64, total uint64, ok bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, 0, false
	}
	root := filepath.VolumeName(abs)
	if root == "" {
		return 0, 0, false
	}
	prefix := root
	if len(root) == 2 && root[1] == ':' {
		prefix = root + `\`
	}
	wpath, err := syscall.UTF16PtrFromString(prefix)
	if err != nil {
		return 0, 0, false
	}
	var freeB, totalB, totFree uint64
	err = windows.GetDiskFreeSpaceEx(wpath, &freeB, &totalB, &totFree)
	if err != nil {
		return 0, 0, false
	}
	return freeB, totalB, totalB > 0
}
