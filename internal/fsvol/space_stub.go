//go:build !unix && !windows

package fsvol

func volumeBytes(string) (avail uint64, total uint64, ok bool) {
	return 0, 0, false
}
