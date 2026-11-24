//go:build unix

package diskusage

import (
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"golang.org/x/sys/unix"
)

func pathStatDevice(abs string) (uint64, bool) {
	var st unix.Stat_t
	if err := unix.Stat(abs, &st); err != nil {
		return 0, false
	}
	return uint64(st.Dev), true
}

// PathDevice returns st_dev for abs after pathloc.Clean. ok is false when Stat fails or on unsupported platforms.
func PathDevice(abs string) (dev uint64, ok bool) {
	if abs == "" {
		return 0, false
	}
	return pathStatDevice(pathloc.Clean(abs))
}
